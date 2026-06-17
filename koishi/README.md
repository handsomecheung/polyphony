# Koishi Cluster

Koishi is a personal home-lab Kubernetes cluster managed as a fully declarative monorepo. Every service, secret reference, network rule, and build pipeline is expressed in code. The cluster runs across multiple physical nodes (`nur`, `miniba`, `nippon`) and is designed around five guiding principles that make every service accessible from anywhere, securely, via a plain web browser.

## Concept

- **Web-First (Web >>> App)**: Every service that normally requires a desktop or mobile application is replaced with, or supplemented by, a browser-accessible equivalent—DBGate for databases, Coder/code-server for IDEs, Guacamole for remote desktops, openlist for file browsing, JupyterLab for notebooks. No specialized client is ever required.
- **Access From Anywhere**: A layered connectivity stack (Cloudflare proxy, WireGuard, DDNS, Traefik ingress) ensures every internal service is reachable from any network without manual VPN configuration.
- **HTTPS Everywhere**: TLS is mandatory and automatic. cert-manager issues wildcard and per-domain certificates via Let's Encrypt DNS-01 challenges resolved through the Cloudflare API. HTTP is globally redirected to HTTPS at the Traefik entrypoint.
- **No Secrets in Code**: Sensitive values are never committed. All `__{{...}}__` placeholders in YAML and Dockerfile sources are rendered at deploy time by the `bww` client against a self-hosted Vaultwarden vault. The git history is clean by design; the `infra/` hooks enforce this with a pre-commit keyword blacklist.
- **Infra as Code**: The entire cluster state—networking, storage, builds, schedules, RBAC, DNS—is defined in this repository and applied via `my-k8s-deploy`.

---

## Network Topology and Data Flow

```
Public Internet
    │  HTTPS (443)
    ▼
Cloudflare Proxy  ──── NetworkPolicy allows only Cloudflare CIDRs ────► Traefik ingress (3 replicas,
    │                                                                   spread across traefik-node=true nodes)
    ▼                                                                         │
Home Cluster (k3s)                                                      TLS termination (cert-manager)
  DDNS CronJobs keep Cloudflare DNS current                        │
  Calico CNI: fine-grained NetworkPolicy (IPv6 not yet active)          SSO forward-auth middleware
                                                                        (traefik-forward-auth per domain)
                                                                              │
                                                                        Custom middleware
                                                                        (antimitm IP filter / playonjan KV filter)
                                                                              │
                                                                        Sablier (scale-to-zero / on-demand wake)
                                                                              │
                                                                        Workload pod (default namespace or
                                                                        dedicated namespace: database, sso,
                                                                        middleware, ddns, finance …)
```

### Request path step by step

1. **Calico CNI**: The Traefik NetworkPolicy similarly notes *"IPv6 is not supported in Koishi yet"* and lists only IPv4 Cloudflare CIDR blocks — Cloudflare-proxied IPv6 connections are rejected by the NetworkPolicy as currently deployed. IPv6 end-to-end is planned but not yet active. NetworkPolicy is used at the workload level.
2. **DNS**: Cloudflare resolves `*.domain` to the cluster's home IP, maintained by the DDNS service (one CronJob running every 5 minutes—one for Cloudflare).
3. **Cloudflare NetworkPolicy**: Traefik pods accept ingress only from the 15 published Cloudflare IPv4 CIDR blocks plus private RFC-1918 ranges. IPv6 Cloudflare ranges are not yet included — the NetworkPolicy carries the explicit comment *"IPv6 is not supported in Koishi yet"*, so requests arriving via Cloudflare's IPv6 addresses are rejected. The Traefik management dashboard (port 8080) is further restricted to LAN-only by a separate NetworkPolicy rule.
4. **TLS termination**: cert-manager uses the `cluster-letsencrypt-dns-cloudflare` ClusterIssuer (DNS-01 challenge via Cloudflare API token) to provision and auto-renew certificates. A global `noindex` middleware injects `X-Robots-Tag: noindex` on all responses.
5. **Traefik**: The single HTTP(S) entrypoint. Deployed as a 3-replica Deployment using `HelmChartConfig`, spread with pod anti-affinity across `traefik-node`-labelled nodes. Ports: 80 → permanent redirect to 443; 443 HTTPS; 8080 dashboard; 7030/TCP and 7070/UDP for TCP/UDP passthrough entrypoints. Logs Cloudflare headers for tracing. In addition to LAN-only access on port 8080, the Traefik dashboard is exposed at `traefik.<domain>` via a dedicated public HTTPS Ingress (`k8s.traefik-dashboard.yaml`) routing paths `/dashboard` and `/api` to port 8080, protected by the full middleware chain. The raw port 8080 is reachable only from LAN; the public URL requires SSO authentication through Traefik itself.
6. **SSO / forward-auth**: Five independent `traefik-forward-auth` deployments each backed by separate Google OAuth2 credentials and email whitelists.
7. **Custom middleware**: A small Go service (`kubernetes/middleware`) exposes two Traefik `forwardAuth` middlewares. These are applied to the Traefik dashboard, download tools, and other sensitive routes.
8. **Sablier**: An in-cluster scale-to-zero controller patches Deployment/StatefulSet replicas. Idle workloads are scaled to 0; the first HTTP request triggers a wake with a configurable session TTL. Used by: n8n, JupyterLab, WebDAV, Guacamole, download stack, Bliss, Ollama WebUI, and more.

---

## Cluster Infrastructure

### Nodes

| Node     | Role / Notes                                                           |
|----------|------------------------------------------------------------------------|
| `nur`    | Primary worker; most stateful workloads, NFS server, Kaniko build node |
| `miniba` | GPU-capable; Ollama and JupyterLab pinned here                         |
| `nippon` | Secondary NFS server; dnsmasq split-view DNS           |

### Traefik

Deployed via k3s `HelmChartConfig`. Three replicas with node affinity (`traefik-node=true`) and pod anti-affinity to spread across nodes. `externalTrafficPolicy: Local` to preserve source IPs. Cross-namespace CRD routing enabled (`allowCrossNamespace: true`).

### cert-manager

`ClusterIssuer` `cluster-letsencrypt-dns-cloudflare` uses the Cloudflare API token (stored as a Kubernetes Secret rendered from Vaultwarden) for DNS-01 ACME. Covers wildcard and per-subdomain certificates for all five domains.

### SSO and Custom Middleware

Five `traefik-forward-auth` instances in the `sso` namespace, one per domain group, each with isolated Google OAuth2 credentials. The custom Go middleware service in the `middleware` namespace implements IP filtering and KV filtering endpoints, wrapped as Traefik `forwardAuth` middlewares.

### Sablier

`sablierapp/sablier:1.11.1` in the `sablier` namespace. ClusterRole grants `get/list/watch/patch/update` on Deployments and StatefulSets. Services declare `replicas: 0` and attach a Sablier Traefik middleware specifying session duration and UI theme.

### Calico

`v3.30.5` CNI. NetworkPolicy is applied per namespace to enforce least-privilege egress/ingress at the pod level.

**Note on IPv6 status**: The IPv6 IPPool manifest (`calico-ipv6-ippool.yaml`, pool `default-ipv6-ippool`, CIDR `2001:cafe:42::/56`) exists in the repository but carries the comment *"this config was generated but has not been used"*. The Traefik NetworkPolicy similarly notes *"IPv6 is not supported in Koishi yet"* and lists only IPv4 Cloudflare CIDR blocks. IPv6 end-to-end is planned but not yet active; Cloudflare-proxied IPv6 connections will be rejected by the NetworkPolicy as currently deployed.

### Argo Workflows

`v3.5.4` in the `argo` namespace for pipeline orchestration.

### Self-healing CronJobs

- `reschedule` (every 10 min): lists Deployments and Nodes, reschedules workloads across healthy nodes.
- `restart` (every 5 min): performs pod-restart orchestration with emptyDir state tracking.

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
- **vaultwardendatabackup** Job: full data directory zip, GPG-encrypted with a SHA256-derived passphrase fetched from an internal endpoint.

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

Redis Stack is deployed in both `ck-dev`, `ck-prod` and `database` namespaces for caching (LiteLLM response cache) and vector storage (prag RAG index).

DBGate provides a web-based DBA UI with connections to all PostgreSQL clusters, MySQL (AWS RDS), MSSQL, and SQLite. Strict egress NetworkPolicy limits dbgate outbound to kube-dns, Traefik, defined namespaces, and specific AWS RDS IPs.

### Cloud Storage and Backups

`media/rclone` runs a privileged pod (SYS_ADMIN + `privileged: true`) with FUSE mounts:

- `/mnt/webdav` — openlist + PikPak WebDAV
- `/mnt/remote` — cloud remotes (Google Drive, Aliyun)
- `/mnt/decrypted` — Rclone-encrypted backup target (transparent to consumers)

All cloud-bound data is encrypted at the rclone layer before leaving the cluster. Backup pipeline:

1. **backup-prepare** CronJob (18:00 UTC): compresses application data to an encrypted `ebackup/` directory.
2. **backup-upload** CronJob (08:00 UTC): triggers `rclone.py` inside the rclone pod to push to PikPak and Google Drive.

`media/aliyunpan` handles Aliyun cloud storage: an `encfs`-encrypted upload container and a decrypt/download container, using `encfs6.xml` for the encryption config.

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

---

## Related Projects

- [`../qosmon/`](../qosmon/) — Rust-based infrastructure health monitor that validates HTTP endpoints, TCP ports, DNS records, SSL certificates, and `noindex` headers across all Koishi ingresses. Auto-generates check configs from live Kubernetes Ingress/Service resources.
- [`../infra/`](../infra/) — Git hooks (`pre-commit`, `commit-msg`) that scan staged files and commit messages against a user-configurable blackword list to prevent accidental secret commits.
