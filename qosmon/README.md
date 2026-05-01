# qosmon

`qosmon` is a lightweight and fast monitoring framework implemented in Rust.
It supports HTTP(S), TCP, DNS, SSL, and parallel port scanning, aiming to verify infrastructure health in a single run.

## Key Features

- **Multi-protocol support**: HTTP/API, TCP Port, DNS Resolution, SSL Certificate.
- **Fast parallel port scanning**: Uses Tokio to scan up to 1,000 ports concurrently.
- **Flexible expectation validation**: Supports status codes, body strings, JSONPath, expected/unexpected IP addresses, and more.
- **Modular configuration**: Load multiple YAML files from a directory and merge global settings.
- **Asynchronous execution**: All I/O operations are performed asynchronously for efficient operation.

## Quick Start

### Build

```bash
cd qosmon
cargo build --release
```

### Execution

You can run it with sample configurations using the provided helper script:

```bash
./run.sh
```

To specify a particular config file or directory:

```bash
./run.sh --config-file custom.yaml
./run.sh --config-dir my_configs/
```

## Automatic Generation of Kubernetes Ingress/Service Configuration

A script is provided to automatically generate monitoring configurations from Ingress and LoadBalancer Services on a Kubernetes cluster.

### Usage

```bash
# Generate by specifying a configuration file
python3.12 qosmon/scripts/generate.py --config qosmon/scripts/generator-config.yaml
```

### Configuration File (`qosmon/scripts/generator-config.yaml`)

Generation behavior can be managed via this YAML file:

- `namespaces`: List of Kubernetes namespaces to monitor.
- `output_dir`: Destination for generated YAML configs (default is `qosmon/configs/auto-generated`).
- `service_host`: Common host to use for LoadBalancer Service checks (optional).
- `sso_middlewares`: Automatically changes the expected status to `307` for Ingresses with specified Traefik middlewares applied.

### Generation Rules

- **Ingress**: Generates HTTP GET checks for each host. Uses HTTPS if TLS is configured.
- **LoadBalancer Service**: Generates TCP connectivity checks for all TCP ports found in the service.

## Configuration Method (YAML)

Configuration consists of `globals` (common settings) and `tasks` (individual tasks) sections.

### Global Settings

```yaml
globals:
  timeout: "5s" # Default timeout
```

### Monitoring Task Types

#### 1. HTTP / API Monitoring
```yaml
- name: "API Check"
  type: http
  target: "https://api.example.com/v1/status"
  method: GET
  headers:
    Accept-Language: "en-US"
  expect:
    status: 200
    json:
      $.status: "ok"
```

#### 2. TCP Port Connectivity Check
```yaml
- name: "DB Port"
  type: tcp
  host: "db.internal"
  ports: [5432, 6379] # Multiple ports can be specified
```

#### 3. DNS Resolution Check
```yaml
- name: "DNS Check"
  type: dns
  target: "example.com"
  server: "8.8.8.8" # DNS server to query (optional)
  expected_records: ["93.184.216.34"] # IPs that should be present
  unexpected_records: ["1.2.3.4"]     # IPs that should not be present
```

#### 4. SSL Certificate Check
```yaml
- name: "SSL Expiry"
  type: ssl
  target: "example.com:443"
```

#### 5. Port Scan
```yaml
- name: "Security Audit"
  type: port_scan
  target: "192.168.1.1"
  range: "1-1024"       # Scan range
  expect_open: [22, 80] # Ports that should be open
  expect_closed: [443]  # Ports that should be closed
```

## Design Philosophy

- **Surgical Execution**: A tool for quick execution and results when needed, without unnecessary daemonization or persistence.
- **Fail Fast**: Clearly reports the cause of failures (which IP, port, or mismatch with expected values).
- **Safety First**: Uses semaphores during port scanning to control parallelism and protect local resources and the network.

## License

MIT License
