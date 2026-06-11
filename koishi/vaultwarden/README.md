# Vaultwarden: Self-Hosted Password Manager & Secret Store

Provides a self-hosted Bitwarden-compatible password manager (vaultwarden) with a custom HTTP API wrapper (bww) for templated secret injection into configs, and automated encrypted backups of vault data. Serves as the cluster's centralized secret source with strict network policies, encrypted backup exports, and inter-pod secret templating.

## Architecture

The vaultwarden deployment is organized into three main components:

1. **Vaultwarden Server**: Upstream vaultwarden/server container running the vault REST API and web UI
2. **BWW (BitWarden for Webapp)**: Custom Go API server wrapping the `bw` CLI with template rendering for secret injection
3. **Backup System**: Two parallel backup strategies (portwarden item exports + full data directory archives) with GPG symmetric encryption

All pods are pinned to node `nur` via nodeSelector and communicate over a restricted Kubernetes network policy that blocks vaultwarden egress entirely and limits portwarden/datadirbackup to DNS, Traefik, and external HTTPS only.

## Components

### Main Deployment (`k8s.app.yaml`, `k8s.namespace.yaml`)

Creates namespace `default-vaultwarden` and deploys the core vaultwarden service:

**Vaultwarden Deployment**:
- **Image**: `vaultwarden/server:latest` (upstream Bitwarden fork)
- **Port**: 80 (internal ClusterIP)
- **Storage**: Mounts `/mnt/runtime-data-app/vaultwarden` on node `nur` for persistent data and logs
- **Resource requests**: 100m CPU, 50Mi memory
- **Network policy**: Strict egress deny-all (no outbound connections)
- **Ingress routes**: 
  - Public endpoint: `vaultwarden.__{{infra.domains:f:x}}__/` (no auth, weight 30)
  - Admin endpoint: `vaultwarden.__{{infra.domains:f:x}}__/admin` (SSO-protected with sso-domainx middleware, weight 40)
- **TLS**: cert-manager with cluster-letsencrypt-dns-cloudflare issuer
- **Request body limit**: 525MB (via Traefik buffering middleware sizelimit)

**Middleware chain**: Traefik applies `playonjan` and `antimitm` policies to both public and admin routes.

**Service**: ClusterIP on port 80, no external access. Reachable by other pods via `vaultwarden.default-vaultwarden.svc.cluster.local`.

### BWW: BitWarden for Webapp (`bww/`)

Custom Go application that wraps the Bitwarden CLI and exposes an HTTP API for templated secret retrieval. Enables declarative infrastructure-as-code with dynamic secrets injected at deployment time.

**Subcomponents**:

- **`code/main.go`**: Core API server (1,100+ lines)
  - **Cache**: Loads all Bitwarden items into memory on startup for fast retrieval
  - **Authentication**: HMAC-SHA256 request signing with `X-BWW-Timestamp` and `X-BWW-Signature` headers; 5-minute clock window prevents replay attacks
  - **Endpoints**:
    - `GET /{name}/password`: Retrieve password for item
    - `GET /{name}/field/{field_name}`: Retrieve custom field value
    - `GET /{name}/field/{field_name}/base64`: Base64-encoded custom field
    - `GET /{name}/attachment/{filename}`: Download binary attachment
    - `GET /{name}/attachment/{filename}/base64`: Base64-encoded attachment
    - `UPDATE /sync`: Trigger `bw sync` and reload cache
    - `POST /render`: Template rendering endpoint (see "Secret Injection" section below)
    - `GET /ok`: Health check (no auth required)

- **`k8s.app.yaml`**: Kubernetes Deployment + Service + Ingress
  - **Image**: `cloudpublic/default/bww:latest` (custom Go binary)
  - **Port**: 8080 internal, 80 via ClusterIP Service
  - **Replicas**: 1
  - **Environment**:
    - `BW_URL`: Vaultwarden server URL (templated as `https://vaultwarden.__{{infra.domains:f:x}}__`)
    - `BW_CLIENTID`, `BW_CLIENTSECRET`: API credentials (templated)
    - `BW_PASSWORD`: Master password to unlock vault (templated)
    - `BWW_SECRET_KEY`: HMAC secret key for request signing (templated)
  - **Health checks**: Liveness and readiness probes on `/ok` (10s initial delay, 10s period)
  - **Ingress**: Exposes at `bww.home4p.__{{infra.domains:f:x}}__` with TLS
  - **Resource requests**: 30m CPU, 50Mi memory

- **`Dockerfile`**: Multi-stage build
  - **Stage 1**: Go 1.25 builder; downloads Bitwarden CLI v2024.6.0, compiles main.go to binary
  - **Stage 2**: Ubuntu 24.04 runtime; includes curl and ca-certificates
  - **Entrypoint**: `/usr/local/bin/entrypoint.sh`

- **`entrypoint.sh`**: Initializes bw CLI and unlocks vault on container startup
  - Configures bw server URL
  - Sources API credentials from environment
  - Logs in via `bw login --apikey`
  - Unlocks vault with `bw unlock --passwordenv BW_PASSWORD --raw` and exports session

- **`code/build.sh`**: Invokes `my-k8s-build-image cloudpublic/default/bww:latest`

**Secret Injection**: The `/render` endpoint accepts templated files with placeholders and substitutes them with Bitwarden data:

| Placeholder                  | Example                                                                     | Meaning                             |
|:-----------------------------|:----------------------------------------------------------------------------|:------------------------------------|
| `__{{name}}__`               | `__{{koishi.litellm}}__`                                                    | Password of item `name`             |
| `__{{name:_:b64}}__`         | `__{{koishi.litellm:_:b64}}__`                                              | Base64-encoded password             |
| `__{{name:f:field}}__`       | `__{{infra.common-users:f:hh}}__`                                           | Custom field `field` of item `name` |
| `__{{name:f:field:f:b64}}__` | `__{{infra.common-users:f:hh:f:b64}}__`                                     | Base64-encoded custom field         |
| `__{{name:a:file}}__`        | `__{{koishi.foldersync.sshkeys.system.files:a:ssh_host_ecdsa_key}}__`       | Attachment content                  |
| `__{{name:a:file:a:b64}}__`  | `__{{koishi.foldersync.sshkeys.system.files:a:ssh_host_ecdsa_key:a:b64}}__` | Base64-encoded attachment           |

### Portwarden Backup Tool (`portwarden/`)

Custom Go-based Bitwarden item exporter that encrypts exports with GPG. Runs nightly via Kubernetes CronJob and stores encrypted backups to a local node mount.

**Subcomponents**:

- **Dockerfile**: Multi-stage build
  - **Stage 1**: Go 1.22 builder
    - Clones https://github.com/handsomecheung/portwarden (branch `get-seesion-from-env`)
    - Generates a random 24-character salt via `/utils/generate_salt_file.go`
    - Builds portwarden binary with CGO disabled
    - Downloads Bitwarden CLI v2024.6.0
  - **Stage 2**: Ubuntu 22.04 runtime; includes curl and gnupg
  - **Entrypoint**: `/data/entrypoint.sh`

- **`entrypoint.sh`**: Backup workflow
  1. Configures `bw` CLI to connect to vaultwarden server (via `URL_VW` env var)
  2. Sources API credentials from `/.env`
  3. Logs in and unlocks vault via `bw unlock --passwordenv BW_PASSWORD --raw`
  4. Exports all vault items to portwarden encrypted format
  5. Retrieves passphrase from `URL_PASS` endpoint (fs-downserver in default namespace)
  6. Derives symmetric key as `sha256sum(passphrase)` in hex
  7. Encrypts export with GPG using derived key: `gpg --batch --passphrase <key> -c <file>.zip`
  8. Moves encrypted file to `/data/export/` with restrictive permissions (go-rwx)
  9. Cleans up temporary directories

- **`build.sh`**: Invokes `my-k8s-build-image cloudprivate/default/portwarden:latest`

- **`env`**: Placeholder file for Bitwarden API credentials (sourced at runtime)

**Kubernetes CronJob (`k8s.app.yaml` lines 167–221)**:
- **Schedule**: `30 0 * * *` (00:30 UTC nightly)
- **Concurrency policy**: Forbid (prevents overlapping backups)
- **History limits**: Keep 2 successful, 5 failed job records
- **Node pinning**: `kubernetes.io/hostname: nur`
- **Volumes**:
  - `dns-config` ConfigMap: Maps vaultwarden IP to hostname
  - `env`: hostPath file with Bitwarden credentials (`.env`)
  - `local`: hostPath `/mnt/user-data-others/ebackup/upload/local/vaultwarden/portwarden/` (output directory)
- **Environment variables**:
  - `URL_VW`: Vaultwarden HTTPS URL (templated)
  - `URL_PASS`: Passphrase endpoint via fs-downserver (http://fs-downserver.default.svc.cluster.local/private/thepassbase)
- **Resource requests**: 50m CPU, 100Mi memory
- **Image pull policy**: Always

**Output**: Encrypted backups written as `backup.YYYYMMDD-HHMMSS.portwarden.decrypted.zip.gpg` to `/data/export/` with mode `0700` (owner read/write only).

### Vaultwarden Data Directory Backup (`vaultwardendatabackup/`)

Standalone Kubernetes Job (manually triggered via `run.sh`) that archives the entire vaultwarden data directory and encrypts it with GPG. Enables full-database recovery independently of portwarden.

**Subcomponents**:

- **`Dockerfile`**: Simple build
  - **Base**: `cloudpublic/base/ubuntu:24.04`
  - **Tools**: curl, gnupg, zip
  - **Entrypoint**: `/data/entrypoint.sh`

- **`entrypoint.sh`**: Full data backup workflow
  1. Archives entire `/data/vaultwarden` directory to a ZIP file with timestamp
  2. Retrieves passphrase from `URL_PASS` endpoint (same as portwarden)
  3. Derives symmetric key as `sha256sum(passphrase)` in hex
  4. Encrypts ZIP with GPG: `gpg --batch --passphrase <key> -c <file>.zip`
  5. Moves encrypted file to `/data/local/` with restrictive permissions (go-rwx)
  6. Cleans up temporary directories

- **`app.yaml`**: Kubernetes Job manifest
  - **Kind**: Job (not CronJob; triggered manually)
  - **Namespace**: default-vaultwarden
  - **Node pinning**: `kubernetes.io/hostname: nur`
  - **Backoff limit**: 4 retries on failure
  - **Restart policy**: Never
  - **Volumes**:
    - `vaultwarden-encrypted`: PersistentVolumeClaim (read-only mount of `/data/vaultwarden/data`)
    - `local`: hostPath `/mnt/user-data-others/ebackup/upload/local/vaultwarden/vaultwardendatabackup/` (output)
  - **Environment**: `URL_PASS` (same as portwarden)
  - **Resource requests**: 50m CPU, 100Mi memory
  - **Working directory**: `/data`

- **`build.sh`**: Invokes `my-k8s-build-image cloudprivate/default/vaultwardendatabackup:latest`

- **`run.sh`**: Helper script to manually trigger the backup
  1. Deploys Job manifest
  2. Scales vaultwarden Deployment to 0 replicas (prevents concurrent access)
  3. Polls for job completion (checks `.status.succeeded == 1` every 5s)
  4. Streams job logs to stdout
  5. Deletes the completed Job
  6. Restores vaultwarden to 1 replica

**Output**: Encrypted full-database archive written as `datadir.YYYYMMDD-HHMMSS.zip.gpg` to `/data/local/` with mode `0700`.

## Network Security

**NetworkPolicy on vaultwarden pod** (lines 226–238):
- **Type**: Egress policy
- **Rule**: Deny all egress traffic
- **Effect**: vaultwarden pod cannot initiate outbound connections; relies on ingress-only communication

**NetworkPolicy on portwarden and datadirbackup pods** (lines 240–294):
- **Type**: Egress policy
- **Rules**:
  1. Allow DNS (UDP 53) to kube-system `kube-dns`
  2. Allow traffic to Traefik (for HTTPS passphrase retrieval from fs-downserver via external routes)
  3. Allow communication to fs-downserver pod in default namespace (passphrase fetch)
  4. Allow communication to vaultwarden pod in same namespace
  5. Allow HTTPS (TCP 80, 443) to external subnet (templated as `__{{koishi.infra.infos:f:subnetv4}}__.0/24`)
- **Effect**: Backup jobs can reach vaultwarden, DNS, fs-downserver, and external HTTPS, but nothing else

## Integration

**Homepage Dashboard**: Both vaultwarden endpoints are registered in gethomepage:
- Vaultwarden public: `gethomepage.dev/weight: "30"` (lower priority)
- Vaultwarden admin: `gethomepage.dev/weight: "40"` (higher priority, SSO-protected)
- Both use templated domain names via bww's render engine

**Cluster-wide Secret Source**: Other Kubernetes deployments can call bww's `/render` endpoint (with HMAC authentication) to inject secrets into configuration files at deployment time. This enables:
- Dynamic secret injection into YAML manifests
- Centralized credential management (all secrets stored in Bitwarden)
- Audit trail of secret access via vaultwarden logs

## Build & Deployment

### Build

Build all container images:

```bash
./build.sh                           # Builds all subcomponents
./bww/build.sh                       # Build bww only
./portwarden/build.sh                # Build portwarden only
./vaultwardendatabackup/build.sh     # Build vaultwardendatabackup only
```

Each invokes `my-k8s-build-image` to build and push to the internal registry (`cloudprivate/default/*` or `cloudpublic/default/*`).

### Deploy

Deploy to Kubernetes cluster:

```bash
./deploy.sh
```

This applies manifests in order:
1. `k8s.namespace.yaml` (creates `default-vaultwarden` namespace)
2. `k8s.app.yaml` (deploys vaultwarden, portwarden CronJob, and network policies)
3. `bww/deploy.sh` (deploys bww API server in the same namespace)

All deployments use `my-k8s-deploy` wrapper, which applies templating (replacing `__{{...}}__` placeholders with values from infrastructure config files).

### Manual Backup Trigger

To manually run the full data directory backup (when vaultwarden is shut down):

```bash
./vaultwardendatabackup/run.sh
```

This scales vaultwarden to 0, runs the backup Job, waits for completion, logs output, and restores vaultwarden to 1 replica.

## Backup Encryption & Recovery

Both backup formats (portwarden exports and data directory archives) are encrypted with GPG symmetric cipher using a key derived from a remote passphrase file:

```
Key = SHA256(passphrase_file_contents)
```

The passphrase is fetched from `fs-downserver` at deployment time and stored only in Kubernetes env vars (not in the cluster). To decrypt a backup:

```bash
# Retrieve passphrase from fs-downserver
curl http://fs-downserver.default.svc.cluster.local/private/thepassbase > /tmp/pass

# Derive key
key=$(sha256sum /tmp/pass | awk '{print $1}')

# Decrypt backup
gpg --batch --passphrase "$key" -d backup.YYYYMMDD-HHMMSS.portwarden.decrypted.zip.gpg > backup.zip
```

Backups are stored on the local node mount `/mnt/user-data-others/ebackup/upload/local/vaultwarden/` (accessible only on node `nur`).

## Troubleshooting

**vaultwarden not accessible**: Check Ingress rules and Traefik middleware. Verify cert-manager certificate is issued and stored in `vaultwarden-domainx` TLS secret.

**bww authentication failures**: Ensure `BWW_SECRET_KEY` is set in `bww/k8s.app.yaml` environment. Verify client clock skew is within 5 minutes (timestamp validation window). Check HMAC calculation matches the server's secret key.

**Backup job hangs**: Verify fs-downserver is reachable (test with `curl` from pod). Check that vaultwarden data directory is readable (Job mounts it read-only). Review Job logs with `kubectl logs -n default-vaultwarden jobs/vaultwardendatabackup`.

**NetworkPolicy blocking traffic**: Inspect egress rules with `kubectl describe networkpolicy -n default-vaultwarden`. Ensure fs-downserver pod has matching labels (`app: fs-downserver`) for policy selector to work.

## Related

- [Vaultwarden GitHub](https://github.com/dani-garcia/vaultwarden)
- [Bitwarden CLI Documentation](https://bitwarden.com/help/cli/)
- [Portwarden Fork](https://github.com/handsomecheung/portwarden) (handsomecheung's branch with get-seesion-from-env)
- Parent directory: [`koishi/`](../README.md)
