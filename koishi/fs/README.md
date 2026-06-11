# FS Module: File Upload/Download and Code Editing Suite

The FS module provides a comprehensive file management system for the Koishi cluster, combining static file distribution, web-based uploads, and in-browser code editing. It consists of three complementary services: **fs-downserver** (static file downloads via nginx), **fs-upserver** (Flask-based file uploads), and **fs-codeserver** (VS Code in the browser). All services are deployed on Kubernetes with Traefik ingress and optional session management via Sablier.

## Architecture Overview

The module is split across three independent deployments:

| Service            | Port     | Purpose                                     | Storage                                                                 | Access Control                                           |
|--------------------|----------|---------------------------------------------|-------------------------------------------------------------------------|----------------------------------------------------------|
| fs-downserver      | 80, 8404 | Static file downloads via nginx             | `/mnt/runtime-data-app/commondata/downserver` + `/mnt/coder-sharepoint` | Public (no auth)                                         |
| fs-upserver        | 80       | File upload form (Flask)                    | `/mnt/coder-sharepoint/upserver`                                        | BasicAuth protected; managed by Sablier (wake-on-demand) |
| fs-upserver-downcc | 80       | Enhanced upserver with download URL display | `/mnt/runtime-data-app/commondata/downserver/cc`                        | SSO protected; always on                                 |
| fs-codeserver      | 8000     | VS Code in browser                          | `/mnt/coder-sharepoint/upserver`                                        | BasicAuth protected; always on                           |

All deployments are pinned to node `nur` and mounted to shared volumes for file persistence.

## Services

### fs-downserver

**Location:** `downserver/`

Serves static files via **nginx** on two separate ports:

- **Port 80**: Serves files from `/downserver` and `/coder-sharepoint`; ingress routes `/` paths here
- **Port 8404**: Returns 404 for all requests; ingress routes `/private` paths here (explicit denial for private content)

**Dockerfile and Config:**
- `downserver/Dockerfile` - lightweight nginx container
- `downserver/nginx.conf` - two-server configuration with try_files fallback

**Build & Deploy:**
```bash
cd downserver && bash build.sh   # Builds cloudpublic/default/fs-downserver:latest
cd fs && bash deploy.sh          # Deploys via my-k8s-deploy
```

**Ingress:**
- Host: `downserver.<domain>` (TLS via cert-manager)
- Paths: `/` (port 80) and `/private` (port 8404)
- Resources: 50m CPU, 50Mi memory (request); 100m CPU, 80Mi memory (limit)

### fs-upserver

**Location:** `upserver/`

**Purpose:** Simple HTTP file upload endpoint with optional URL display for downloads. Runs as a Flask application and is managed by Sablier for wake-on-demand (default: 30-minute session duration).

**Flask Application:**
- `upserver/code/main.py` (Python 3.12) - file upload handler with environment-driven configuration
  - Route `GET /` - renders `upload.html` form
  - Route `POST /` - accepts multipart `file` uploads, saves to `/files` mount
  - Route `GET /ok` - health check endpoint
  - Environment variables:
    - `SHOW_URL` - if `"true"`, displays clickable download links after upload
    - `ROOT_URL` - base URL for generated download links (e.g., `https://downserver.example.com/cc`)
- `upserver/code/requirements.txt` - Flask, requests
- `upserver/code/templates/upload.html` - multifile upload form with submit button
- `upserver/code/static/default.css` - stylesheet

**Dockerfile:**
```dockerfile
FROM python:slim
RUN pip install -r /tmp/requirements.txt
WORKDIR /code
ENTRYPOINT ["python3", "-u", "main.py"]
```

**Build & Deploy:**
```bash
cd upserver && bash build.sh     # Builds cloudpublic/default/fs-upserver:latest and restarts fs-upserver-downcc
cd fs && bash deploy.sh
```

**Kubernetes Deployment:**
- Replicas: 0 (wake-on-demand via Sablier)
- Storage: `/mnt/coder-sharepoint/upserver` mounted as `/files`
- Ingress Host: `p.<domain>` (TLS via cert-manager)
- Middleware Stack:
  - `fsbasicauth` - HTTP Basic Auth (protected credential in Secret `fsbasicauth`)
  - `sablierfsupserver` - wake-on-demand with 30-minute session; group `fsupserver`
  - `fsupserversizelimit` - 2GB max request body for large uploads
  - `middleware-antimitm` - anti-MITM security
- Resources: 50m CPU, 64Mi memory
- Health checks: liveness and readiness probes on `/ok` endpoint

### fs-upserver-downcc

**Location:** `k8s.app.upserver-downcc.yaml`

**Purpose:** Secondary fs-upserver deployment configured to display shareable download links after file upload. Runs permanently (replicas: 1) and is gated by SSO auth. Files are stored in a separate path (`/mnt/runtime-data-app/commondata/downserver/cc`) allowing parallel operation with the primary upserver.

**Key Differences from fs-upserver:**
- Environment Variables: `SHOW_URL=true`, `ROOT_URL=https://downserver.<domain>/cc`
- Ingress Host: `upserver-downcc.<domain>` (TLS)
- Middleware Stack:
  - `sso-domainx` - SSO authentication
  - `middleware-antimitm` - anti-MITM security
  - `fsupserversizelimit` - 2GB max request body
  - **No BasicAuth** (uses SSO instead)
- Replicas: 1 (always on, unlike primary upserver)
- Storage: `/mnt/runtime-data-app/commondata/downserver/cc` mounted as `/files`
- HomePage Badge: labeled as "Upload to Daily Paw"

**Deploy:**
- Restarted automatically when fs-upserver image is rebuilt (see `upserver/build.sh`)
- Manually deployable via `my-k8s-deploy --file=k8s.app.upserver-downcc.yaml`

### fs-codeserver

**Location:** `codeserver/`

**Purpose:** VS Code running in the browser via **code-server** (version 4.96.4), providing a full-featured IDE for editing files in `/mnt/coder-sharepoint/upserver`.

**Dockerfile:**
- Base: `cloudpublic/base/debian:bookworm`
- Installs code-server v4.96.4 via official install script
- Extensions:
  - `vscodevim.vim` - Vim keybindings
  - `vscode-icons-team.vscode-icons` - file icons
  - `oderwat.indent-rainbow` - visual indent guides
  - `mechatroner.rainbow-csv` - CSV syntax highlighting
  - `redhat.vscode-yaml` - YAML support
- Entrypoint: `entrypoint.sh` - launches code-server with `--auth none` (relies on Traefik BasicAuth)

**Build & Deploy:**
```bash
cd codeserver && bash build.sh   # Builds cloudpublic/default/fs-codeserver:latest
cd fs && bash deploy.sh
```

**Kubernetes Deployment:**
- Replicas: 1 (always on)
- Storage: `/mnt/coder-sharepoint/upserver` mounted as `/workspace`
- Container Port: 8000
- Service Port: 80 (Kubernetes service maps 8000 internally)
- Ingress Host: `fs-codeserver.<domain>` (TLS via cert-manager)
- Middleware: `fsbasicauth` (HTTP Basic Auth)
- Resources: 50m CPU, 64Mi memory
- siteMonitor: enabled for homepage monitoring

## Deployment

### Build Process

Each service has a `build.sh` script that uses `my-k8s-build-image` helper to:
1. Build the Docker image
2. Tag it as `cloudpublic/default/<service>:latest`
3. Push to the registry

Example:
```bash
cd koishi/fs/downserver && bash build.sh
cd koishi/fs/upserver && bash build.sh
cd koishi/fs/codeserver && bash build.sh
```

**Special:** `upserver/build.sh` also triggers a rollout restart of `fs-upserver-downcc` to ensure it picks up the new fs-upserver image.

### Deployment Process

```bash
cd koishi/fs && bash deploy.sh
```

This script runs:
- `my-k8s-deploy --file=app.yaml` - deploys main services (downserver, upserver, codeserver)
- `my-k8s-deploy --file=k8s.app.upserver-downcc.yaml` - deploys secondary upserver-downcc

Both manifests are rendered with templated values (e.g., `__{{infra.domains:f:x}}__` for domain substitution).

## Storage and Volumes

All services mount to persistent host directories on node `nur`:

| Mount                      | Host Path                                        | Service(s)         | Purpose                                            |
|----------------------------|--------------------------------------------------|--------------------|----------------------------------------------------|
| `/downserver`              | `/mnt/runtime-data-app/commondata/downserver`    | fs-downserver      | Public static files                                |
| `/mnt/coder-sharepoint`    | `/mnt/coder-sharepoint/`                         | fs-downserver      | Shared file exchange                               |
| `/files` (upserver)        | `/mnt/coder-sharepoint/upserver`                 | fs-upserver        | Upload destination                                 |
| `/files` (upserver-downcc) | `/mnt/runtime-data-app/commondata/downserver/cc` | fs-upserver-downcc | CC upload destination (exposed via downserver /cc) |
| `/workspace`               | `/mnt/coder-sharepoint/upserver`                 | fs-codeserver      | Code editor workspace                              |

## Security & Access Control

- **fs-downserver:** Public, no authentication
- **fs-upserver:** Protected by HTTP Basic Auth (credential in `fsbasicauth` Secret); wake-on-demand via Sablier
- **fs-upserver-downcc:** Protected by SSO (`sso-domainx` middleware); always on
- **fs-codeserver:** Protected by HTTP Basic Auth; editor runs with `--auth none` (security delegated to ingress)
- **TLS:** All ingress rules use cert-manager with ClusterIssuer `cluster-letsencrypt-dns-cloudflare`
- **Size Limits:** fs-upserver and fs-upserver-downcc allow up to 2GB request body via `fsupserversizelimit` middleware
- **Anti-MITM:** fs-upserver-downcc includes `middleware-antimitm` middleware

## Related Documentation

- Parent module: [Koishi Cluster README](../README.md)
- Sablier (wake-on-demand): managed via Traefik middleware plugin
- cert-manager: handles automatic TLS renewal
- Traefik Ingress Controller: routes and applies middleware
