#!/usr/bin/python3
# -*- coding: utf-8 -*-

import os
import glob
import json
import base64
import socket
import subprocess
import warnings

import requests

# Suppress SSL warnings when verify=False
warnings.filterwarnings('ignore', message='Unverified HTTPS request')

import convert_ss

current_dir = os.path.dirname(os.path.abspath(__file__))
singbox_config_dir = os.path.abspath(os.path.join(current_dir, "../sing-box/configs"))
singbox_config_prefix = "20_outbounds_"
NODE_PREFIX = "node"


def run_shell(cmd):
    process = subprocess.Popen(["bash", "-c", cmd], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    stdout, stderr = process.communicate()
    if process.returncode != 0:
        raise RuntimeError(f"failed to run cmd `{cmd}`, error: `{stderr}`, output: `{stdout}`")

    return str(stdout, "utf-8").strip(), str(stderr, "utf-8").strip()


def parse_subscription(subscription):
    # Try to fetch subscription content directly
    try:
        response = requests.get(subscription, timeout=30, verify=False)
        content = response.text

        # Try to parse as JSON (sing-box format)
        try:
            data = json.loads(content)
            if "outbounds" in data:
                # This is a sing-box JSON config
                return {"type": "singbox", "data": data}
        except json.JSONDecodeError:
            pass

        # If not JSON, check if it's base64 encoded
        # Base64 content usually doesn't have newlines and special chars
        if '\n' not in content or content.startswith('ss://'):
            # Looks like already decoded or plain text URI list
            return {"type": "uri_list", "data": content.split("\n")}

        # Try base64 decode
        try:
            decoded = base64.b64decode(content).decode('utf-8')
            return {"type": "uri_list", "data": decoded.split("\n")}
        except Exception:
            # If decode fails, treat as plain text
            return {"type": "uri_list", "data": content.split("\n")}

    except Exception as e:
        print(f"Error fetching subscription: {e}")
        raise


def write_config(config, filename):
    content = json.dumps(config, indent=4, ensure_ascii=False)
    with open(filename, "w") as f:
        f.write(content)


def gen_singbox_config_filename(country):
    country_lower = country.lower()
    return os.path.join(singbox_config_dir, singbox_config_prefix + f"{country_lower}.json")


def read_singbox_config(filename):
    if os.path.exists(filename):
        with open(filename, "r") as f:
            return json.load(f)
    return {"outbounds": []}


def delete_all_country_configs():
    pattern = os.path.join(singbox_config_dir, singbox_config_prefix + "*.json")
    deleted_count = 0
    for f in glob.glob(pattern):
        os.remove(f)
        deleted_count += 1
        print(f"Deleted {f}")
    if deleted_count > 0:
        print(f"Deleted {deleted_count} country config files")


def update_selectors(outbounds_by_country):
    selector_file = os.path.join(singbox_config_dir, "30_outbounds_selectors.json")

    # Read existing selector config or create new one
    if os.path.exists(selector_file):
        with open(selector_file, "r") as f:
            config = json.load(f)
    else:
        config = {"outbounds": []}

    # Only create selector for Japan nodes
    if "JP" not in outbounds_by_country:
        print("No JP nodes found, skipping selector update")
        return

    country = "JP"
    outbounds = outbounds_by_country[country]
    selector_tag = "BalancerJP"

    # Extract node tags
    node_tags = [ob["tag"] for ob in outbounds]

    # Find existing selector or create new one
    selector_found = False
    for selector in config["outbounds"]:
        if selector.get("tag") == selector_tag:
            selector["outbounds"] = node_tags
            selector_found = True
            break

    if not selector_found:
        config["outbounds"].append({
            "type": "urltest",
            "tag": selector_tag,
            "outbounds": node_tags,
            "url": "https://www.gstatic.com/generate_204",
            "interval": "1m",
            "tolerance": 50
        })

    print(f"Updated selector {selector_tag} with {len(node_tags)} nodes")

    # Write config
    write_config(config, selector_file)



def generate_ss_outbound(uri, name, sub_index):
    raw_config = convert_ss.convert(uri)
    if len(raw_config) == 0:
        return None

    sconfig = raw_config["config"]
    country = get_country(sconfig["server"], raw_config["remark"])
    tag = f"{NODE_PREFIX}-{country}-{name}-{sconfig['server']}-{sub_index}"

    # sing-box format for shadowsocks outbound
    outbound = {
        "type": "shadowsocks",
        "tag": tag,
        "server": sconfig["server"],
        "server_port": sconfig["server_port"],
        "method": sconfig["method"],
        "password": sconfig["password"]
    }

    return {
        "country": country,
        "outbound": outbound
    }


def generate(subscription):
    # Collect all outbounds by country
    outbounds_by_country = {}

    parsed = parse_subscription(subscription["url"])

    if parsed["type"] == "singbox":
        # Direct sing-box JSON format
        print("Processing sing-box JSON format subscription")
        outbounds = parsed["data"].get("outbounds", [])

        for sub_index, outbound in enumerate(outbounds):
            # Skip non-proxy outbounds
            if outbound.get("type") not in ["shadowsocks", "vmess", "trojan", "vless", "hysteria", "hysteria2"]:
                continue

            # Get server and tag
            server = outbound.get("server", "")
            tag = outbound.get("tag", f"node-{sub_index}")

            # Skip info nodes (traffic, expire, etc.)
            if not server or any(keyword in tag for keyword in ["Traffic", "Expire", " G |", "Reset"]):
                continue

            # Detect country from tag
            country = extract_country_from_tag(tag)

            if not country:
                # Fallback to IP lookup
                country = get_country(server, tag) if server else "UNKNOWN"

            # Rename tag with our naming convention
            new_tag = f"{NODE_PREFIX}-{country}-{subscription['name']}-{server}-{sub_index}"
            outbound["tag"] = new_tag

            if country not in outbounds_by_country:
                outbounds_by_country[country] = []
            outbounds_by_country[country].append(outbound)

    elif parsed["type"] == "uri_list":
        # URI list format (ss://, vmess://, etc.)
        print("Processing URI list format subscription")
        for sub_index, uri in enumerate(parsed["data"]):
            if uri == "":
                continue

            if uri.startswith("ss://"):
                result = generate_ss_outbound(uri, subscription["name"], sub_index)
                if result is not None:
                    country = result["country"]
                    outbound = result["outbound"]

                    if country not in outbounds_by_country:
                        outbounds_by_country[country] = []
                    outbounds_by_country[country].append(outbound)
            else:
                print(f"Warning: skipping unsupported URI: {uri[:20]}...")

    # Write outbounds to country-specific files
    for country, new_outbounds in outbounds_by_country.items():
        filename = gen_singbox_config_filename(country)
        config = read_singbox_config(filename)

        # Add new nodes
        config["outbounds"].extend(new_outbounds)

        # Write config
        write_config(config, filename)
        print(f"Updated {filename} with {len(new_outbounds)} nodes")

    # Update selectors
    update_selectors(outbounds_by_country)


def extract_country_from_tag(tag):
    """Extract country code from node tag with emoji flag or country name"""
    # Country emoji flag to code mapping
    # Regional Indicator Symbol Letters (U+1F1E6-1F1FF)
    emoji_to_country = {}
    country_codes = ['HK', 'TW', 'JP', 'SG', 'US', 'GB', 'DE', 'FR', 'CA', 'AU', 'KR', 'NL', 'CH', 'SE', 'RU', 'IN', 'BR', 'AR', 'CL', 'AE', 'TR', 'ID', 'IE', 'AT', 'BG', 'HU']

    for code in country_codes:
        # Convert country code to flag emoji
        # A = U+1F1E6, so HK = U+1F1ED (H) + U+1F1F0 (K)
        flag = chr(0x1F1E6 + ord(code[0]) - ord('A')) + chr(0x1F1E6 + ord(code[1]) - ord('A'))
        emoji_to_country[flag] = code

    # Try to find emoji flag
    for emoji, code in emoji_to_country.items():
        if emoji in tag:
            return code

    # Try to match country names in tag
    country_names = {
        "Hong Kong": "HK",
        "Taiwan": "TW",
        "Japan": "JP",
        "Singapore": "SG",
        "United States": "US",
        "USA": "US",
        "United Kingdom": "GB",
        "UK": "GB",
        "Germany": "DE",
        "France": "FR",
        "Canada": "CA",
        "Australia": "AU",
        "Korea": "KR",
        "Netherlands": "NL",
        "Switzerland": "CH",
        "Sweden": "SE",
        "Russia": "RU",
        "India": "IN",
        "Brazil": "BR",
        "Argentina": "AR",
        "Chile": "CL",
        "UAE": "AE",
        "Turkey": "TR",
        "Indonesia": "ID",
        "Ireland": "IE",
        "Austria": "AT",
        "Bulgaria": "BG",
        "Hungary": "HU",
    }

    for name, code in country_names.items():
        if name in tag:
            return code

    return None


def get_country_by_remark(remark):
    country_info = {
        "美国": "US",
        "德国": "DE",
        "英国": "GB",
        "日本": "JP",
        "香港": "HK",
        "台湾": "TW",
        "韩国": "KR",
        "印度": "IN",
        "泰国": "TH",
        "马来西亚": "MY",
        "新加坡": "SG",
        "Hong Kong": "HK",
        "USA": "US",
        "United States": "US",
        "Japan": "JP",
        "Korea": "KR",
        "Singapore": "SG",
        "Taiwan": "TW",
        "Germany": "DE",
        "United Kingdom": "GB",
        "France": "FR",
        "Netherlands": "NL",
        "Russia": "RU",
        "Switzerland": "CH",
        "Sweden": "SE",
        "Austria": "AT",
        "Bulgaria": "BG",
        "Ireland": "IE",
        "Turkey": "TR",
        "Hungary": "HU",
        "Thailand": "TH",
        "Malaysia": "MY",
        "India": "IN",
        "Australia": "AU",
        "United Arab Emirates": "AE",
        "Indonesia": "ID",
        "Brazil": "BR",
        "Argentina": "AR",
        "Chile": "CL",
        "Canada": "CA",
    }

    for country, code in country_info.items():
        if country in remark:
            return code

    return None


def get_country(addr, remark):
    if not is_ip(addr):
        addr = lookup_ip(addr)

    country = get_country_by_remark(remark)
    if country is None:
        country = get_country_by_ip(addr)

    return country


def get_country_by_ip(ip):
    try:
        r = requests.get(f"http://ip2location.default/info?ip={ip}", timeout=2)
        return r.json()["country_code"]
    except Exception as e:
        print(f"Warning: failed to get country for IP {ip}: {e}")
        return "UNKNOWN"


def lookup_ip(addr):
    return socket.gethostbyname(addr)


def is_ip(addr):
    try:
        socket.inet_aton(addr)
        return True
    except socket.error:
        return False


if __name__ == "__main__":
    subscriptions = [
        {
            "index": "03",
            "name": "Nexitally-SingBox-2022",
            "url": "",
        },
    ]

    # Delete all existing country config files at the start
    delete_all_country_configs()

    for subscription in subscriptions:
        generate(subscription)
