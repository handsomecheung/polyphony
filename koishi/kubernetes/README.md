# Kubernetes — Core Platform Layer

The `kubernetes/` directory is the foundational control-plane layer for the Koishi home-lab cluster. It wires together the CNI network plugin (Calico), the TLS certificate authority (cert-manager + Cloudflare DNS-01), the ingress controller (Traefik), centralized Google OAuth2 SSO across six independent domains, custom IP/KV Traefik middlewares written in Go, the Sablier on-demand scale-to-zero controller, NFS and local-path persistent storage, a workflow engine (Argo), GCP registry lifecycle rules, and two self-healing CronJob operators (reschedule, restart). Every subdirectory is a discrete component; together they form the ingress → networking → TLS → auth → storage stack that all other Koishi services depend on.

---

## Contents

| Directory       | Role                                                                   |
|-----------------|------------------------------------------------------------------------|
| `calico/`       | Calico v3.30.5 CNI with IPv6 IPPool config                             |
| `cert-manager/` | Let's Encrypt DNS-01 ClusterIssuer via Cloudflare                      |
| `middleware/`   | Custom Go service exposing IP-filter and KV-filter Traefik middlewares |
| `reschedule/`   | CronJob that reschedules pods onto preferred nodes when they come back |
| `restart/`      | CronJob that restarts stuck deployments with exponential backoff       |
| `sablier/`      | Scale-to-zero controller (sablierapp/sablier 1.11.1)                   |
| `sso/`          | Six independent Google OAuth2 forward-auth instances per DNS domain    |
| `traefik/`      | Traefik HelmChartConfig, dashboard Ingress, and NetworkPolicy          |
| `argo/`         | Argo Workflows v3.5.4 pipeline engine                                  |


---

## calico

Calico v3.30.5 is the CNI plugin providing pod networking and NetworkPolicy enforcement. `calico.yaml` is the upstream manifest, downloaded and then patched for the cluster's IP address configuration. `calico-ipv6-ippool.yaml` defines a named `default-ipv6-ippool` (CIDR `2001:cafe:42::/56`, `natOutgoing: true`, `blockSize: 122`) that enables dual-stack pod addressing — currently prepared but noted as a TODO in production. To activate NAT outgoing on an existing pool: `kubectl patch ippool default-ipv6-ippool --type=merge -p '{"spec":{"natOutgoing":true}}'`.

There is no `deploy.sh` in this directory; `calico.yaml` is applied directly with `kubectl apply`.

---

## cert-manager

cert-manager provides cluster-wide TLS automation. A single `ClusterIssuer` named `cluster-letsencrypt-dns-cloudflare` is configured for DNS-01 challenges via the Cloudflare API, allowing wildcard and subdomain certificates to be issued without exposing HTTP ports.

`deploy.sh` applies cert-manager itself from the upstream release URL, then uses `my-k8s-deploy` to apply the Cloudflare API token Secret and the ClusterIssuer:

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.15.0/cert-manager.yaml
my-k8s-deploy --file=token.cloudflare-api-token.yaml
my-k8s-deploy --file=cluster-issuer-dns-cloudflare.yaml
```

The issuer's ACME email and the Cloudflare token are injected at deploy time via the Vaultwarden template engine. Every Ingress or Certificate resource across the cluster references this issuer by the name `cluster-letsencrypt-dns-cloudflare`.

---

## middleware

A custom Go HTTP service (Gin framework, Go 1.24) that provides two Traefik `forwardAuth` middlewares used cluster-wide.

| Middleware CRD         | Endpoint         | Behaviour                                                                                                                                                      |
|------------------------|------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `middleware-antimitm`  | `GET /ipfilter1` | Reads client IP from `CF-Connecting-IP`; returns 403 if the IP matches any CIDR in `ip_filter1_file.txt` (Netskope + Zscaler egress ranges blocked by default) |
| `middleware-playonjan` | `GET /kvfilter1` | Checks a configurable request header key/value pair; returns 404 if the value does not match (acts as a shared secret header gate)                             |

The service is deployed as a single-replica Deployment in the `middleware` namespace on image `cloudprivate/middleware/common:latest`. Both CIDR lists and the KV secret are injected via environment variables at deploy time (`IP_FILTER1_TEXT`, `KV_FILTER1_KEY`, `KV_FILTER1_VALUE`); a static block-list file (`ip_filter1_file.txt`) is baked into the image at build time.

---

## reschedule

A CronJob that runs every 10 minutes in the `default` namespace. It detects deployments with `preferredDuringSchedulingIgnoredDuringExecution` node affinity rules whose pods are running on non-preferred nodes, and — only when the preferred node is in `Ready` state — issues a `kubectl rollout restart` to migrate them back. This handles the case where a node reboots and pods land on fallback nodes but are never moved back after the preferred node recovers.

RBAC grants read on `nodes`/`pods` and get/list/patch on `deployments` cluster-wide (no wildcard permissions).


---

## restart

A CronJob that runs every 5 minutes in the `default` namespace and monitors a hardcoded set of deployments for non-running pod states. It implements exponential backoff (1 → 2 → … → 60 minutes) so it does not storm-restart a pod that is legitimately stuck. State is persisted to an `emptyDir` volume (`/tmp/restart/operator_state.json`, 50 Mi limit) scoped to the CronJob Pod.

Logic:
- Skip if replicas == 0 (Sablier-managed).
- Skip if pod is `Pending` (waiting for a node).
- Wait at least 5 minutes after pod creation before acting.
- Respect the backoff window from the last restart.

RBAC is namespace-scoped (`default`) and grants only get/list on pods and get/patch on deployments.


---

## sablier

Sablier provides on-demand scale-to-zero for any Deployment or StatefulSet. It runs in its own `sablier` namespace with a ClusterRole that allows it to get/list/watch/patch Deployments and StatefulSets cluster-wide.

The service is exposed as `http://sablier.sablier` on port 80 (targeting container port 10000). Individual services reference Sablier by adding a Traefik `forwardAuth` or Sablier plugin middleware pointing at this URL with a `sessionDuration` (typically 1 h). When no requests arrive during the session window, Sablier scales the backing deployment to 0; the first inbound request is held while Sablier scales it back up, then released.

---

## sso

Six independent Google OAuth2 forward-auth instances (using `thomseddon/traefik-forward-auth`) are deployed in the `sso` namespace, one per DNS domain:

| Manifest           | Domain variable     | Purpose                                                                  |
|--------------------|---------------------|--------------------------------------------------------------------------|
| `k8s.domainx.yaml` | `infra.domains:f:x` | Primary domain — protects 20+ services with per-service email whitelists |
| `k8s.domaint.yaml` | `infra.domains:f:t` | T-domain services                                                        |
| `k8s.domainy.yaml` | `infra.domains:f:y` | Y-domain services                                                        |
| `k8s.domainp.yaml` | `infra.domains:f:p` | P-domain services                                                        |
| `k8s.domainc.yaml` | `infra.domains:f:c` | C-domain services                                                        |
| `k8s.domaind.yaml` | `infra.domains:f:d` | D-domain services                                                        |

Each instance has its own Kubernetes Secret holding the Google OAuth2 client ID, client secret, and a random cookie-signing secret. 

Session cookies have a 1-year lifetime (`LIFETIME=31536000`). Each instance exposes a Traefik `Middleware` CRD (`sso-domainx@kubernetescrd`, etc.) and an Ingress at `gsso.<domain>` for the OAuth callback. Certificates are issued by `cluster-letsencrypt-dns-cloudflare`.

---

## traefik

Traefik is managed as a k3s Helm chart. Key settings in `k8s.traefik-config.yaml`:

- **Entrypoints**: `web` (80, permanent redirect to HTTPS), `websecure` (443), `traefik` (9000 internal dashboard), `tcpep` (7030/TCP, external 37030), `udpep` (7070/UDP, external 37070).
- **Cross-namespace CRDs**: `allowCrossNamespace: true` so middlewares defined in `middleware/` and `sso/` can be referenced by Ingresses in any namespace.
- **externalTrafficPolicy: Local** — preserves client IP at the node level.
- **3 replicas** with `requiredDuringSchedulingIgnoredDuringExecution` node affinity (`traefik-node=true` label) and pod anti-affinity (`topologyKey: kubernetes.io/hostname`) to spread replicas across distinct nodes.
- **Cloudflare header logging**: `CF-Connecting-IP`, `CF-Real-IP`, `CF-Request-ID`, `CF-Visitor` are retained in JSON access logs.
- A global `noindex` middleware (injects `X-Robots-Tag: noindex` response header) is applied to both `web` and `websecure` entrypoints by default.

**NetworkPolicy** (`k8s.networkpolicy.yaml`) restricts inbound to Traefik pods:

- Ports 80, 443, 7030, 7070: from all 15 Cloudflare IPv4 CIDR blocks plus RFC-1918 LAN ranges.
- Port 9000 (dashboard): from LAN only (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`).

**Dashboard Ingress** (`k8s.traefik-dashboard.yaml`) exposes `/api` and `/dashboard` paths at `traefik.<domain>`, protected by the full middleware chain (`middleware-playonjan`, `middleware-antimitm`, `sso-domainx`), with TLS from cert-manager.

---

## argo

Argo Workflows is deployed in its own `argo` namespace. `deploy.sh` creates the namespace then applies the upstream install manifest directly from GitHub (`v3.5.4`), then waits for `workflow-controller` and `argo-server` Deployments to become available (300 s timeout). The Argo UI is not externally exposed; it can be reached via `kubectl port-forward svc/argo-server -n argo 2746:2746`.

---

## Architecture: how the layers fit together

```
External client
      │  HTTPS (via Cloudflare CDN)
      ▼
NetworkPolicy (Cloudflare CIDRs + LAN only → ports 80/443)
      │
Traefik (3 replicas, spread across traefik-node=true hosts)
      │  entrypoints: web → redirect HTTPS, websecure, tcpep, udpep
      │
      ├─► middleware-antimitm  (Go IP-filter → blocks DLP proxy CIDRs)
      ├─► middleware-playonjan (Go KV-filter → shared secret header gate)
      ├─► sso-domainx / sso-domaint / …  (Google OAuth2 forward-auth per domain)
      └─► Sablier middleware   (scale-to-zero for idle workloads)
                │
                ▼
          Backend Pods (TLS terminated, cert-manager DNS-01 certificates)
```

- **cert-manager** issues all TLS certificates via Let's Encrypt DNS-01 (Cloudflare). No HTTP-01 challenge ports are needed.
- **Calico** enforces NetworkPolicy across pods; the IPv6 IPPool is defined and ready to activate.
- **SSO** is domain-scoped: each of the six DNS domains has independent Google OAuth2 credentials and per-route email whitelists. Cookie lifetime is 1 year, so repeat authentication is rare.
- **Sablier** reduces idle resource consumption cluster-wide; the reschedule/restart CronJobs keep workloads healthy after node reboots or transient failures.
- **NFS PVs** (`ReadWriteMany`) allow multiple pods on different nodes to share the same storage; the `local-path-retain` class handles node-local state that must survive pod restarts.

---

## Cross-references

- [Koishi Cluster README](../README.md) — Overview of the home-lab infrastructure, deployment patterns, and security model.
- [`traefik/`](traefik/) — manages the Traefik config; Traefik itself is a k3s-bundled Helm chart
- [`cert-manager/`](cert-manager/) — Cloudflare DNS-01 `ClusterIssuer` for wildcard TLS
- [`sso/`](sso/) — per-domain forward-auth middleware groups
- [`../deploy/`](../deploy/) — Ruby-based `my-k8s-deploy` wrapper used by every `deploy.sh` here
- [`../scripts/`](../scripts/) — `my-k8s-build-image` Kaniko builder used by middleware, reschedule, and restart
- [`../database/`](../database/) — PostgreSQL/Redis; depends on `volume/` NFS PVCs
- [`../coder/`](../coder/) — uses `nfs-disk-coder-workspaces` and `nfs-disk-coder-sharepoint`
- [`../vaultwarden/`](../vaultwarden/) — secret store; all `__{{...}}__` template placeholders resolve against it at deploy time
