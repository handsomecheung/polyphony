# umbilical

Umbilical is the network boundary management layer for the polyphony home lab. It bridges on-premises machines (router node `r3b`, secondary machine `silver`) with a public cloud VM (`jserver`) through encrypted WireGuard VPN, autossh reverse tunnels, dynamic DNS, OAuth-gated ingress, transparent proxy routing via sing-box, and remote power management (Wake-on-LAN and shutdown). All configuration is templated with `__{{...}}__` placeholders and rendered at deploy time by the `bwww` tool, keeping secrets out of source control.

## Directory Structure

```
umbilical/
├── cloud/          # Docker Compose stack for the public cloud VM (jserver)
├── images/         # Dockerfiles and build scripts for all custom images
├── local/
│   ├── router/     # Main Docker Compose stack for the on-premises router (r3b)
│   └── silver/     # Docker Compose stack for the secondary on-premises machine
└── scripts/        # Shared build tooling
```

## Cloud Stack (`cloud/`)

The cloud VM runs a minimal two-container stack, acting as the public network edge:

| Service     | Image                 | Port(s)   | Role                                                                                                                                   |
|-------------|-----------------------|-----------|----------------------------------------------------------------------------------------------------------------------------------------|
| `nginx`     | `umbilical/nginx`     | 25443:443 | TLS termination; reverse proxies `wireguard.` subdomain to the wg-easy UI (port 51821) and serves a health endpoint on the root domain |
| `wireguard` | `umbilical/wireguard` | 51820/udp | WireGuard VPN endpoint (wg-easy web UI on 51821, proxied through nginx)                                                                |

Deployment is driven by `cloud/sync.sh`, which rsync-copies the rendered config to `jserver` via SSH:

```bash
cd cloud
./sync.sh     # render secrets and push to jserver
./restart.sh  # docker compose up on jserver
./stop.sh     # docker compose down on jserver
```

The nginx config (`cloud/nginx/configs/nginx.conf`) terminates TLS using Let's Encrypt certificates from the shared `static/letsencrypt/` directory and proxies the WireGuard management UI at `wireguard.<domain>:443`.

## Router Stack (`local/router/`)

The router runs the core of umbilical's network control logic. All containers use `network_mode: host` to avoid NetworkManager DNS conflicts on the Raspberry Pi 64-bit OS. The stack is deployed via `local/router/sync.sh` (rsync + `bwww render-file` to host `r3b`) and managed with `restart.sh` / `stop.sh`.

### Services

| Service         | Image                     | Role                                                                                                 |
|-----------------|---------------------------|------------------------------------------------------------------------------------------------------|
| `tunnel-ssh`    | `umbilical/tunnel`        | Separate autossh tunnel dedicated to port 3701 (SSH relay)                                           |
| `tunnel`        | `umbilical/tunnel`        | autossh reverse tunnels: exposes router ports 80/443/1083 and silver ports 3737/3738/3739 to jserver |
| `ipset-watcher` | `umbilical/ipset-watcher` | Maintains the `priority` Linux ipset for iptables policy routing                                     |
| `ddns`          | `umbilical/ddns`          | Dynamic DNS; updates Aliyun A records every 2 minutes                                                |
| `sing-box`      | `umbilical/sing-box`      | Multi-proxy transparent proxy (mixed:1080, redirect:1081, tun:1080 on `tun0/172.19.0.1`)             |
| `nginx`         | `umbilical/openresty`     | OpenResty (nginx + Lua) ingress: TLS, vouch auth, per-user RBAC                                      |
| `vouch`         | `umbilical/vouch`         | Vouch-proxy; Google OAuth authentication backend on port 9090                                        |
| `wakeup`        | `umbilical/wakeup`        | GoTTY web terminal on port 37902; sends Wake-on-LAN magic packets to silver                          |

#### Reverse SSH Tunnels (`tunnel/`, `tunnel-ssh/`)

`local/router/tunnel/entrypoint.sh` runs four autossh processes that maintain persistent reverse tunnels from `jserver.public.<domain>` to local machines:

- `jserver:eth0:80` and `:443` forwarded to `r3b:80/443` (HTTP/HTTPS ingress)
- `jserver:eth0:1083` forwarded to `r3b:1083` (proxy port)
- `jserver:eth0:3737/3738/3739` forwarded to `silver:3737/3738/3739`

`tunnel-ssh/entrypoint.sh` adds a separate tunnel for `jserver:eth0:3701` to `r3b:3701`. This multi-tunnel design lets the cloud nginx edge forward authenticated public HTTPS traffic back to the home network.

#### DDNS (`ddns/`)

`main.py` (Python 3.12, `python:3.12-slim` image) polls `http://members.3322.org/dyndns/getip` every 120 seconds and uses the Aliyun Python SDK to update A records for the configured subdomains (`direct`, `*.direct`) whenever the home ISP IP changes. Credentials are injected as environment variables at runtime.

#### ipset-watcher (`ipset-watcher/`)

`run.sh` calls `watch.sh` every 600 seconds. `watch.sh` compares the live `priority` ipset with a freshly generated IP list and performs an incremental update if they differ. `gen_ips.priority.sh` populates the list by DNS-resolving `jserver.public.<domain>` and `home.<domain>`. These IPs are used by `scripts/iptables/iptables-priority.sh` to install mangle rules that mark packets destined for them with `0x200`, so they bypass the sing-box TUN via `ip rule` policy routing (priority 8000, main routing table).


#### OpenResty / nginx (`nginx/`)

OpenResty serves all authenticated web services on 443 (TLS certificates from `static/letsencrypt/`). Port 80 redirects unconditionally to HTTPS. Virtual hosts configured in `nginx.conf`:

| Subdomain                  | Backend                                                 | Auth                         |
|----------------------------|---------------------------------------------------------|------------------------------|
| `vouch.<domain>`           | vouch-proxy on `r3b:9090`                               | none (public OAuth callback) |
| `<domain>` (root)          | static `index.html` landing page                        | vouch + Lua RBAC             |
| `cockpit.<domain>`         | silver Cockpit UI on `silver.<domain>:9090` (WebSocket) | vouch + Lua RBAC             |
| `wakeup.<domain>`          | wakeup GoTTY on `r3b:37902` (WebSocket)                 | vouch + Lua RBAC             |
| `shutdown-silver.<domain>` | shutdown GoTTY on `silver.<domain>:37901` (WebSocket)   | vouch + Lua RBAC             |

The `conf/vouch.conf` include adds `auth_request /validate` to each protected server block; `conf/vouch-auth.conf` passes the resolved user to the Lua script.

#### Lua RBAC (`nginx/configs/lua/vouch-auth.lua`)

After vouch validates the Google OAuth token and sets the `X-Vouch-User` header, a Lua script enforces per-user service access. Users with value `"*"` can access everything. Others carry a set of allowed subdomain prefixes (`"@"` represents the root domain, `"stable-diffusion"` etc. for specific services). Unrecognized users or out-of-scope subdomains receive HTTP 403.

#### vouch (`vouch/`)

`config.yml` configures vouch-proxy with Google OAuth (`provider: google`), a domain whitelist, a per-user email whitelist, and a 7-day (10080 minute) secure cookie. The OAuth callback URL is `https://vouch.<domain>/auth`.

## Silver Stack (`local/silver/`)

The silver machine runs a two-container stack:

| Service      | Image                | Port         | Role                                                                                          |
|--------------|----------------------|--------------|-----------------------------------------------------------------------------------------------|
| `tunnel-ssh` | `umbilical/tunnel`   | —            | autossh reverse tunnel exposing silver port 3737 to `jserver:eth0:37371`                      |
| `shutdown`   | `umbilical/shutdown` | 37901 (host) | GoTTY web terminal; exposes a tmux-wrapped `systemctl poweroff` with interactive confirmation |

The shutdown container bind-mounts `/bin/systemctl`, `/run/systemd/system`, `/var/run/dbus/system_bus_socket`, and `/sys/fs/cgroup` from the host to execute `systemctl poweroff` inside the container. An optional auto-shutdown mode (`arg=true`) waits 30 minutes, then aborts if the current time is outside 00:00–06:00.

Deployment: `local/silver/sync.sh` rsync-copies the rendered config to the `silver` host; `restart.sh` and `stop.sh` manage the compose stack remotely.

## Custom Images (`images/`)

All images are built with `scripts/my-build-image.sh`, which wraps `docker buildx build` for `linux/amd64,linux/arm64/v8` multi-arch builds and pushes to the private registry:

```bash
cd images/<name>
./build.sh [--no-cache] [--platform linux/amd64]
```

| Image           | Base                               | Added tooling                                                             |
|-----------------|------------------------------------|---------------------------------------------------------------------------|
| `nginx`         | `nginx:latest`                     | none (re-tagged for private registry)                                     |
| `openresty`     | `openresty/openresty:buster-fat`   | `/var/log/nginx` directory                                                |
| `wireguard`     | `ghcr.io/wg-easy/wg-easy:latest`   | none                                                                      |
| `tunnel`        | `alpine:3.10`                      | `openssh-client`, `autossh`                                               |
| `ddns`          | `python:3.12-slim`                 | `aliyun-python-sdk-core`, `aliyun-python-sdk-alidns`, `requests`          |
| `ipset-watcher` | `alpine:3.23`                      | `bash`, `ipset`, `curl`, `bind-tools`, `python3`                          |
| `sing-box`      | `ghcr.io/sagernet/sing-box:latest` | none                                                                      |
| `vouch`         | `voucher/vouch-proxy:latest-arm`   | none                                                                      |
| `wakeup`        | `ubuntu:22.04`                     | `etherwake`, `net-tools`, `netcat`, GoTTY v2.0.0-alpha.3 (ARM), `kubectl` |
| `shutdown`      | `umbilical/ubuntu:24.04`           | `tmux`, GoTTY v2.0.0-alpha.3 (amd64)                                      |
| `ubuntu/24.04`  | `ubuntu:24.04`                     | shared base for shutdown image                                            |

`my-build-image.sh` reads the private registry host from `bwww`, creates a named buildx builder, rsyncs sources to a temporary build directory under `/mnt/coder-sharepoint/build-image/`, and removes the directory after a successful push.

## Network Architecture

```
Internet
   │
   ▼
jserver (cloud VM)
├── nginx:25443  ── TLS termination ──► router:80/443 (via autossh tunnel)
├── nginx:443    ── wireguard UI proxy
└── WireGuard:51820/udp ── VPN peers

router (r3b, on-premises)
├── OpenResty:443 ── vouch auth ──► cockpit / wakeup / shutdown-silver
├── vouch-proxy:9090
├── sing-box tun0:172.19.0.1 ── transparent proxy (CN direct, non-CN → Balancer)
├── ipset 'priority' ── iptables bypass for jserver/home IPs (mark 0x200)
├── DDNS (every 2 min) ──► Aliyun DNS API
└── autossh tunnels ──► jserver (ports 80/443/1083/3701/3737-3739)

silver (on-premises)
├── shutdown GoTTY:37901 ── remote poweroff via browser
└── autossh tunnel ──► jserver:37371
```

## Deployment

### Prerequisites

- `bwww` CLI for rendering `__{{...}}__` placeholders
- SSH access to `jserver` (alias `umbilical-jserver`), `r3b`, and `silver`
- Docker with buildx for multi-arch image builds

### Sync and restart the cloud stack

```bash
cd umbilical/cloud
./sync.sh     # render + rsync to jserver
./restart.sh  # docker compose up -d on jserver
```

### Sync and restart the router stack

```bash
cd umbilical/local/router
./sync.sh          # render + rsync to r3b
./restart.sh [service]   # restart one or all services
./stop.sh          # docker compose down on r3b
```

### Sync and restart the silver stack

```bash
cd umbilical/local/silver
./sync.sh
./restart.sh
```

### Build and push a custom image

```bash
cd umbilical/images/tunnel
./build.sh
# or with options:
./build.sh --no-cache --platform linux/arm64/v8
```

### Update sing-box proxy subscriptions

```bash
cd umbilical/local/router/parse-singbox-nexitally
python3.12 gen_outbound_config.py   # writes 20_outbounds_*.json + updates 30_outbounds_selectors.json
```

### Update sing-box geo rule sets

```bash
bash umbilical/local/router/sing-box/rule-sets/download.sh
```

### Apply iptables priority bypass rules

Run on the router host (not inside Docker):

```bash
bash umbilical/local/router/scripts/iptables/iptables-priority.sh
```

## Related

- [../qosmon/README.md](../qosmon/README.md) — Infrastructure health monitor that validates ingress endpoints managed here
- [../README.md](../README.md) — Repository overview
