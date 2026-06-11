# Project Context

## Koishi Development Rules

This file provides context and rules for working within the `koishi` directory, which contains the infrastructure and services management for the home network cluster.

### Infrastructure & Development

*   **Cluster Management:** The cluster is managed via Kubernetes.
*   **Deployment:** Services are typically deployed using a `deploy.sh` script located in their respective subdirectories or the `koishi` directory itself.
*   **Secret Management:** Sensitive data should never be committed to the repository. Secrets are managed via Vaultwarden and rendered dynamically.
*   **Build System:** Container images are built in-cluster using Kaniko.


---

## Service Catalog

### AI

| Service    | Subdirectory | Notes                                                                                                                                                                                          |
|------------|--------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| LiteLLM    | `litellm/`   | LLM gateway; routes to Ollama (local), Gemini, optional Anthropic; Redis response cache; rate limiting                                                                                         |
| Ollama     | `ollama/`    | Local inference on `miniba` (GPU node); OLLAMA_NUM_PARALLEL=4; 12 CPU / 8 Gi limit                                                                                                             |
| Open WebUI | `ollama/`    | Browser chat UI for Ollama; Sablier scale-to-zero                                                                                                                                              |
| prag       | `prag/`      | RAG service: indexes markdown notes, embeds via Ollama bge-m3, stores in Redis HNSW, serves `/query` via FastAPI                                                                               |
| aiagent    | `aiagent/`   | Generic AI agent framework using `pydantic-ai-skills`; dynamically loads skills (Python/Node); WebUI + REST + WebSocket; routes through LiteLLM — see [`aiagent/README.md`](aiagent/README.md) |

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
