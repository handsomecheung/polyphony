# polyphony

A personal home-lab infrastructure monorepo containing everything needed to run, secure, and monitor a self-hosted Kubernetes cluster, its network edge, and the tooling that keeps it all honest. The repository is organized into four cooperating pillars: the **koishi** Kubernetes cluster, the **umbilical** cloud-edge relay, the **qosmon** health-monitoring daemon, and **infra** shared repo tooling.

## Pillars

| Pillar                             | Role                                 | Details                                                                             |
|------------------------------------|--------------------------------------|-------------------------------------------------------------------------------------|
| [koishi](./koishi/README.md)       | Flagship Kubernetes home-lab cluster | 30+ services, Traefik ingress, multi-domain SSO, Kaniko builds, Vaultwarden secrets |
| [umbilical](./umbilical/README.md) | External cloud-edge relay            | WireGuard VPN, reverse SSH tunnels, DDNS, OAuth proxy, sing-box routing             |
| [qosmon](./qosmon/README.md)       | Rust infrastructure health monitor   | HTTP/TCP/DNS/SSL checks, auto-generated configs from K8s resources                  |
| [infra](./infra/README.md)         | Shared repo tooling                  | Git hooks that block accidental secret commits                                      |

---

### [koishi](./koishi/README.md)

The largest pillar. Koishi is a multi-node Kubernetes cluster built around a "Web-First" philosophy: every service is reachable from a browser over HTTPS, authenticated through a centralized SSO layer, and deployed declaratively with no secrets in source control.

Key design decisions:

- **Traefik** ingress with **cert-manager** and Cloudflare DNS-01 ACME for automatic wildcard TLS across multiple domains.
- **Vaultwarden** (self-hosted Bitwarden) as the secret store; `__{{placeholder}}__` syntax is rendered at deploy time via the `bww` API and the `koishi/deploy` Ruby orchestrator.
- **Kaniko** for in-cluster container image builds — no privileged Docker daemon required.
- **Calico** CNI for IPv6 and fine-grained `NetworkPolicy` isolation.
- **Sablier** scale-to-zero middleware lets infrequently used services (JupyterLab, Coder, WebDAV, download stack, etc.) hibernate and wake on first HTTP request.
- Six independent **Google OAuth2** SSO domains (domainx/t/y/p/c/d) each with their own email whitelist, protecting 20+ services.
- **Nix** package manager ensures reproducible dev tooling inside the `devbox` environment.

Notable services: AI agent framework (`aiagent`), LiteLLM LLM gateway, Ollama local inference, RAG search (`prag`), Vaultwarden, media stack (Plex, Jellyfin, rclone cloud mounts), download stack (qBittorrent, Aria2, Jackett), WireGuard VPN, remote desktop (Guacamole), finance data service, dynamic DNS, CloudNativePG HA databases, and more.

---

### [umbilical](./umbilical/README.md)

The network boundary management system that bridges on-premises hardware (router, silver) with public cloud VMs.

- **WireGuard** VPN endpoint on UDP 51820 running on the cloud VM.
- **autossh** reverse tunnels expose local ports 80/443/1083 to the cloud's nginx edge, enabling external HTTPS access to home services.
- **DDNS** service polls the home IP every 2 minutes and updates Aliyun DNS records.
- **OpenResty + Vouch-proxy** provide Google OAuth + Lua-based per-user RBAC at the local network gateway.
- **sing-box** multi-proxy selector routes traffic via configurable outbounds (direct CN, proxy providers).
- Deployment uses Docker Compose on the router and silver nodes; cloud config is minimal (nginx + WireGuard only).

---

### [qosmon](./qosmon/README.md)

A Rust async monitoring tool (tokio) that validates the full infrastructure in a single run.

- Six check types: `http`, `tcp`, `dns`, `ssl` (including certificate-expiry validation), `port_scan`, and `noindex`.
- A Python generator script (`scripts/generate.py`) queries live Kubernetes Ingress and Service resources and auto-produces YAML monitoring configs, reducing manual drift.
- SSO middleware detection: Ingresses with Traefik SSO annotations automatically receive an expected-307 redirect check.
- `NoIndex` header and meta-tag validation ensures private services are excluded from search engines.
- Global semaphore limits concurrent connections (default 1000); `run.sh` sets the default concurrency to 50.

```bash
./run.sh                      # run with default concurrency
python3.12 scripts/generate.py --config configs/generate/config.yaml
```

---

### [infra](./infra/README.md)

Shared Git security hooks that prevent accidental commits of secrets or credentials.

- `git/hooks/pre-commit` — scans staged filenames and file content against a keyword blacklist.
- `git/hooks/commit-msg` — scans commit messages for the same blacklist.
- `git/blackwords.sample` — example blacklist; each developer creates their own `git/blackwords` with project-specific sensitive terms.
- Case-insensitive, word-boundary matching via `grep -iw`.

---

## Setup

### Git Hooks

After cloning the repository, configure Git to use the project's custom hooks:

```bash
git config core.hooksPath infra/git/hooks
```

Then create your personal blackwords file from the sample:

```bash
cp infra/git/blackwords.sample infra/git/blackwords
# edit infra/git/blackwords and add project-specific sensitive terms
```

This enables the pre-commit hook that prevents sensitive keywords (defined in `infra/git/blackwords`) from being committed to any file or message in this repository.
