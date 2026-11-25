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
    - Public: `vaultwarden.{{ tacet://org/Default/DevOps/Base/infra-domains/x }}/` (no authentication, weight 30).
    - Admin: `vaultwarden.{{ tacet://org/Default/DevOps/Base/infra-domains/x }}/admin` (protected by `sso-domainx` middleware, weight 40).
  - **TLS**: Auto-managed by cert-manager with DNS-01 Cloudflare challenge.
  - **Limits**: 525MB request body limit configured on Traefik buffering middleware.

### Tacet Integration

Build and deployment tooling accesses Vaultwarden through `tacet-wrapper`:

- `tacet-wrapper read "tacet://org/Default/DevOps/Koishi/item-name/password"` retrieves an item password.
- `tacet-wrapper read "tacet://org/Default/DevOps/Koishi/item-name/field-name"` retrieves a custom field; a filename selects an attachment.
- `tacet-wrapper inject` resolves placeholders in configuration templates.
- `scripts/tacet-source-env` loads `TACET_SERVER`, `TACET_CLIENT_ID`, `TACET_CLIENT_SECRET`, and `TACET_PASSWORD` from the protected environment file. The wrapper logs in when needed and unlocks locked sessions before executing the command.

### Tacet Template Syntax

Build and deployment templates use `tacet-wrapper inject` with the following syntax:

URIs must specify the organization and full collection path: `tacet://org/<organization>/<collection-path>/<name>/<selector>`. These examples use organization `Default` and collection `DevOps/Koishi`; `name`, `field`, and `file` in the placeholder column are values to replace. Wildcard URI lookup is being removed; use explicit paths for all new references.

| Placeholder                                                             | Example                                                                                                             | Meaning                             |
| :---------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------------ | :---------------------------------- |
| `{{ tacet://org/Default/DevOps/Koishi/name/password }}`                 | `{{ tacet://org/Default/DevOps/Koishi/koishi.litellm/password }}`                                                   | Password of item `name`             |
| `{{ tacet://org/Default/DevOps/Koishi/name/password?encoding=base64 }}` | `{{ tacet://org/Default/DevOps/Koishi/koishi.litellm/password?encoding=base64 }}`                                   | Base64-encoded password             |
| `{{ tacet://org/Default/DevOps/Koishi/name/field }}`                    | `{{ tacet://org/Default/DevOps/Koishi/infra.common-users/hh }}`                                                     | Custom field `field` of item `name` |
| `{{ tacet://org/Default/DevOps/Koishi/name/field?encoding=base64 }}`    | `{{ tacet://org/Default/DevOps/Koishi/infra.common-users/hh?encoding=base64 }}`                                     | Base64-encoded custom field         |
| `{{ tacet://org/Default/DevOps/Koishi/name/file }}`                     | `{{ tacet://org/Default/DevOps/Koishi/koishi.foldersync.sshkeys.system.files/ssh_host_ecdsa_key }}`                 | Attachment content                  |
| `{{ tacet://org/Default/DevOps/Koishi/name/file?encoding=base64 }}`     | `{{ tacet://org/Default/DevOps/Koishi/koishi.foldersync.sshkeys.system.files/ssh_host_ecdsa_key?encoding=base64 }}` | Base64-encoded attachment           |

### Portwarden Backup Tool (`portwarden/`)

Runs nightly logical backups.

- **Workflow (`entrypoint.sh`)**:
  1. Login and unlock `bw` CLI using API credentials.
  2. Export raw vault JSON via Portwarden format.
  3. Retrieve pass from `URL_PASS` (`http://fs-downserver-private.default.svc.cluster.local/private/thepassbase`).
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
  - Subnet `{{ tacet://org/Default/DevOps/Koishi/Base/infra-infos/subnetv4 }}.0/24` for authorized local external interfaces.

---

## Integration Details

- **Homepage Dashboard**:
  - Configures endpoints in `gethomepage` with custom weighting (public weight 30, admin weight 40).
- **Cluster-wide Secret Source**:
  - Build and deployment tooling resolves vault values and templates through `tacet-wrapper read` and `tacet-wrapper inject`.

---

## Operations & Commands

### Build Procedures

Build the backup container images:
```bash
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
curl http://fs-downserver-private.default.svc.cluster.local/private/thepassbase > /tmp/pass

# 2. Derive key (hex SHA256)
key=$(sha256sum /tmp/pass | awk '{print $1}')

# 3. Perform GPG Decryption
gpg --batch --passphrase "$key" -d backup.YYYYMMDD-HHMMSS.portwarden.decrypted.zip.gpg > backup.zip
```

---

## Troubleshooting Details

- **Vaultwarden Access**: Verify Traefik routing rules and check cert-manager logs for Cloudflare challenge status.
- **Tacet Authentication Errors**: Check `TACET_SERVER`, `TACET_CLIENT_ID`, `TACET_CLIENT_SECRET`, and `TACET_PASSWORD` in the protected environment file. Use `tacet status` to inspect the session, `tacet login --apikey` to authenticate again, and `tacet unlock` to unlock an existing session.
- **Backup Failures**: Ensure `fs-downserver` is running and healthy. Confirm database volumes are correctly mounted read-only during data directory backups.
