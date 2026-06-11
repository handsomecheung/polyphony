# DDNS (Dynamic DNS) Service

Automatically synchronizes dynamic IP addresses (IPv4 and IPv6) with DNS providers (Cloudflare and Aliyun). Two separate Kubernetes CronJobs run every 5 minutes to detect and update DNS records when IP addresses change, enabling reliable external access to cluster services despite ISP-assigned IPs.

## Architecture

The DDNS service maintains two independent update pipelines:

- **Cloudflare pipeline**: Updates 6 internal domains (DOMAIN_X, Y, P, T, C, D) with root, wildcard, and subdomain A records
- **Aliyun pipeline**: Updates an Aliyun-hosted domain (DOMAIN_U) with specific subdomains

Both pipelines share a common IP discovery module that queries multiple sources for resilience.

## Components

### `code/common.py`

Shared utilities for IP discovery and logging.

**IPv4 detection**: Queries 5 independent public APIs in order of preference with 2-second timeout per request. Falls back to next API on failure:

- `https://ifconfig.me/ip`
- `https://api.ipify.org`
- `https://ipinfo.io/ip`
- `https://icanhazip.com`
- `https://ident.me`

Applies IPv4-only socket policy (via urllib3 monkey-patch) to avoid IPv6 API confusion.

**IPv6 detection**: Queries internal `comapi` service running on nur, nippon, and miniba nodes via the `/ipv6` endpoint. Gracefully skips unavailable nodes.

**Logging**: Timestamped log output for debugging and monitoring.

### `code/run-cloudflare.py`

Updates DNS records for 6 Cloudflare-hosted domains via the Cloudflare API.

**Domains**: Reads DOMAIN_X, Y, P, T, C, D from environment.

**Records per domain**:

| Domain               | Records                                                           | Proxied                            |
|----------------------|-------------------------------------------------------------------|------------------------------------|
| DOMAIN_X             | `""` (root), `*.`, `home.`, `*.home.`, `nur.home.`, `*.nur.home.` | Root & wildcard=yes; subdomains=no |
| DOMAIN_Y, T, P, C, D | `""` (root), `*.`                                                 | yes                                |

**TTL**: 60 seconds for fast failover.

**Reconciliation logic**: For each record:
- If no records exist, creates a new A record
- If records exist but content differs, deletes stale records and creates the correct one
- If a matching record already exists, skips the update (idempotent)

**IPv6 support**: Currently disabled (configuration commented out in DOMAINS_IPV6 map). When enabled, updates AAAA records for home6 subdomains on nippon, nur, and miniba nodes.

### `code/run-aliyun.py`

Updates DNS records for an Aliyun-hosted domain via Aliyun's DNS SDK.

**Domain**: Reads DOMAIN_U from environment.

**Subdomains**: Targets `*.j` subdomain only.

**TTL**: 600 seconds.

**Record type**: A (IPv4 only).

**Authentication**: Uses Aliyun Access Key ID and Secret (ACCESS_ID, ACCESS_SECRET) injected at runtime.

**Reconciliation logic**: Queries all A records for the domain, finds those in SUBDOMAINS, and updates IP only if it differs from current.

## Build & Deployment

### Build

```bash
./build.sh
```

Invokes `my-k8s-build-image` to build `cloudpublic/default/ddns:latest` via the Dockerfile. The multi-stage Python image includes:

- **Base**: `python:slim-bookworm`
- **Dependencies**: 
  - `cryptography==3.4.6` (with Rust build disabled)
  - `aliyun-python-sdk-core`
  - `aliyun-python-sdk-alidns`
  - `cloudflare==2.19.4`
  - `requests`

### Deploy

```bash
./deploy.sh
```

Applies `app.yaml` to the Kubernetes cluster via `my-k8s-deploy`.

## Kubernetes Manifest (`app.yaml`)

### Namespace

Creates a dedicated `ddns` namespace.

### ConfigMap

`common-envs` holds domain names and comapi endpoints, templated via external configuration:

```
DOMAIN_X, Y, P, C, D, T, U — root domain names
COMAPI_HOST_NUR, NIPPON, MINIBA — internal service URLs (http://<ip>:<port>)
COMAPI_PATH_IPV6 — fixed path /ipv6 for IPv6 queries
```

### CronJobs

**cloudflare** and **aliyun** (identical schedule and policies):

- **Schedule**: `*/5 * * * *` (every 5 minutes)
- **Concurrency policy**: Forbid (prevents overlapping runs)
- **History limits**: Keep 2 successful and 5 failed job records
- **Image pull policy**: Always (pulls latest image on each run)
- **Restart policy**: Never

**Environment injection**:

- **cloudflare job**: 
  - CLOUDFLARE_API_TOKEN from templated config
  - Common envs from ConfigMap
  - Runs `python3 -u run-cloudflare.py`

- **aliyun job**:
  - ACCESS_ID and ACCESS_SECRET from templated config
  - Common envs from ConfigMap
  - Runs `python3 -u run-aliyun.py`

## Security & Configuration

- **API credentials** (Cloudflare token, Aliyun keys) are injected via Kubernetes template variables at deployment time (not hardcoded)
- **Stateless updates**: No persistent state; idempotent logic ensures safe re-runs
- **Concurrency control**: CronJob forbids overlapping executions to prevent race conditions
- **Graceful degradation**: IPv4 detection tries multiple APIs; IPv6 queries skip offline nodes

## Notes

- **Dockerfile**: Disables Rust build for cryptography to reduce build time and dependencies
- **Logging**: All operations are logged with timestamps for debugging and auditing
- **Error handling**: Logs failures but continues execution; DNS updates proceed only when a valid IPv4 is available
- **IPv6 implementation**: Infrastructure and code support IPv6 (via comapi service queries), but updates are currently disabled pending configuration activation
