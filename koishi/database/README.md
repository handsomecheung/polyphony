# Koishi Database Infrastructure

Multi-tier PostgreSQL and Redis deployment across development and production environments, backed by automated backup-to-cloud and unified web-based database access layer (dbgate) with fine-grained connection management and network isolation.

## Overview

The database module provides stateful data persistence for all koishi services through three deployment tiers:

1. **Local tier** (`database` namespace): Single-node PostgreSQL 18.1 and Redis for development, pinned to the `nur` node with hostPath storage at `/mnt/runtime-data-app/postgres.local`.
2. **Development tier**: CloudNativePG 2-replica HA PostgreSQL cluster (1 Gi storage) and Redis instance with automated 3-day GCS backup retention.
3. **Production tier**: CloudNativePG 3-replica HA PostgreSQL cluster (10 Gi storage) and Redis instance with automated 30-day GCS backup retention.

Access to all databases is mediated through **dbgate**, a unified web-based management UI running in three variants: full and lite (scale-to-zero via Sablier). Network policies enforce strict egress control, limiting outbound traffic to DNS, traefik ingress, internal namespaces, and specific external AWS RDS endpoints.

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

Builds a dbgate image with the `sqlite-vec` extension for vector database support in SQLite.

**Files:**
- `Dockerfile`: Based on `dbgate/dbgate:latest`, installs `sqlite-vec-linux-x64` npm package, patches the SQLite plugin to load the extension, and sets correct permissions.
- `patch-sqlite-plugin.js`: Node.js script that injects sqlite-vec loading into the minified dbgate SQLite backend. Finds and replaces the `connect` function to call `db.loadExtension("/home/dbgate-docker/node_modules/sqlite-vec-linux-x64/vec0.so")` at database connection time.
- `build.sh`: Invokes `my-k8s-build-image` to build the custom image as `cloudpublic/default/dbgate:latest` within the cluster.

## Storage

All deployments use **local-path-retain** storage class for PersistentVolumes:
- **Local PostgreSQL**: 1 hostPath volume at `/mnt/runtime-data-app/postgres.local` (node-pinned to `nur`).
- **CNPG clusters**: Kubernetes-managed PVCs (1 Gi for dev, 10 Gi for prod) handled by CNPG's internal replication mechanism.
- **SQLite databases**: NFS-mounted from `coder-workspaces` PVC, read-only snapshots where applicable.

## Networking & Security

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


See parent [koishi README](../README.md) for cluster-wide architecture.
