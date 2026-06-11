# Koishi Cluster

Koishi is a personal home-lab Kubernetes cluster managed as a fully declarative monorepo. Every service, secret reference, network rule, and build pipeline is expressed in code. The cluster runs across multiple physical nodes (`nur`, `miniba`, `nippon`, `sa`) and is designed around five guiding principles that make every service accessible from anywhere, securely, via a plain web browser.

## Concept

- **Web-First (Web >>> App)**: Every service that normally requires a desktop or mobile application is replaced with, or supplemented by, a browser-accessible equivalent—DBGate for databases, Coder/code-server for IDEs, Guacamole for remote desktops, AriaNg and VueTorrent for download management, openlist for file browsing, JupyterLab for notebooks. No specialized client is ever required.
- **Access From Anywhere**: A layered connectivity stack (Cloudflare proxy, external edge relay, WireGuard, DDNS, Traefik ingress) ensures every internal service is reachable from any network without manual VPN configuration.
- **HTTPS Everywhere**: TLS is mandatory and automatic. cert-manager issues wildcard and per-domain certificates via Let's Encrypt DNS-01 challenges resolved through the Cloudflare API. HTTP is globally redirected to HTTPS at the Traefik entrypoint.
- **No Secrets in Code**: Sensitive values are never committed. All `__{{...}}__` placeholders in YAML and Dockerfile sources are rendered at deploy time by the `bww` client against a self-hosted Vaultwarden vault. The git history is clean by design; the `infra/` hooks enforce this with a pre-commit keyword blacklist.
- **Infra as Code**: The entire cluster state—networking, storage, builds, schedules, RBAC, DNS—is defined in this repository and applied via `my-k8s-deploy`.

---

## Network Topology and Data Flow

```
Public Internet
    │  HTTPS (443)
    ▼
Cloudflare Proxy  ──── NetworkPolicy allows only Cloudflare CIDRs ────►
    │                                                                    │
    ▼                                                          Traefik ingress (3 replicas,
External Edge Relay ("umbilical")                              spread across traefik-node=true nodes)
  cloud VPS: nginx/OpenResty + Vouch auth + sing-box               │
  ↕ WireGuard tunnel to home cluster                         TLS termination (cert-manager)
  Wake-on-LAN wakeup / shutdown control                            │
    │                                                        SSO forward-auth middleware
    │                                                        (traefik-forward-auth per domain)
    ▼                                                              │
Home Cluster (k3s)                                          Custom middleware
  DDNS CronJobs keep Cloudflare and Aliyun DNS current       (antimitm IP filter / playonjan KV filter)
  Calico CNI: fine-grained NetworkPolicy (IPv6 not yet active)    │
                                                             Sablier (scale-to-zero / on-demand wake)
                                                                   │
                                                             Workload pod (default namespace or
                                                             dedicated namespace: database, sso,
                                                             middleware, finance, ck-dev, ck-prod…)
```

### Request path step by step

1. **DNS**: Cloudflare resolves `*.domain` to the cluster's home IP, maintained by the DDNS service (two CronJobs running every 5 minutes—one for Cloudflare, one for Aliyun).
2. **Edge relay (umbilical)**: The sibling `umbilical` project runs a cloud VPS with nginx/OpenResty as a TLS-terminating reverse proxy and Vouch-based OAuth gating. A WireGuard tunnel connects the VPS back to the home cluster so traffic that cannot reach the cluster directly can be relayed. The VPS also provides Wake-on-LAN control (etherwake) and a coordinated shutdown interface for home nodes.
3. **Cloudflare NetworkPolicy**: Traefik pods accept ingress only from the 15 published Cloudflare IPv4 CIDR blocks plus private RFC-1918 ranges. Note: IPv6 Cloudflare ranges are not yet included — the NetworkPolicy carries the explicit comment *"IPv6 is not supported in Koishi yet"*, so requests arriving via Cloudflare's IPv6 addresses are rejected. The Traefik management dashboard (port 9000) is further restricted to LAN-only by a separate NetworkPolicy rule.
4. **Traefik**: The single HTTP(S) entrypoint. Deployed as a 3-replica Deployment using `HelmChartConfig` (k3s), spread with pod anti-affinity across `traefik-node`-labelled nodes. Ports: 80 → permanent redirect to 443; 443 HTTPS; 9000 dashboard; 7030/TCP and 7070/UDP for TCP/UDP passthrough entrypoints. Logs Cloudflare headers for tracing. In addition to LAN-only access on port 9000, the Traefik dashboard is exposed at `traefik.<domain>` via a dedicated public HTTPS Ingress (`k8s.traefik-dashboard.yaml`) routing paths `/dashboard` and `/api` to port 9000, protected by the full `playonjan + antimitm + sso-domainx` middleware chain. The raw port 9000 is reachable only from LAN; the public URL requires SSO authentication through Traefik itself.
5. **TLS termination**: cert-manager uses the `cluster-letsencrypt-dns-cloudflare` ClusterIssuer (DNS-01 challenge via Cloudflare API token) to provision and auto-renew certificates. A global `noindex` middleware injects `X-Robots-Tag: noindex` on all responses.
6. **SSO / forward-auth**: Six independent `traefik-forward-auth` deployments (one per domain: `domainx`, `domaint`, `domainy`, `domainp`, `domainc`, `domaind`) each backed by separate Google OAuth2 credentials and email whitelists. Domain X alone guards 20+ services with context-aware rules (e.g. `vaultwarden/admin` requires `hh` only; dev tools whitelist `hh` and `hz`; general services allow four email addresses). Session cookies have a 1-year lifetime (`LIFETIME=31536000`).
7. **Custom middleware**: A small Go service (`kubernetes/middleware`) exposes two Traefik `forwardAuth` middlewares: `antimitm` (IP-range filter via `IP_FILTER1_FILE`) and `playonjan` (key-value filter). These are applied to the Traefik dashboard, download tools, and other sensitive routes.
8. **Sablier**: An in-cluster scale-to-zero controller (`sablierapp/sablier:1.11.1`) patches Deployment/StatefulSet replicas. Idle workloads are scaled to 0; the first HTTP request triggers a wake with a configurable session TTL (typically 1 h). Used by: Calibre-Web, Coder, n8n, JupyterLab, WebDAV, Gollum, Guacamole, download stack, Bliss, OpenWebUI, and more.
9. **Calico CNI**: An IPv6 IPPool manifest (`default-ipv6-ippool`, `2001:cafe:42::/56`, `natOutgoing: true`) exists in the repository but carries the comment *"this config was generated but has not been used"*. The Traefik NetworkPolicy similarly notes *"IPv6 is not supported in Koishi yet"* and lists only IPv4 Cloudflare CIDR blocks — Cloudflare-proxied IPv6 connections are rejected by the NetworkPolicy as currently deployed. IPv6 end-to-end is planned but not yet active. NetworkPolicy is used at the workload level (e.g. dbgate egress is restricted to kube-dns, Traefik, and specific database namespaces and AWS RDS IPs; Vaultwarden has no egress).

---

## Cluster Infrastructure

### Nodes

| Node     | Role / Notes                                                           |
|----------|------------------------------------------------------------------------|
| `nur`    | Primary worker; most stateful workloads, NFS server, Kaniko build node |
| `miniba` | GPU-capable; Ollama and JupyterLab pinned here                         |
| `nippon` | Secondary NFS server (download data); dnsmasq split-view DNS           |
| `sa`     | Additional node; devbox template supports deployment here              |

### Traefik

Deployed via k3s `HelmChartConfig`. Three replicas with node affinity (`traefik-node=true`) and pod anti-affinity to spread across nodes. `externalTrafficPolicy: Local` to preserve source IPs. Cross-namespace CRD routing enabled (`allowCrossNamespace: true`).

### cert-manager

`ClusterIssuer` `cluster-letsencrypt-dns-cloudflare` uses the Cloudflare API token (stored as a Kubernetes Secret rendered from Vaultwarden) for DNS-01 ACME. Covers wildcard and per-subdomain certificates for all six domains.

### SSO and Custom Middleware

Six `traefik-forward-auth` instances in the `sso` namespace, one per domain group, each with isolated Google OAuth2 credentials. The custom Go middleware service in the `middleware` namespace implements IP filtering (`/ipfilter1`) and KV filtering (`/kvfilter1`) endpoints, wrapped as Traefik `forwardAuth` middlewares `antimitm` and `playonjan`.

### Sablier

`sablierapp/sablier:1.11.1` in the `sablier` namespace. ClusterRole grants `get/list/watch/patch/update` on Deployments and StatefulSets. Services declare `replicas: 0` and attach a Sablier Traefik middleware specifying session duration and UI theme.

### Calico

`v3.30.5` CNI. NetworkPolicy is applied per namespace to enforce least-privilege egress/ingress at the pod level.

**Note on IPv6 status**: The IPv6 IPPool manifest (`calico-ipv6-ippool.yaml`, pool `default-ipv6-ippool`, CIDR `2001:cafe:42::/56`) exists in the repository but carries the comment *"this config was generated but has not been used"*. The Traefik NetworkPolicy similarly notes *"IPv6 is not supported in Koishi yet"* and lists only IPv4 Cloudflare CIDR blocks. IPv6 end-to-end is planned but not yet active; Cloudflare-proxied IPv6 connections will be rejected by the NetworkPolicy as currently deployed.

### Argo Workflows

`v3.5.4` in the `argo` namespace for pipeline orchestration.

### Storage

NFS PersistentVolumes (all with `Retain` reclaim policy) exported from `nur` and `nippon`:

| PV                          | Server | Capacity | Use                         |
|-----------------------------|--------|----------|-----------------------------|
| `nfs-disk-user-data-others` | nur    | 100 Gi   | Media, misc user data       |
| `nfs-disk-user-data-music`  | nur    | 100 Gi   | Music library               |
| `nfs-disk-user-data-slight` | nur    | 100 Gi   | Photos, foldersync          |
| `nfs-disk-coder-sharepoint` | nur    | 200 Gi   | Shared code/build workspace |
| `nfs-disk-coder-workspaces` | nur    | 100 Gi   | Developer workspaces        |
| `nfs-data-download`         | nippon | 120 Gi   | Torrent / download output   |

Local hostPath mounts on individual nodes (`/mnt/runtime-data-app/*`, `/mnt/user-data-*`) are used for node-affined stateful workloads (Postgres, Ollama models, WireGuard config, etc.).

### Self-healing CronJobs

- `reschedule` (every 10 min): lists Deployments and Nodes, reschedules workloads across healthy nodes.
- `restart` (every 5 min): performs pod-restart orchestration with emptyDir state tracking.

### TLS Certificate Refresh (Aliyun DNS)

`letsencrypt-refresh/` contains a Kubernetes Job that uses `certbot` with the `certbot-dns-aliyun` plugin to issue wildcard certificates (`*.domain`, `*.public.domain`, `*.silver.domain`, `*.direct.domain`, `*.j.domain`) via an Aliyun DNS-01 challenge. The Job runs a two-stage pipeline: an `initContainer` runs certbot, then a `copy` container places the PEM files (`cert.pem`, `chain.pem`, `fullchain.pem`, `privkey.pem`) at a configured host path for consumption by the `home-service` deployment. Aliyun access key and secret are injected from Vaultwarden. This is a second, manually-triggered certificate issuance path that complements the in-cluster cert-manager `ClusterIssuer` used for the main wildcard domains.

### Image Registry GCP Cleanup

`kubernetes/registry/gcp/` contains a `cleanup-cache.json` policy and `run.sh` script that apply a GCP Artifact Registry cleanup policy to the `docker` repository. The policy deletes `cache/kaniko` images older than 72 hours (regardless of tag state), keeping the Kaniko layer cache from growing unbounded in the `cloudprivate` GCP registry.

---

## Build and Dev

### In-cluster Kaniko Builds

`scripts/my-k8s-build-image` orchestrates builds without a privileged Docker daemon. It:

1. Fetches registry credentials and secrets from Vaultwarden via `bwww`.
2. Copies source to a shared build directory on `coder-sharepoint`.
3. Renders all `__{{...}}__` placeholders in source files before build.
4. Creates a Kaniko `Job` (`gcr.io/kaniko-project/executor:v1.24.0`) pinned to `nur`, with a randomised name (`kaniko-build-<timestamp>-<random>`).
5. Pushes to `cloudpublic` (GitHub Container Registry) or `cloudprivate` (GCP Artifact Registry).

Kaniko cache is stored in the `cloudprivate` GCP Artifact Registry with a 24-hour TTL. A separate GCP cleanup policy (`kubernetes/registry/gcp/`) deletes `cache/kaniko` layers older than 72 hours to bound cache growth.

### Image Registry

Two registries are used:

- `cloudpublic` — GitHub Container Registry; open images (base images, most services).
- `cloudprivate` — GCP Artifact Registry; private or proprietary images. Secrets injected as `imagePullSecrets` at deploy time by `deploy/deploy.rb`.

### Watchtower

Runs via `docker-compose` on each node. Polls every 120 seconds, mounts `/var/run/docker.sock` and `~/.docker/config.json` for registry authentication, and auto-updates running containers.

### Nix and Devbox

`devbox/default.nix` declares the developer shell with reproducible tool versions: `kubectl`, `helm`, `kubectx`, `emacs30`, `ffmpeg`, `imagemagick`, `gopls`, `ruff`, `pandoc`, `ranger`, and more. A lazy-loaded `default.editor.nix` brings in Go, Node.js 22, JDK 21, and LSP tooling. `entrypoint.box.sh` bootstraps the container by running Ansible playbooks, setting `asdf` tool versions (Ruby 3.2.2, Python 3.12.4), and installing the Nix environment. Base images in `base-images/` provide Ubuntu 24.04, Debian Bookworm, Alpine, Go, and Flutter builder images, all built with Kaniko and tagged to `cloudpublic`.

---

## Security

### Secret Management: Vaultwarden + bww

Vaultwarden (`vaultwarden/server:latest`) is the cluster's secret backend, deployed in the `default-vaultwarden` namespace with a strict NetworkPolicy that blocks all pod egress. A custom Go API wrapper, **bww** (BitWarden for Webapp), wraps the Bitwarden CLI to provide an HTTP endpoint that renders template placeholders:

```
__{{item-name:f:field-name}}__        → custom field value
__{{item-name:a:filename:a:b64}}__    → attachment, base64-encoded
__{{item-name:f:field:f:b64}}__       → field value, base64-encoded
```

HMAC-SHA256 request authentication (headers `X-BWW-Timestamp` + `X-BWW-Signature`) with a 5-minute clock window prevents replay attacks. The `bwww` Ruby shell client signs requests and is invoked by `my-k8s-build-image` and `my-k8s-deploy` before any secrets touch disk.

Vault data is backed up nightly by two mechanisms:

- **Portwarden** CronJob (00:30 UTC): item-level encrypted exports.
- **vaultwardendatabackup** Job: full data directory zip, GPG-encrypted with a SHA256-derived passphrase fetched from an internal `fs-downserver` endpoint.

### MFA SSH

SSH access to the devbox environment requires both a password (injected via Kubernetes Secret) and Google Authenticator TOTP (`libpam-google-authenticator`).

### Least Privilege

The devbox operates as the unprivileged `box` user. `kubectl` and `docker` commands require `sudo`. The Kaniko build node credentials are gated behind `bwww-source-env`, which reads encrypted credentials via `sudo` to prevent plaintext exposure in shell history. RBAC is scoped per component (devbox has `cluster-admin` for operator workflows; reschedule/restart CronJobs have minimal patch-only permissions).

---

## Data

### Databases

| Cluster                  | Namespace  | Instances | Storage                 | Backup                                          |
|--------------------------|------------|-----------|-------------------------|-------------------------------------------------|
| Local Postgres (pg 18.1) | `database` | 1         | hostPath `nur`          | —                                               |
| CNPG ck-dev              | `ck-dev`   | 2         | 1 Gi local-path-retain  | GCS `koishi-cnpg-backup-dev`, 3-day retention   |
| CNPG ck-prod             | `ck-prod`  | 3         | 10 Gi local-path-retain | GCS `koishi-cnpg-backup-prod`, 30-day retention |

CloudNativePG (`release-1.29`) with the `barman-cloud` plugin provides continuous WAL archiving (gzip) and scheduled daily backups to Google Cloud Storage. Managed roles are declared in the cluster spec.

Redis Stack is deployed in both `ck-dev` and `ck-prod` namespaces for caching (LiteLLM response cache) and vector storage (prag RAG index).

DBGate provides a web-based DBA UI with connections to all PostgreSQL clusters (local, ck-dev, ck-prod), MySQL (AWS RDS), MSSQL, and SQLite. Three variants: full, lite (Sablier scale-to-zero), and cc (single-user). Strict egress NetworkPolicy limits dbgate outbound to kube-dns, Traefik, defined namespaces, and specific AWS RDS IPs.

### Cloud Storage and Backups

`media/rclone` runs a privileged pod (SYS_ADMIN + `privileged: true`) with FUSE mounts:

- `/mnt/webdav` — openlist + PikPak WebDAV
- `/mnt/remote` — cloud remotes (Google Drive, Aliyun)
- `/mnt/decrypted` — Rclone-encrypted backup target (transparent to consumers)

All cloud-bound data is encrypted at the rclone layer before leaving the cluster. Backup pipeline:

1. **backup-prepare** CronJob (18:00 UTC): compresses Plex metadata and codebase to an encrypted `ebackup/` directory.
2. **backup-upload** CronJob (08:00 UTC): triggers `rclone.py` inside the rclone pod to push to PikPak and Google Drive.

`media/aliyunpan` handles Aliyun cloud storage: an `encfs`-encrypted upload container and a decrypt/download container, using `encfs6.xml` for the encryption config.

---

## Service Catalog

### Media

| Service  | Subdirectory     | Notes                                                            |
|----------|------------------|------------------------------------------------------------------|
| Jellyfin | `media/jellyfin` | Video streaming; waits for PikPak WebDAV mount via initContainer |
| Plex     | `media/plex`     | Media server; library aggregated from local NFS + cloud          |
| Roon     | `media/roon`     | Audio server; `hostNetwork: true` for low-latency port access    |
| Bliss    | `media/bliss`    | Music tagger; Sablier scale-to-zero, 1 h session                 |
| openlist | `media/openlist` | File browser/manager (v4.1.1); 20 GB upload limit                |
| Samba    | `media/samba`    | SMB shares; LoadBalancer TCP/UDP 137-139, 445 for LAN access     |
| MediaMTX | `mediamtx/`      | RTSP/HLS/WebRTC streaming server; on-demand ffmpeg transcoding   |

### Download

| Service                  | Subdirectory                                              | Notes                                                  |
|--------------------------|-----------------------------------------------------------|--------------------------------------------------------|
| qBittorrent + VueTorrent | `download/qbittorrent`, `download/qbittorrent-vuetorrent` | Torrent client; 120 Gi NFS download volume             |
| Aria2 + AriaNg           | `download/aria2`, `download/ariang`                       | HTTP/FTP downloads; auto-fetches tracker list on start |
| Jackett                  | `download/jackett`                                        | Torrent search aggregator                              |
| FlareSolverr             | `download/flaresolverr`                                   | Cloudflare/anti-bot bypass for Jackett                 |

All download services use Sablier (grouped as `download`), start at 0 replicas, and are pinned to `nur`.

### AI

| Service    | Subdirectory | Notes                                                                                                                                                                                          |
|------------|--------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| LiteLLM    | `litellm/`   | LLM gateway; routes to Ollama (local), Gemini, optional Anthropic; Redis response cache; rate limiting                                                                                         |
| Ollama     | `ollama/`    | Local inference on `miniba` (GPU node); OLLAMA_NUM_PARALLEL=4; 12 CPU / 8 Gi limit                                                                                                             |
| Open WebUI | `ollama/`    | Browser chat UI for Ollama; Sablier scale-to-zero                                                                                                                                              |
| prag       | `prag/`      | RAG service: indexes markdown notes, embeds via Ollama bge-m3, stores in Redis HNSW, serves `/query` via FastAPI                                                                               |
| aiagent    | `aiagent/`   | Generic AI agent framework using `pydantic-ai-skills`; dynamically loads skills (Python/Node); WebUI + REST + WebSocket; routes through LiteLLM — see [`aiagent/README.md`](aiagent/README.md) |

### Productivity and Tools

| Service              | Subdirectory             | Notes                                                                                              |
|----------------------|--------------------------|----------------------------------------------------------------------------------------------------|
| n8n                  | `n8n/`                   | Workflow automation; Sablier 1 h, persistent PVC                                                   |
| Gollum               | `gollum/`                | Git-backed wiki; read-only mount of notebook repo                                                  |
| JupyterLab           | `jupyterlab/`            | PyTorch and TensorFlow instances on `miniba`; Sablier 1 h; token + SSO auth                        |
| Guacamole            | `guacamole/`             | Clientless RDP/VNC/SSH gateway; PostgreSQL session store; Sablier 1 h                              |
| Coder                | `coder/`                 | Remote dev workspace platform (Helm); GitHub OAuth only; Sablier 1 h                               |
| code-server (devbox) | `devbox/`                | VS Code in browser, part of devbox; SSH + MFA access                                               |
| Calibre-Web          | `calibre-web-automated/` | E-book library; Sablier 1 h                                                                        |
| Markviewer           | `markviewer/`            | Caddy-based Markdown renderer for notebook/openclaw workspaces                                     |
| WebDAV               | `webdav/`                | Per-user Caddy WebDAV server; 20 GB limit; Sablier 2 h; 4 users                                    |
| FolderSync           | `foldersync/`            | SSH/SFTP ingress for photo sync and notebook sharing; LoadBalancer port 37000                      |
| FS (file tools)      | `fs/`                    | Static downserver, Flask upserver (Sablier), and code-server IDE for file exchange                 |
| openclaw             | `openclaw/`              | AI agent framework (Node.js); dual instances (default + KZK/LINE webhook); Caddy workspace sidecar |
| Fountain scanner     | `ck/`                    | WASM-compiled Rust scanner frontend served by Nginx; deployed to `ck-prod` namespace               |

### Finance

| Service | Subdirectory     | Notes                                                                                                          |
|---------|------------------|----------------------------------------------------------------------------------------------------------------|
| Seek    | `finance/seek/`  | FastAPI price query service; dual ingress (internal + API-key-gated external)                                  |
| Tempo   | `finance/tempo/` | Daily CronJob fetching OHLCV data from Yahoo Finance (GOOG, SPY, VGT, BTC-USD, ETH-USD, ADA-USD) into Postgres |

### Networking

| Service                 | Subdirectory         | Notes                                                                                                                                                                                                                   |
|-------------------------|----------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| WireGuard               | `wireguard/`         | VPN endpoint (wg-easy); LoadBalancer UDP 37004; web UI SSO-protected                                                                                                                                                    |
| wireproxy               | `wireproxy/`         | Userspace WireGuard SOCKS5 (port 1080) + HTTP (port 8080) proxy; requires NET_ADMIN                                                                                                                                     |
| xray                    | `xray/`              | VMess proxy (xray-core 25.12.8); two inbounds—`koishi` (direct egress) and `umbilical` (via wireproxy); WebSocket transport                                                                                             |
| DDNS                    | `ddns/`              | CronJobs every 5 min for Cloudflare (6 domains) and Aliyun (1 domain); IPv4 + IPv6 infrastructure                                                                                                                       |
| ip2location             | `ip2location/`       | Go HTTP geolocation API; IP2Location LITE DB1 embedded in image                                                                                                                                                         |
| External services       | `external-services/` | K8s Service+EndpointSlice wrappers for routers, Pi-KVM, VMs, cockpit, shutdown/wakeup                                                                                                                                   |
| dnsmasq                 | `docker-compose/`    | Split-view DNS for internal domains; runs on `nippon` via docker-compose                                                                                                                                                |
| Homepage                | `homepage/`          | Cluster dashboard (gethomepage); ClusterRole for live Kubernetes metrics                                                                                                                                                |
| macbind                 | `macbind/`           | One-shot Kubernetes Job; reads a notebook-mounted `macip-mapping.json` and reconciles MAC-to-IP bindings in the NTT router's DHCP static-assignment table via its web management API; validates entries before applying |
| Docker Compose services | `docker-compose/`    | Watchtower, comapi metrics API, shutdown/wakeup gotty terminals per node                                                                                                                                                |

---

## Deploy Workflow

Each service contains a `build.sh` and/or `deploy.sh`. The canonical pattern:

```bash
# Build container image in-cluster via Kaniko
./build.sh                          # calls my-k8s-build-image <image-name>

# Render secrets and apply Kubernetes manifests
./deploy.sh                         # calls my-k8s-deploy --file=k8s.app.yaml
```

`my-k8s-deploy` delegates to `deploy/deploy.rb`, which:

- Substitutes `__{{...}}__` placeholders via Vaultwarden/bww.
- Rewrites `cloudpublic/` and `cloudprivate/` image prefixes to full registry URLs.
- Injects `imagePullSecrets`, timezone, `terminationGracePeriodSeconds`, `nodeSelector` (defaulting to `amd64`; switches to `arm64` for ARM images), and `RollingUpdate` strategy.
- Applies via `kubectl apply` and waits for rollout with a 300-second timeout.

Docker Compose services on each node are synced via `docker-compose/sync.sh`, which renders secret templates with `my-secret render` then rsyncs the rendered configs to the target hosts.

---

## Related Projects

- [`../qosmon/`](../qosmon/) — Rust-based infrastructure health monitor that validates HTTP endpoints, TCP ports, DNS records, SSL certificates, and `noindex` headers across all Koishi ingresses. Auto-generates check configs from live Kubernetes Ingress/Service resources.
- [`../umbilical/`](../umbilical/) — External edge relay: cloud VPS running nginx/OpenResty + Vouch OAuth + sing-box proxy selector, connected to the home cluster via WireGuard reverse tunnel. Provides the public entry point when direct Cloudflare routing is unavailable and manages Wake-on-LAN and shutdown for home nodes.
- [`../infra/`](../infra/) — Git hooks (`pre-commit`, `commit-msg`) that scan staged files and commit messages against a user-configurable blackword list to prevent accidental secret commits.
