#!/usr/bin/env python3.12
import argparse
import subprocess
import json
import yaml
import os

def get_k8s_resources(namespace, resource_type):
    try:
        result = subprocess.run(
            ["kubectl", "get", resource_type, "-n", namespace, "-o", "json"],
            capture_output=True, text=True, check=True
        )
        return json.loads(result.stdout)
    except subprocess.CalledProcessError as e:
        print(f"Error fetching {resource_type} for namespace {namespace}: {e.stderr}")
        return None
    except json.JSONDecodeError as e:
        print(f"Error decoding JSON for {resource_type} in {namespace}: {e}")
        return None

def generate_qosmon_config(namespace, ingresses, services, service_host_override=None, sso_middlewares=None):
    tasks = []
    
    if sso_middlewares is None:
        sso_middlewares = []
    sso_middlewares_set = set(sso_middlewares)

    # Process Ingresses
    if ingresses:
        for item in ingresses.get("items", []):
            metadata = item.get("metadata", {})
            ingress_name = metadata.get("name")
            annotations = metadata.get("annotations", {})
            
            middlewares_str = annotations.get("traefik.ingress.kubernetes.io/router.middlewares", "")
            middlewares = [m.strip() for m in middlewares_str.split(",") if m.strip()]
            
            has_sso = any(m in sso_middlewares_set for m in middlewares)
            expected_status = 307 if has_sso else 200

            spec = item.get("spec", {})
            rules = spec.get("rules", [])
            
            for rule in rules:
                host = rule.get("host")
                if not host:
                    continue
                
                protocol = "http"
                tls_list = spec.get("tls", [])
                for tls in tls_list:
                    if host in tls.get("hosts", []):
                        protocol = "https"
                        break
                
                target = f"{protocol}://{host}"
                
                task = {
                    "name": f"{namespace}/ingress/{ingress_name}/{host}",
                    "type": "http",
                    "target": target,
                    "method": "GET",
                    "expect": {
                        "status": expected_status
                    },
                    "sla": {
                        "latency": "2000ms"
                    }
                }
                tasks.append(task)

    # Process LoadBalancer Services
    if services:
        for item in services.get("items", []):
            spec = item.get("spec", {})
            if spec.get("type") != "LoadBalancer":
                continue
            
            metadata = item.get("metadata", {})
            svc_name = metadata.get("name")
            
            ports = []
            for p in spec.get("ports", []):
                if p.get("protocol") == "TCP":
                    ports.append(p.get("port"))
            
            if not ports:
                continue
            
            if service_host_override:
                task = {
                    "name": f"{namespace}/svc/{svc_name}",
                    "type": "tcp",
                    "host": service_host_override,
                    "ports": ports,
                    "timeout": "5s"
                }
                tasks.append(task)
            else:
                status = item.get("status", {})
                lb_ingress = status.get("loadBalancer", {}).get("ingress", [])
                
                for idx, ing in enumerate(lb_ingress):
                    host = ing.get("ip") or ing.get("hostname")
                    if not host:
                        continue
                    
                    name_suffix = f"-{idx}" if len(lb_ingress) > 1 else ""
                    task = {
                        "name": f"{namespace}/svc/{svc_name}{name_suffix}",
                        "type": "tcp",
                        "host": host,
                        "ports": ports,
                        "timeout": "5s"
                    }
                    tasks.append(task)
    
    if not tasks:
        return None
        
    return {"tasks": tasks}

def main():
    parser = argparse.ArgumentParser(description="Generate qosmon config from k8s ingresses and services using a config file")
    parser.add_argument("--config", required=True, help="Path to the generator configuration file")
    args = parser.parse_args()
    
    if not os.path.exists(args.config):
        print(f"Config file not found: {args.config}")
        return

    with open(args.config, "r") as f:
        gen_config = yaml.safe_load(f)

    namespaces = gen_config.get("namespaces", [])
    output_dir = gen_config.get("output_dir", "qosmon/configs/auto-generated")
    service_host = gen_config.get("service_host")
    sso_middlewares = gen_config.get("sso_middlewares", [])
    
    os.makedirs(output_dir, exist_ok=True)
    
    for ns in namespaces:
        print(f"Processing namespace: {ns}")
        ingresses = get_k8s_resources(ns, "ingress")
        services = get_k8s_resources(ns, "service")
        
        config = generate_qosmon_config(ns, ingresses, services, service_host, sso_middlewares)
        if config:
            file_path = os.path.join(output_dir, f"{ns}.yaml")
            with open(file_path, "w") as f:
                f.write(f"# Auto-generated qosmon config for namespace: {ns}\n")
                
                class NoAliasDumper(yaml.SafeDumper):
                    def ignore_aliases(self, data):
                        return True
                
                yaml.dump(config, f, Dumper=NoAliasDumper, default_flow_style=False, sort_keys=False)
            print(f"Generated {file_path}")
        else:
            print(f"No resources found for namespace {ns}")

if __name__ == "__main__":
    main()
