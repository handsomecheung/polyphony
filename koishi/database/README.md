# Koishi Database Infrastructure

Multi-tier PostgreSQL and Redis deployment across development and production environments, backed by automated backup-to-cloud and unified web-based database access layer (dbgate) with fine-grained connection management and network isolation.

## Overview

The database module provides stateful data persistence for all koishi services through three deployment tiers:

1. **Local tier** (`database` namespace): Single-node PostgreSQL 18.1 and Redis for development, pinned to the `nur` node with hostPath storage at `/mnt/runtime-data-app/postgres.local`.
2. **Development tier** (`ck-dev` namespace): CloudNativePG 2-replica HA PostgreSQL cluster (1 Gi storage) and Redis instance with automated 3-day GCS backup retention.
3. **Production tier** (`ck-prod` namespace): CloudNativePG 3-replica HA PostgreSQL cluster (10 Gi storage) and Redis instance with automated 30-day GCS backup retention.

Access to all databases is mediated through **dbgate**, a unified web-based management UI running in three variants: full (all 13 connections) and lite (scale-to-zero via Sablier). Network policies enforce strict egress control, limiting outbound traffic to DNS, traefik ingress, internal namespaces, and specific external AWS RDS endpoints.

## Directory Structure

### `deploy.sh`
Main deployment orchestrator that invokes sub-deployments in sequence:
- Namespace and shared Redis instances
- Local development database
- CNPG clusters (dev and prod)
- dbgate tools

## `local/` – Development Database

Single-node PostgreSQL and Redis for local development, node-pinned to `nur` for consistent pod scheduling.

**Files:**
- `k8s.app.postgres.yaml`: PostgreSQL 18.1 Deployment with 1 replica, 100m CPU / 200Mi memory requests, hostPath volume at `/mnt/runtime-data-app/postgres.local`, liveness probe via `pg_isready`. ClusterIP Service on port 5432.
- `k8s.app.redis.yaml`: Redis Stack Server Deployment (in-memory, no persistence), 1 replica, 100m/256Mi requests, 500m/1Gi limits. ClusterIP Service on port 6379.
- `deploy.sh`: Deploys both via `my-k8s-deploy` wrapper.

**Access:** `postgres.database` (port 5432), `redis.database` (port 6379)

## `cnpg/` – CloudNativePG Operator & HA Clusters

CloudNativePG-based PostgreSQL clusters with automated backup to Google Cloud Storage via Barman Cloud plugin.

### `cnpg/install.sh`
Installs CloudNativePG operator (v1.29.1) and Barman Cloud plugin (v0.12.0) into `cnpg-system` namespace. Run once before deploying any CNPG clusters.

### `cnpg/ck-dev/` – Development Cluster

**Files:**
- `k8s.app.cluster.yaml`:
  - `Secret` (gcs-creds): Base64-encoded Google Service Account JSON for Barman Cloud backup to `gs://koishi-cnpg-backup-dev/backups/` with 3-day retention, gzip-compressed WAL archiving.
  - `Secret` (postgres-superuser): Postgres superuser password (templated).
  - `ObjectStore` (postgres-backup): Barman Cloud configuration specifying GCS destination and retention policy.
  - `Cluster` (postgres): 2-replica HA PostgreSQL 18, 1 Gi storage via `local-path-retain` storage class, managed roles.
  - `Service` (postgres): ClusterIP routing to primary instance on port 5432, also exposes `postgres-rw.ck-dev` for read-write access.

**Access:** `postgres.ck-dev` (primary), `postgres-rw.ck-dev` (read-write)

### `cnpg/ck-prod/` – Production Cluster

**Files:**
- `k8s.app.cluster.yaml`: Identical structure to ck-dev but:
  - 3-replica HA cluster (higher availability for production).
  - 10 Gi storage.
  - 30-day GCS backup retention to `gs://koishi-cnpg-backup-prod/backups/`.
  - Managed roles.

**Access:** `postgres.ck-prod` (primary on port 5432)

## `k8s.app.namespace.yaml`
Defines the shared `database` namespace for local and coordinator services.

## `k8s.app.redis.ck-{dev,prod}.yaml`
Redis Stack Server instances deployed in `ck-dev` and `ck-prod` namespaces respectively, mirroring the local Redis configuration (1 replica, in-memory, no persistence). ClusterIP Services on port 6379.

## `tools/` – Database Access & Management

### `tools/dbgate/` – Custom dbgate Image

Builds a dbgate image with the `sqlite-vec` extension for vector database support in SQLite.

**Files:**
- `Dockerfile`: Based on `dbgate/dbgate:latest`, installs `sqlite-vec-linux-x64` npm package, patches the SQLite plugin to load the extension, and sets correct permissions.
- `patch-sqlite-plugin.js`: Node.js script that injects sqlite-vec loading into the minified dbgate SQLite backend. Finds and replaces the `connect` function to call `db.loadExtension("/home/dbgate-docker/node_modules/sqlite-vec-linux-x64/vec0.so")` at database connection time.
- `build.sh`: Invokes `my-k8s-build-image` to build the custom image as `cloudpublic/default/dbgate:latest` within the cluster.

### `tools/k8s.app.dbgate.yaml`

Three dbgate variants, all deployed in the `default` namespace:

#### **dbgate (Full)**
- **Image:** `cloudpublic/default/dbgate:latest` (custom build with sqlite-vec)
- **Replicas:** 1 (always on)
- **Connections (13 total):**
  - PostgreSQL: `postgres.database` (local), `postgres-rw.ck-dev` (dev), `postgres.ck-prod` (prod) 
  - MySQL: AWS RDS instances (`ci-jsf` dev and prod)
  - MSSQL: `ci-ipass` (system, euc-dev, euc-prod, library)
  - SQLite: subscout, openclaw (default and kzk variants)
- **Volumes:** hostPath mounts to subscout and openclaw SQLite databases from NFS coder-workspaces PVC.
- **Networking:** ConfigMap with `/etc/hosts` entries for AWS RDS resolution. NetworkPolicy egress rules limit outbound to:
  - kube-dns (UDP 53)
  - traefik pods
  - Local subnet (`__{{koishi.infra.infos:f:subnetv4}}__.0/24`)
  - `database`, `ck-dev`, `ck-prod`, `ci-ipass` namespaces
  - Specific AWS RDS IP addresses (port 3306)
- **Ingress:** HTTPS via Traefik (`dbgate.__{{infra.domains:f:x}}__`) with cert-manager DNS-01 (Cloudflare). Middlewares: playonjan (auth), antimitm, sso-domainx (SSO). Homepage integration (Development group, weight 40).
- **Resources:** 100m CPU, 256Mi memory requests.

#### **dbgate-lite**
- **Image:** `dbgate/dbgate:latest` (vanilla, no sqlite-vec)
- **Replicas:** 0 (scale-to-zero via Sablier middleware)
- **Connections (6 subset):** PostgreSQL (ck-dev only), MySQL (ci-jsf dev/prod), MSSQL (ci-ipass all)
- **Ingress:** HTTPS (`dbgate-lite.__{{infra.domains:f:x}}__`). Middleware includes Sablier scale-to-zero trigger (group `dbgate-lite`, 1h session duration).
- **Purpose:** On-demand, minimal-resource variant for occasional DBA access without keeping full instance running.

### `tools/deploy.sh`
Deploys the full dbgate YAML via `my-k8s-deploy`.

## Secret Management

All sensitive values (PostgreSQL passwords, GCS credentials, external database passwords) are templated using the `__{{path:modifiers}}__` syntax and injected at deploy time by the `my-k8s-deploy` wrapper from a centralized secret store (Vaultwarden). Examples:
- `__{{database.postgres-local.accounts:f:postgres}}__` – Local postgres user password
- `__{{database.postgres.ck-dev.accounts:f:postgres}}__` – ck-dev postgres user password
- `__{{database.postgres.ck-dev.accounts:a:koishi-cnpg-backup-sa.json:a:b64}}__` – Base64-encoded GCS service account key
- `__{{infra.domains:f:x}}__` – Base domain for ingress hostnames

## Storage

All deployments use **local-path-retain** storage class for PersistentVolumes:
- **Local PostgreSQL**: 1 hostPath volume at `/mnt/runtime-data-app/postgres.local` (node-pinned to `nur`).
- **CNPG clusters**: Kubernetes-managed PVCs (1 Gi for dev, 10 Gi for prod) handled by CNPG's internal replication mechanism.
- **SQLite databases**: NFS-mounted from `coder-workspaces` PVC, read-only snapshots where applicable.

## Networking & Security

**Namespaces:**
- `database`: Local development postgres/redis and shared coordinator.
- `ck-dev`, `ck-prod`: CNPG clusters and environment-specific Redis.
- `default`: dbgate ingress access points.
- `cnpg-system`: CloudNativePG operator and Barman Cloud plugin.

**Ingress:**
- All dbgate endpoints exposed via **Traefik** with **Cert-manager** DNS-01 validation (Cloudflare DNS provider).
- TLS hostnames: `dbgate.<domain>`, `dbgate-lite.<domain>`, `dbgate-cc.<domain>`.
- Middleware stack: playonjan (password), antimitm (HTTPS enforcement), sso-domainx (SSO token validation).

**NetworkPolicy:**
- dbgate pods restricted to egress-only rules: DNS lookup allowed, traefik routing allowed, internal namespace access permitted, external AWS RDS access limited to specific IPs/ports.

## Backup & Disaster Recovery

**CNPG + Barman Cloud:**
- Automated continuous archiving of WAL (Write-Ahead Log) files to GCS with gzip compression.
- Backup retention: 3 days (dev), 30 days (prod).
- Barman Cloud plugin (v0.12.0) manages lifecycle and recovery procedures.
- GCS credentials injected as Kubernetes Secret in each cluster namespace.

**Recovery process:**
1. CNPG supports point-in-time recovery (PITR) using archived WAL and backup snapshots.
2. Manual restore documented via CNPG's recovery procedures and Barman Cloud recovery tool.

## Cross-Namespace Service Discovery

PostgreSQL and Redis services are accessible cluster-wide via DNS:
- `postgres.database.svc.cluster.local` – Local postgres
- `postgres.ck-dev.svc.cluster.local` – ck-dev primary
- `postgres-rw.ck-dev.svc.cluster.local` – ck-dev read-write
- `postgres.ck-prod.svc.cluster.local` – ck-prod primary
- `redis.database.svc.cluster.local` – Local redis
- `redis.ck-dev.svc.cluster.local` – ck-dev redis
- `redis.ck-prod.svc.cluster.local` – ck-prod redis

Applications in other namespaces reference these via standard Kubernetes DNS resolution.

## Deployment Steps

1. **Install CloudNativePG operator (one-time):**
   ```bash
   ./cnpg/install.sh
   ```

2. **Deploy entire database stack:**
   ```bash
   ./deploy.sh
   ```

3. (Optional) **Build custom dbgate image with sqlite-vec:**
   ```bash
   ./tools/dbgate/build.sh
   ```

4. **Verify deployments:**
   ```bash
   kubectl get pods -n database
   kubectl get pods -n ck-dev
   kubectl get pods -n ck-prod
   kubectl get pods -n cnpg-system
   ```

## Integration with Koishi Services

This database module serves as the primary stateful backend for:
- **formant** – Primary application database (ck-prod postgres).
- **ckhome** – Shared home automation database (roles in both ck-dev and ck-prod).
...

See parent [koishi README](../README.md) for cluster-wide architecture.
