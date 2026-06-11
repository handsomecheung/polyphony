# qosmon

`qosmon` is a high-performance infrastructure health monitoring framework implemented in Rust. It executes a diverse portfolio of health checks across HTTP/API endpoints, TCP connectivity, DNS resolution, SSL certificates, and port scanning in parallel, using a global semaphore-based resource pool to ensure stable, predictable resource consumption across all task types. It is designed for surgical execution—quick, targeted validation of infrastructure state without persistent services or log collection.

## Key Features

- **Full parallel execution**: All monitoring tasks are executed concurrently for maximum speed.
- **Global resource management**: Uses a global semaphore to limit concurrent network connections (default 1000) across all tasks, preventing OS resource exhaustion and network congestion.
- **Multi-protocol support**: HTTP/API, TCP Port, DNS Resolution, SSL Certificate, NoIndex Tags, and Port Scanning.
- **Fast parallel port scanning**: Efficiently scans large port ranges using the global connection pool.
- **Flexible expectation validation**: Supports HTTP status codes, body strings, JSONPath queries, expected/unexpected IP addresses, DNS records, and port states.
- **Modular configuration**: Load multiple YAML files from a directory with automatic merging of global settings and per-task overrides.
- **Kubernetes-integrated**: Auto-generate configurations from live Ingress and LoadBalancer resources via kubectl queries; automatically detects SSO middlewares and health check paths.
- **Search engine privacy enforcement**: Validates NoIndex tags (HTTP header and HTML meta tags) to ensure private services are not indexed by search engines.

## Quick Start

### Build

```bash
cd qosmon
cargo build --release
```

### Execution

Run with sample configurations using the provided helper script:

```bash
./run.sh
```

To specify a particular config file or directory:

```bash
./run.sh --config-file custom.yaml
./run.sh --config-dir my_configs/
```

The default invocation (`./run.sh`) loads all YAML configs from `configs/check/` (both manual and auto-generated) and runs all checks concurrently with a global concurrency limit of 50 concurrent connections.

## Configuration

### Overview

Configuration is structured into two main categories:

1. **Manual Configurations** (`configs/check/`): Hand-written YAML files for specific checks (e.g., `koishi.yaml`, `ck.yaml`). These define custom health checks, DNS validations, and port scans for key infrastructure endpoints.
2. **Auto-Generated Configurations** (`configs/check/auto-generated/`): Generated from live Kubernetes Ingress and Service resources via the `generate.py` script. These provide dynamic, drift-free monitoring of all public-facing and internal services.

### Global Settings

Place global defaults in `configs/check/global.yaml`:

```yaml
globals:
  timeout: "5s"  # Default timeout for all tasks
```

All YAML files in `configs/check/` are merged at startup, with task-specific settings overriding globals.

### Monitoring Task Types

#### 1. HTTP / API Monitoring

Validate API responses, status codes, body content, or JSON payloads.

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
    body:
      - "success"
  timeout: "5s"
```

**Fields:**
- `target`: Full URL (required)
- `method`: HTTP method (`GET`, `POST`, etc.; default `GET`)
- `headers`: Custom HTTP headers (optional)
- `expect`: Validation rules (optional)
  - `status`: Expected HTTP status code
  - `json`: JSONPath assertions (e.g., `$.field: "value"`)
  - `body`: List of strings that must appear in response body
- `timeout`: Task-specific timeout (overrides global)

#### 2. TCP Port Connectivity Check

Verify that one or more TCP ports are open and reachable.

```yaml
- name: "Database Port"
  type: tcp
  host: "db.internal"
  ports: [5432, 6379]
  timeout: "5s"
```

**Fields:**
- `host`: Hostname or IP address (required)
- `ports`: List of port numbers to check (required)
- `timeout`: Task-specific timeout

#### 3. DNS Resolution Check

Verify that DNS resolves to expected IPs and does not resolve to unexpected ones (useful for detecting DNS hijacking or misconfiguration).

```yaml
- name: "DNS Check"
  type: dns
  target: "example.com"
  server: "8.8.8.8"
  expected_records: ["93.184.216.34"]
  unexpected_records: ["1.2.3.4"]
```

**Fields:**
- `target`: Domain name (required)
- `server`: Custom DNS server IP (optional; uses system resolver if not specified)
- `expected_records`: List of IPs that must resolve (optional)
- `unexpected_records`: List of IPs that must NOT resolve (optional)

#### 4. SSL Certificate Check

Validate SSL certificate validity and expiration.

```yaml
- name: "SSL Expiry"
  type: ssl
  target: "example.com:443"
  alert_days_before: 14
```

**Fields:**
- `target`: Hostname:Port (required)
- `alert_days_before`: Warn if cert expires within N days (optional; not enforced, informational)

#### 5. Port Scan

Scan a range of ports and verify expected open/closed states.

```yaml
- name: "Security Audit"
  type: port_scan
  target: "192.168.1.1"
  range: "1-1024"
  expect_open: [22, 80]
  expect_closed: [443]
```

**Fields:**
- `target`: Hostname or IP address (required)
- `range`: Port range (e.g., `1-1024`, `80,443,8080`; required)
- `expect_open`: Ports that should be reachable (optional)
- `expect_closed`: Ports that should NOT be reachable (optional)

#### 6. NoIndex Tag Check

Verify that a web page is protected from search engine indexing via `X-Robots-Tag: noindex` HTTP header or `<meta name="robots" content="noindex">` HTML tag. Essential for ensuring private services are not exposed to search engines.

```yaml
- name: "Search Engine Exclusion"
  type: noindex
  target: "https://private.example.com"
  timeout: "5s"
```

**Fields:**
- `target`: Full URL (required)
- `timeout`: Task-specific timeout

## Automatic Generation of Kubernetes Ingress/Service Configuration

The `generate.py` script automatically creates monitoring configurations from live Kubernetes Ingress and LoadBalancer Service resources, eliminating manual config drift and ensuring all public-facing and internal services are monitored.

### Usage

```bash
python3.12 scripts/generate.py --config configs/generate/config.yaml
```

Or use the convenience script:

```bash
scripts/run.generate.sh
```

### Configuration File (`configs/generate/config.yaml`)

The generator is configured via `configs/generate/config.yaml`:

```yaml
namespaces:
  - default
  - default-vaultwarden
  - finance
  - middleware
  - database
output_dir: /path/to/qosmon/configs/check/auto-generated
service_host: home.my.domain
sso_middlewares:
  - sso-domainx@kubernetescrd
  - sso-domaind@kubernetescrd
  - sso-domainc@kubernetescrd
  - sso-domaint@kubernetescrd
  - sso-domainp@kubernetescrd
  - sso-domainy@kubernetescrd
```

**Configuration fields:**
- `namespaces`: List of Kubernetes namespaces to scan for Ingress/Service resources
- `output_dir`: Directory where generated YAML configs are written
- `service_host`: Override hostname for LoadBalancer Service checks (useful if services lack external IPs)
- `sso_middlewares`: List of Traefik middleware names that indicate SSO is enabled (see Generation Rules below)

### Generation Rules

**Ingress Resources:**
- Generates one HTTP GET check per host, per Ingress
- Detects HTTPS from TLS configuration
- Auto-detects health check paths from Kubernetes `readinessProbe` and `livenessProbe` annotations
- If Ingress has an SSO middleware (from `sso_middlewares` list), sets expected status to `307` (redirect); otherwise expects `200`
- Generates a companion `noindex` check for each host to verify search engine exclusion
- For HTTPS endpoints, generates an `ssl` check to validate certificate validity

**LoadBalancer Service Resources:**
- Generates TCP connectivity checks for all TCP ports
- Uses service external IP (or `service_host` override) as the target
- Supports multiple external IPs by creating separate checks for each

### Example: Running the Generator

```bash
# Regenerate all configs (removes old auto-generated configs)
rm -rf qosmon/configs/check/auto-generated
python3.12 qosmon/scripts/generate.py --config qosmon/configs/generate/config.yaml --reject-internal-ingress
```

The `--reject-internal-ingress` flag skips Ingress resources with non-resolvable (private/internal) hostnames.

## Architecture

### Rust Core (`src/main.rs`)

The monitoring engine is written in async Rust using `tokio`. It:

1. Loads all YAML configs from the specified directory
2. Parses global settings and task definitions
3. Creates a global semaphore for connection limiting (tunable via `--concurrency` CLI flag)
4. Spawns concurrent tasks for each monitoring check
5. Reports success/failure as checks complete

**Key dependencies:**
- `tokio`: Async runtime
- `reqwest`: HTTP client (with `rustls-tls` for TLS)
- `hickory-resolver`: DNS resolution
- `serde_yaml`: YAML parsing
- `serde_json`: JSON handling
- `jsonpath_lib`: JSONPath queries for response validation
- `chrono`: Date/time for SSL cert validation
- `regex`: Pattern matching

### Configuration Generator (`scripts/generate.py`)

Python 3.12 script that:

1. Queries the live Kubernetes API via `kubectl` for Ingress and Service resources
2. Extracts hosts, ports, TLS settings, middleware annotations, and health check paths
3. Generates YAML task definitions
4. Writes output to the configured directory

Uses `socket.getaddrinfo()` and `ipaddress` module to distinguish public from private domains.

### Configuration Directories

```
configs/
├── check/
│   ├── global.yaml                  # Global settings (timeout, etc.)
│   ├── koishi.yaml                  # Manual checks for koishi.my.domain
│   ├── ck.yaml                      # Manual checks for ck.my.domain
│   └── auto-generated/
│       ├── default.yaml             # Auto-generated from default namespace
│       ├── database.yaml            # Auto-generated from database namespace
│       └── ...
└── generate/
    └── config.yaml                  # Generator configuration
```

## Concurrency and Resource Management

The global semaphore limits the number of concurrent network operations, preventing:
- OS file descriptor exhaustion
- Network congestion from hundreds of simultaneous connections
- Resource starvation on constrained systems

**Default:** 50 concurrent connections (set in `run.sh`)

**Override at runtime:**

```bash
cargo run -- --config-dir configs/check --concurrency 100
```

## Design Philosophy

- **Surgical Execution**: A tool for quick, targeted infrastructure validation when needed—no daemonization, no persistent state, no log archival. Run it on demand to answer "is my cluster healthy?"
- **High Concurrency**: Executes all tasks in parallel, printing results as they complete for immediate feedback.
- **Resource Safety**: Global semaphore-based concurrency control ensures predictable, bounded resource usage even under high task volume.
- **Fail Fast**: Reports the specific cause of failures (which IP, port, header, or expected value mismatch) for rapid diagnosis.
- **Kubernetes-Native**: Deeply integrated with the cluster via auto-generation of configs from live Ingress/Service resources, reducing manual drift and maintenance burden.

## License

MIT License
