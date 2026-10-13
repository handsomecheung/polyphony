# scrcpy

Web-based Android remote control and mirroring service powered by [ws-scrcpy](https://github.com/NetrisTV/ws-scrcpy).

## Architecture

- **Emulator Host**: Android emulator runs on host (`nur`), exposing ADB port (e.g. `<host-ip>:15555`).
- **Container / Pod**: `scrcpy` runs in Kubernetes with an entrypoint connecting to `ADB_DEVICE_ADDRESS`.
- **Ingress**: Traefik ingress exposing Web UI and WebSockets with Google SSO and LAN access.

## Build and Deploy

```bash
# Build image with in-cluster Kaniko
./build.sh

# Deploy to Kubernetes
./deploy.sh
```
