# Vaultwarden Implementation Details & Operations (Agent Context)

This document complements [README.md](file:///mnt/coder-workspaces/private-workspace/repos/local/polyphony/koishi/vaultwarden/README.md) by detailing the implementation, configurations, and operational workflows of the Vaultwarden cluster services.

---

## Component Details

### Main Deployment (`k8s.app.yaml`, `k8s.namespace.yaml`)

- **Namespace**: `default-vaultwarden`
- **Deployment**:
  - **Image**: `vaultwarden/server:latest`
  - **Port**: 80 (internal ClusterIP)
  - **Storage**: Mounts `/mnt/runtime-data-app/vaultwarden` on node `nur` for database persistence.
  - **Resource requests**: 100m CPU, 50Mi memory.
  - **Network Policy**: Strict egress deny-all.
  - **Ingress routes**: 
    - Public: `vaultwarden.__{{infra.domains:f:x}}__/` (no authentication, weight 30).
    - Admin: `vaultwarden.__{{infra.domains:f:x}}__/admin` (protected by `sso-domainx` middleware, weight 40).
  - **TLS**: Auto-managed by cert-manager with DNS-01 Cloudflare challenge.
  - **Limits**: 525MB request body limit configured on Traefik buffering middleware.

### BWW: BitWarden for Webapp (`bww/`)

Exposes a custom API wrapping the Bitwarden CLI (`bw`) to serve secrets dynamically.

- **API Logic (`code/main.go`)**:
  - Caches Bitwarden items in memory at startup.
  - Requires HMAC-SHA256 signature (`X-BWW-Timestamp` and `X-BWW-Signature` headers) with a 5-minute window for all authenticated requests.
- **Endpoints**:
  - `GET /{name}/password`: Retrieve item password.
  - `GET /{name}/field/{field_name}`: Retrieve custom field.
  - `GET /{name}/field/{field_name}/base64`: Retrieve base64 custom field.
  - `GET /{name}/attachment/{filename}`: Download attachment.
  - `GET /{name}/attachment/{filename}/base64`: Download base64 attachment.
  - `UPDATE /sync`: Sync CLI vault and reload cache.
  - `POST /render`: Render files with secret placeholders.
  - `GET /ok`: Public health check.
- **Environment Settings**:
  - `BW_URL`: Server endpoint.
  - `BW_CLIENTID` / `BW_CLIENTSECRET`: API keys.
  - `BW_PASSWORD`: Master password to unlock.
  - `BWW_SECRET_KEY`: HMAC secret key.
- **Placeholder Syntax for `/render`**:

| Placeholder                  | Example                                                                     | Meaning                             |
|:-----------------------------|:----------------------------------------------------------------------------|:------------------------------------|
| `__{{name}}__`               | `__{{koishi.litellm}}__`                                                    | Password of item `name`             |
| `__{{name:_:b64}}__`         | `__{{koishi.litellm:_:b64}}__`                                              | Base64-encoded password             |
| `__{{name:f:field}}__`       | `__{{infra.common-users:f:hh}}__`                                           | Custom field `field` of item `name` |
| `__{{name:f:field:f:b64}}__` | `__{{infra.common-users:f:hh:f:b64}}__`                                     | Base64-encoded custom field         |
| `__{{name:a:file}}__`        | `__{{koishi.foldersync.sshkeys.system.files:a:ssh_host_ecdsa_key}}__`       | Attachment content                  |
| `__{{name:a:file:a:b64}}__`  | `__{{koishi.foldersync.sshkeys.system.files:a:ssh_host_ecdsa_key:a:b64}}__` | Base64-encoded attachment           |

### Portwarden Backup Tool (`portwarden/`)

Runs nightly logical backups.

- **Workflow (`entrypoint.sh`)**:
  1. Login and unlock `bw` CLI using API credentials.
  2. Export raw vault JSON via Portwarden format.
  3. Retrieve pass from `URL_PASS` (`http://fs-downserver.default.svc.cluster.local/private/thepassbase`).
  4. Derive GPG key: `sha256sum(passphrase)` in hex.
  5. Encrypt: `gpg --batch --passphrase <key> -c <file>.zip`.
  6. Save to `/data/export/` with `0700` permissions.
- **CronJob Settings**:
  - **Schedule**: `30 0 * * *` (00:30 UTC nightly).
  - **Output destination**: `/mnt/user-data-others/ebackup/upload/local/vaultwarden/portwarden/` on node `nur`.

### Vaultwarden Data Directory Backup (`vaultwardendatabackup/`)

Provides complete filesystem snapshot backup capability.

- **Workflow (`entrypoint.sh`)**:
  1. Archive `/data/vaultwarden` to a temporary ZIP.
  2. Get passphrase from `URL_PASS`.
  3. Derive GPG key as hex sha256sum.
  4. Encrypt ZIP with GPG.
  5. Move to `/data/local/` with `0700` permissions.
- **Job Settings**:
  - Manually run K8s Job, maps `/data/vaultwarden/data` volume as read-only.
  - Output destination: `/mnt/user-data-others/ebackup/upload/local/vaultwarden/vaultwardendatabackup/`.

---

## Network Security Specifications

- **Vaultwarden Pod Egress Restrictions**: Denies all outbound traffic.
- **Backup Pod Egress Permissions**:
  - Port 53/UDP to CoreDNS.
  - Port 80/443 to Traefik (internal proxy routing).
  - Cluster IP of `fs-downserver` in `default` namespace.
  - Cluster IP of `vaultwarden` in `default-vaultwarden` namespace.
  - Subnet `__{{koishi.infra.infos:f:subnetv4}}__.0/24` for authorized local external interfaces.

---

## Integration Details

- **Homepage Dashboard**:
  - Configures endpoints in `gethomepage` with custom weighting (public weight 30, admin weight 40).
- **Cluster-wide Secret Source**:
  - Workloads authenticate to BWW via HMAC to fetch secrets or perform template rendering.

---

## Operations & Commands

### Build Procedures

Build all container images:
```bash
./build.sh                           # Builds all subcomponents
./bww/build.sh                       # Build bww only
./portwarden/build.sh                # Build portwarden only
./vaultwardendatabackup/build.sh     # Build vaultwardendatabackup only
```

### Deployment Procedures

```bash
./deploy.sh
```
Executes sequence:
1. Apply `k8s.namespace.yaml`.
2. Apply `k8s.app.yaml` (Deploys vaultwarden & portwarden).
3. Execute `bww/deploy.sh` (Deploys BWW API wrapper).

*All manifest deployment commands parse parameters via `my-k8s-deploy` to resolve placeholders.*

### Manual Backup Trigger

To execute a complete data directory backup manually:
```bash
./vaultwardendatabackup/run.sh
```
*Note: This script automatically handles scaling the vaultwarden replica count to 0, running the job, printing logs, and restoring the replicas to 1.*

### Backup Encryption & Recovery Procedures

To decrypt either logical (portwarden) or physical (datadir) backups:

```bash
# 1. Fetch passphrase from security service
curl http://fs-downserver.default.svc.cluster.local/private/thepassbase > /tmp/pass

# 2. Derive key (hex SHA256)
key=$(sha256sum /tmp/pass | awk '{print $1}')

# 3. Perform GPG Decryption
gpg --batch --passphrase "$key" -d backup.YYYYMMDD-HHMMSS.portwarden.decrypted.zip.gpg > backup.zip
```

---

## Troubleshooting Details

- **Vaultwarden Access**: Verify Traefik routing rules and check cert-manager logs for Cloudflare challenge status.
- **BWW HMAC Errors**: Confirm `BWW_SECRET_KEY` consistency between generator and consumer, and inspect timezone/time skew (must be <5m offset).
- **Backup Failures**: Ensure `fs-downserver` is running and healthy. Confirm database volumes are correctly mounted read-only during data directory backups.
