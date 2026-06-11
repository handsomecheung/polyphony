# Download Stack: Torrent and HTTP Download Management

Integrated Kubernetes-based download management suite providing torrent and HTTP file downloading capabilities. The stack includes qBittorrent for torrent management, Aria2 for HTTP/FTP downloads, Jackett for torrent search aggregation, and FlareSolverr for anti-bot bypass. Web UIs (AriaNg and VueTorrent) enable convenient management and monitoring. All services use Sablier for dynamic on-demand scheduling in the "download" group, scaling to zero replicas by default and waking on user access.

## Architecture

The download stack operates as an isolated cluster tier with Traefik ingress, multi-layered authentication middleware, persistent NFS storage, and stateless gateway services:

- **Data flow**: External HTTP/FTP sources and torrent networks → Aria2 / qBittorrent → NFS PVC (nfs-data-download) → Consumer services
- **Access control**: Traefik ingress with playonjan, antimitm, SSO (some services), and Sablier middleware
- **Storage**: Shared NFS PVC (120Gi capacity, 30Gi claimed) mounted from nippon node; encrypted local hostPath on nur for service configuration
- **Node affinity**: Stateful services (aria2, qbittorrent, jackett) pinned to nur node; stateless services (flaresolverr, ariang, vuetorrent) free-floating
- **Orchestration**: Top-level `deploy.sh` applies middleware, then delegates to per-service deploy scripts using `my-k8s-deploy` and custom image builders

## Components

### Aria2 (`aria2/`)

Lightweight multi-protocol download manager supporting HTTP, FTP, BitTorrent, and Metalink protocols.

**Image**: Alpine-based custom image with Aria2 binary and tracker list auto-fetcher.

**Key features**:
- **Tracker list auto-fetch**: On container startup, fetches latest tracker list from `https://raw.githubusercontent.com/ngosang/trackerslist/master/trackers_all.txt` and injects into `aria2.conf` (line 10, Dockerfile)
- **RPC secret**: Templated from environment (`__{{aria2}}__`)
- **Configuration**: Stored in `/aria2/data/aria2.session` (encrypted local hostPath on nur) with session save interval of 60 seconds
- **Download directory**: `/aria2/downloads` (NFS PVC, subpath aria2/data)
- **Concurrency**: Max 3 concurrent downloads, 1 connection per server
- **Disk cache**: 32MB
- **Resume**: Enabled
- **Log level**: Warn

**Build**:
```bash
./aria2/build.sh
```
Invokes `my-k8s-build-image cloudpublic/default/aria2:latest default aria2`.

**Deploy**: Applied via `aria2/deploy.sh` → `my-k8s-deploy`.

**Kubernetes manifest** (`aria2/app.yaml`):
- Deployment: 0 replicas (Sablier-controlled), node-pinned to nur
- Ports: 6800/TCP (RPC protocol)
- Ingress: `aria2.<domain>` with Sablier middleware, no SSO (internal-only)
- Resources: 100m CPU / 128Mi RAM request
- Volumes: 
  - `default-common-encrypted` (hostPath /mnt/runtime-data-app/commondata/aria2/config)
  - `nfs-data-download` PVC (subpath aria2/data)

### AriaNg (`ariang/`)

Vue.js-based web UI for Aria2 (AriaNg v1.3.7), written in HTML/JavaScript and served by Nginx.

**Image**: Multi-stage build using Alpine 3.19 + Nginx, downloads AriaNg release from GitHub and serves statically.

**Features**:
- **Lightweight**: 50m CPU / 64Mi RAM request
- **Stateless**: No persistent storage
- **RPC backend**: Configured to connect to aria2 service for download management
- **Nginx config**: Serves AriaNg from `/AriaNg/`, nginx worker_processes=auto

**Build**:
```bash
./ariang/build.sh
```
Invokes `my-k8s-build-image cloudpublic/default/ariang:latest default ariang`.

**Deploy**: Applied via `ariang/deploy.sh` → `my-k8s-deploy`.

**Kubernetes manifest** (`ariang/app.yaml`):
- Deployment: 0 replicas (Sablier-controlled), no node affinity
- Port: 80/TCP
- Ingress: `ariang.<domain>` with playonjan, antimitm, SSO, and Sablier middleware
- Resources: 50m CPU / 64Mi RAM request
- Homepage annotations: Media group, weight 40

### qBittorrent (`qbittorrent/`)

Full-featured BitTorrent client with integrated web UI (linuxserver/qbittorrent image).

**Image**: `linuxserver/qbittorrent` (always pulled latest).

**Key features**:
- **WebUI port**: 8080
- **FlareSolverr integration**: FLARE_URL environment variable points to `http://flaresolverr.default/` for tracker discovery with anti-bot bypass
- **File permissions**: PUID=1000, PGID=1000 for consistent NFS ownership
- **Configuration**: Stored in encrypted local hostPath on nur (`commondata/qbittorrent/config`)
- **Downloads**: NFS PVC, subpath qbittorrent/data
- **Ports**: 6881/TCP (peer protocol), 8080/TCP (WebUI)

**Deploy**: Applied via `qbittorrent/deploy.sh` → `my-k8s-deploy`.

**Kubernetes manifest** (`qbittorrent/app.yaml`):
- Deployment: 0 replicas (Sablier-controlled), node-pinned to nur
- Ingress: `qbittorrent.<domain>` with playonjan, antimitm, SSO, and Sablier middleware
- Resources: 200m CPU / 256Mi RAM request, 400m CPU / 512Mi RAM limit
- Homepage annotations: Media group, weight 50
- Volumes:
  - `default-common-encrypted` (hostPath /mnt/runtime-data-app/commondata/qbittorrent/config)
  - `nfs-data-download` PVC (subpath qbittorrent/data)

### VueTorrent (`qbittorrent-vuetorrent/`)

Lightweight Vue.js-based web UI for qBittorrent (alternative to default WebUI), served by Nginx.

**Image**: Multi-stage build using Nginx Alpine, downloads VueTorrent v0.12.0 from GitHub.

**Features**:
- **Lightweight**: 50m CPU / 32Mi RAM request, 100m CPU / 64Mi RAM limit
- **Stateless**: No persistent storage
- **API proxy**: Nginx proxies `/api` requests to qbittorrent service for download control
- **HTTP/2 push**: Enabled (`http2_push_preload on`)
- **Max upload**: 10M

**Build**:
```bash
./qbittorrent-vuetorrent/build.sh
```
Invokes `my-k8s-build-image cloudpublic/default/qbittorrent-vuetorrent:latest default qbittorrent-vuetorrent`.

**Deploy**: Applied via `qbittorrent-vuetorrent/deploy.sh` → `my-k8s-deploy`.

**Kubernetes manifest** (`qbittorrent-vuetorrent/app.yaml`):
- Deployment: 0 replicas (Sablier-controlled), no node affinity
- Port: 80/TCP
- Ingress: `qbittorrent-vuetorrent.<domain>` with playonjan, antimitm, SSO, and Sablier middleware; HTTP/2 push preload enabled
- Resources: 50m CPU / 32Mi RAM request, 100m CPU / 64Mi RAM limit
- Homepage annotations: Media group, weight 60

### Jackett (`jackett/`)

Torrent indexer aggregator translating torrent site-specific search queries into a unified API (linuxserver/jackett image).

**Image**: `linuxserver/jackett` (always pulled latest).

**Key features**:
- **FlareSolverr integration**: FLARE_URL points to `http://flaresolverr.default/` for cloudflare/anti-bot bypass on tracker searches
- **File permissions**: PUID=1000, PGID=1000
- **Configuration**: Stored in encrypted local hostPath on nur (`commondata/jackett/config`)
- **Port**: 9117/TCP
- **Search API**: Responds to `/api/v2.0/indexers/...` queries from torrent clients

**Deploy**: Applied via `jackett/deploy.sh` → `my-k8s-deploy`.

**Kubernetes manifest** (`jackett/app.yaml`):
- Deployment: 0 replicas (Sablier-controlled), node-pinned to nur
- Service: Maps port 80 → 9117 (WebUI)
- Ingress: `jackett.<domain>` with playonjan, antimitm (no SSO), and Sablier middleware
- Resources: 100m CPU / 256Mi RAM request
- Homepage annotations: Media group, weight 70
- Volumes:
  - `default-common-encrypted` (hostPath /mnt/runtime-data-app/commondata/jackett/config)
  - `default-common-encrypted` (hostPath /mnt/runtime-data-app/commondata/jackett/downloads)

### FlareSolverr (`flaresolverr/`)

Stateless proxy service that solves Cloudflare CAPTCHA and anti-bot challenges for torrent indexers and other applications.

**Image**: `ghcr.io/flaresolverr/flaresolverr:latest` (always pulled latest).

**Features**:
- **Protocol**: HTTP API on port 8191
- **Stateless**: No persistent storage; ephemeral container
- **Consumed by**: qBittorrent and Jackett via FLARE_URL environment variable
- **Resources**: 100m CPU / 512Mi RAM request

**Deploy**: Applied via `flaresolverr/deploy.sh` → `my-k8s-deploy`.

**Kubernetes manifest** (`flaresolverr/app.yaml`):
- Deployment: 0 replicas (Sablier-controlled), no node affinity
- Port: 8191/TCP
- Ingress: `flaresolverr.<domain>` with playonjan, antimitm, SSO, and Sablier middleware
- Resources: 100m CPU / 512Mi RAM request
- Homepage annotations: Media group, weight 80

## Networking & Access Control

### Traefik Ingress

All services expose HTTPS via Traefik with cert-manager-issued Let's Encrypt / Cloudflare DNS validation certificates.

**Ingress annotations**:
- `cert-manager.io/cluster-issuer: cluster-letsencrypt-dns-cloudflare`
- `traefik.ingress.kubernetes.io/router.middlewares: ...` (varies per service)

### Middleware Stack (`k8s.middleware.yaml`)

Sablier middleware controls on-demand scheduling for the entire "download" group:

```yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: sablierdownload
spec:
  plugin:
    sablier:
      sablierUrl: "http://sablier.sablier"
      group: "download"
      sessionDuration: "1h"
      dynamic:
        theme: "ghost"
        showDetails: true
```

**Per-service middleware**:
- **aria2** (internal-only): `sablierdownload` only
- **ariang, qbittorrent, qbittorrent-vuetorrent, flaresolverr** (user-facing): `playonjan`, `antimitm`, `sso-domainx` (SSO), `sablierdownload`
- **jackett** (backend): `playonjan`, `antimitm` (no SSO), `sablierdownload`

**Session duration**: 1 hour (Sablier keeps services running 1 hour after last access).

### Security

- **SSO**: Applied to user-facing web UIs (ariang, qbittorrent, vuetorrent, flaresolverr) via `sso-domainx@kubernetescrd` middleware
- **Anti-MITM**: Applied to all ingresses via `middleware-antimitm` middleware
- **IP allowlist**: Applied to user-facing services via `middleware-playonjan` (homepage integration)
- **Encrypted config storage**: Local hostPath on nur node (/mnt/runtime-data-app/commondata) is encrypted

## Storage & Persistence

### Shared Download Storage

**PVC**: `nfs-data-download` (120Gi capacity, 30Gi requested)
- **Source**: NFS mounted from nippon node at `/mnt/nfs/exports/data/nfs-data-download`
- **Subpaths**:
  - `aria2/data` → `/aria2/downloads`
  - `qbittorrent/data` → `/downloads`
- **Permissions**: Services run with PUID=1000/PGID=1000 for consistent ownership

### Configuration Storage

**HostPath**: `/mnt/runtime-data-app/commondata` on nur node (encrypted)
- `aria2/config` → `/aria2/data`
- `qbittorrent/config` → `/config`
- `jackett/config` → `/config`
- `jackett/downloads` → `/downloads`

## Build & Deployment

### Build

Custom images (aria2, ariang, qbittorrent-vuetorrent) are built via per-directory `build.sh` scripts:

```bash
./aria2/build.sh
./ariang/build.sh
./qbittorrent-vuetorrent/build.sh
```

Each invokes `my-k8s-build-image` with the target image name, namespace, and directory. Pre-built images (linuxserver/qbittorrent, linuxserver/jackett, flaresolverr) are pulled directly at deployment.

### Deploy

```bash
./deploy.sh
```

Orchestrates deployment in this order:
1. Applies middleware configuration (`k8s.middleware.yaml` via `my-k8s-deploy`)
2. Deploys aria2 (`./aria2/deploy.sh`)
3. Deploys ariang (`./ariang/deploy.sh`)
4. Deploys flaresolverr (`./flaresolverr/deploy.sh`)
5. Deploys jackett (`./jackett/deploy.sh`)
6. Deploys qbittorrent (`./qbittorrent/deploy.sh`)
7. Deploys qbittorrent-vuetorrent (`./qbittorrent-vuetorrent/deploy.sh`)

Each service deploy applies its `app.yaml` via `my-k8s-deploy --file=app.yaml`.

## Related Documentation

- **Parent**: See [`../README.md`](../README.md) for koishi infrastructure overview
- **Sablier**: On-demand workload scheduling; configured in middleware with 1-hour session duration
- **Traefik**: Ingress controller with middleware chain for authentication and routing
- **Kubernetes**: Manifests use cert-manager, networking.k8s.io/v1 Ingress, and core k8s primitives
