# Docker Compose Infrastructure Services

Lightweight management services running on Kubernetes nodes via docker-compose: watchtower for automatic container updates, comapi for node metrics and discovery, shutdown for coordinated node power-off with drain, wakeup for wake-on-LAN and node uncordoning, and dnsmasq for split-view internal DNS resolution.

## Overview

Each Kubernetes node (nur, miniba, nippon) runs a set of docker-compose services in host network mode, providing operational tooling and observability for the cluster. Services are deployed with secret templating via `bwww render-file`, rsync'd to nodes, and managed through per-node lifecycle scripts (restart.sh, stop.sh, rebuild.sh).

## Architecture

### Service Deployment Model

- **Secret templating**: Source templates in `nippon/local/`, `nur/local/`, `miniba/local/` contain placeholder variables (e.g., `__{{koishi.deploy.timezone}}__`) rendered by `sync.sh` using the `bwww` tool.
- **Synchronization**: `sync.sh` copies rendered configs to live deployment directories on each node via rsync.
- **Current state**: The `current/local/` directory contains example rendered configs with concrete values (registry IPs, timezone, subnet prefix).
- **Service lifecycle**: Each node's `local/` directory includes `docker-compose.yml` plus `restart.sh`, `stop.sh`, and `rebuild.sh` scripts that use `sudo docker compose` to control services.

### Network Configuration

All management services run in **host network mode** (`network_mode: host`), providing:
- Direct port exposure without bridge isolation
- Access to host resources (`docker.sock`, systemd, dbus, `/sys/fs/cgroup`)
- Host interface enumeration for multi-homed setups

## Components

### comapi: Node Metrics API

**Language**: Rust | **Port**: 37900 | **Deployment**: All nodes

HTTP service exposing node metrics via Warp web framework.

**Endpoints**:
- `/ok` — Health check, returns `"ok"`
- `/ipv6` — Queries host interface matching `KOISHI_IPV4_PREFIX` and returns its IPv6 address
- `/top` — Returns comma-separated metrics: `1m_cpu_load,memory_usage_GB,cpu_temp_C`

**Key behavior**:
- Uses `pnet` crate to enumerate network interfaces
- Runs `uptime` and `free -m` commands for CPU load and memory usage
- Reads `/sys/class/thermal/thermal_zone0/temp` for CPU temperature (in millidegree Celsius); falls back to any available thermal zone
- Requires `KOISHI_IPV4_PREFIX` environment variable 

**Build**:
```bash
cd images/comapi
./build.sh  # Uses my-k8s-build-image; creates MUSL-static binary for alpine:3.19
```

### shutdown: Node Power-Off Control

**Tool**: gotty (web terminal) | **Port**: 37901 | **Deployment**: nur, miniba

Web terminal service (gotty v2.0.0-alpha.3) providing interactive command line for graceful node shutdown.

**Entrypoint script** (`entrypoint.gotty.sh`):
- Accepts optional argument `true` for auto-shutdown (30-minute delay, skips interactive prompt, only executes if current hour is 00:00–06:00)
- Verifies node is registered in Kubernetes cluster via `kubectl get nodes`
- Launches tmux session to run: `kubectl drain --delete-emptydir-data --ignore-daemonsets <node> ; systemctl poweroff`
- Attaches to tmux session for live monitoring

**Privileges**: Runs in privileged mode with mounts to systemd and dbus for power control.

**Build**:
- Base: ubuntu:24.04
- Installs: tmux, gotty, kubectl
- Copies: entrypoint.sh, entrypoint.gotty.sh

### wakeup: Node Wake-On-LAN and Uncordon

**Tool**: gotty (web terminal) | **Port**: 37902 | **Deployment**: nippon only

Web terminal service providing remote node wake-up and Kubernetes uncordon operations.

**Entrypoint script** (`entrypoint.gotty.sh`):
- Accepts node name argument (e.g., `nur`, `miniba`, `nippon`)
- Validates node exists in Kubernetes cluster via `kubectl get nodes`
- Uses `etherwake` to send wake-on-LAN magic packet to node's MAC address (sourced from environment: `MAC_NUR`, `MAC_MINIBA`, `MAC_NIPPON`)
- Sends packet via interface matching `KOISHI_IPV4_PREFIX`
- Uncordons node in Kubernetes via `kubectl uncordon <node>`

**Privileges**: Runs in privileged mode for raw socket access (etherwake requirement).

**Build**:
- Base: ubuntu:22.04
- Installs: etherwake, curl, net-tools, netcat, gotty, kubectl
- Copies: entrypoint.sh, entrypoint.gotty.sh

### dnsmasq: Split-View Internal DNS

**Port**: 53 (UDP/TCP) | **Deployment**: nippon only

DNS server providing split-view domain resolution for machines and Kubernetes services within the home lab.

**Configuration** (`dnsmasq.conf`):
- Upstream resolvers: 1.1.1.1, 8.8.8.8
- Host records: maps short names (nur, miniba, nippon, gopya) to subnet-local IPs 
- Domain mappings: routes `home.*`, `home4p.*`, `openlist.*`, `plex.*`, `webdav.*`, `vaultwarden.*` domains to specific node IPs based on configured domain suffixes
- Disables IPv6 for split-view domains (replies with `::`)

**Template placeholders**:
- `__{{koishi.infra.infos:f:subnetv4}}__` — Subnet prefix 
- `__{{infra.domains:f:x}}__`, `__{{infra.domains:f:t}}__`, etc. — Domain suffixes for different split-view zones

**Build**:
- Base: dockurr/dnsmasq
- Adds custom dnsmasq.conf

### watchtower: Container Auto-Updates

**Image**: containrrr/watchtower:latest | **Port**: none (daemon) | **Deployment**: All nodes

Automatic container image update service with configurable interval.

**Configuration**:
- Update interval: 120 seconds
- Mounts: `/var/run/docker.sock` (Docker daemon), `/home/box/.docker/config.json` (registry credentials)
- Restart policy: `unless-stopped` (nippon) or `always` (nur, miniba)

## Deployment and Management

### Synchronization Workflow

```bash
./sync.sh
```

1. Copies entire directory to temporary staging directory
2. Runs `bwww render-file` on staging directory to substitute secret placeholders
3. For each node (nur, nippon, miniba): rsyncs rendered `{node}/` subtree to `{node}:current/` on target servers
4. Cleans up staging directory

### Service Lifecycle Scripts

Each node has local deployment scripts in `current/local/`:

```bash
./restart.sh <service> [--build]   # Rebuild (if --build) and restart service
./stop.sh <service>                # Stop and remove service container
./rebuild.sh <service>             # Rebuild and restart
```

Example:
```bash
ssh nur "cd current/local && ./restart.sh comapi"
```

## Configuration and Secrets

## Network Topology

DNS names (dnsmasq):
- Short names: `nur`, `miniba`, `nippon`, `gopya` → subnet IPs 
- Split-view domains: `home.*`, `home4p.*`, `openlist.*`, `plex.*`, `webdav.*`, `vaultwarden.*` → node-specific IPs

## Cross-Links

- Parent infrastructure: [../README.md](../README.md)
- Kubernetes wrappers that surface these node endpoints in-cluster: [../external-services/](../external-services/)
- Secret configuration system: Uses `bwww` tool from broader polyphony infrastructure
