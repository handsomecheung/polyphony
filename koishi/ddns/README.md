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

See parent [koishi README](../README.md) for cluster-wide architecture.
