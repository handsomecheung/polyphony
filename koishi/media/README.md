# Media

This module is the media streaming and storage orchestration layer for the Koishi cluster. It runs Plex and Jellyfin for video/audio streaming, Roon and Bliss for high-fidelity audio, Samba for LAN file sharing, and openlist as a web file manager. All streaming pods are co-located on the `nur` node alongside the rclone pod, which mounts multiple cloud backends (pikpak WebDAV, PikPak native, Google Drive) as FUSE filesystems shared across pods via bidirectional mount propagation. An automated CronJob pipeline handles nightly encrypted backups of Plex state, the codebase, and PostgreSQL to both PikPak and Google Drive.

## Services

| Service           | Image                             | Ingress             | Notes                                                    |
|-------------------|-----------------------------------|---------------------|----------------------------------------------------------|
| plex              | `cloudpublic/media/plex`          | `plex.<domain>`     | Port 32400; 2 CPU / 4 GiB request                        |
| jellyfin          | `cloudpublic/media/jellyfin`      | `jellyfin.<domain>` | Port 8096; CJK font layer added                          |
| openlist          | `openlistteam/openlist:v4.1.1`    | `openlist.<domain>` | Port 5244; 20 GB request body limit                      |
| bliss             | `romancin/bliss`                  | `bliss.<domain>`    | Port 3220; replica 0, Sablier on-demand                  |
| roon              | `steefdebruijn/docker-roonserver` | `roon.<domain>`     | hostNetwork; RAAT port 9100, web 9330                    |
| samba             | `cloudpublic/media/samba`         | LoadBalancer        | TCP/UDP 137–139, 445                                     |
| rclone            | `cloudpublic/media/rclone`        | —                   | FUSE mount daemon; privileged + SYS\_ADMIN               |
| aliyunpan-decrypt | `cloudprivate/media/aliyunpan`    | —                   | Deployment; decrypts Aliyun → `/mnt/decrypted`           |
| aliyunpan-upload  | `cloudprivate/media/aliyunpan`    | —                   | CronJob (12:00 UTC); encfs reverse-encrypt → upload      |
| backup-prepare    | `cloudpublic/default/backup`      | —                   | CronJob (18:00 UTC); tar+gzip Plex, codebase, pg_dumpall |
| backup-upload     | `bitnami/kubectl`                 | —                   | CronJob (08:00 UTC); kubectl exec into rclone pod        |

## Directory Layout

```
media/
├── plex/          # Plex Media Server
├── jellyfin/      # Jellyfin video server
├── openlist/      # openlist web file manager
├── bliss/         # Bliss music tagger
├── roon/          # Roon audio server
├── samba/         # Samba SMB file sharing
├── rclone/        # rclone FUSE mount daemon + backup scripts
├── backup/        # CronJob backup pipeline (prepare + upload)
├── aliyunpan/     # Aliyun Drive sync/decrypt
└── deploy.sh      # Top-level orchestration script
```

## Deployment

The top-level `deploy.sh` applies all manifests in dependency order:

```bash
./openlist/deploy.sh   # openlist must be up first (rclone initContainer waits for it)
./rclone/deploy.sh
./plex/deploy.sh
./samba/deploy.sh
./bliss/deploy.sh
./backup/deploy.sh
```

Each service directory contains a `deploy.sh` that calls `my-k8s-deploy --file=<manifest>`. Services with custom images include a `build.sh` that calls `my-k8s-build-image` to trigger an in-cluster Kaniko build.

## Components

### rclone

The rclone pod is pinned to node `nur` and holds the FUSE mount layer for all cloud remotes. It runs with `privileged: true` and the `SYS_ADMIN` capability. On start it waits (via initContainer) for openlist to be reachable at `http://openlist/dav`, then mounts six remotes concurrently:

| Mount path                     | Remote                                                                  |
|--------------------------------|-------------------------------------------------------------------------|
| `/mnt/webdav/openlist`         | openlist WebDAV (`webdav-openlist`)                                     |
| `/mnt/webdav/pikpak-readonly`  | PikPak WebDAV (`webdav-pikpak`, read-only protocol)                     |
| `/mnt/remote/pikpak`           | PikPak native (`pikpak`)                                                |
| `/mnt/decrypted/pikpak/backup` | PikPak encrypted backup (`pikpak-encrypted-backup`, rclone crypt)       |
| `/mnt/remote/gdrive`           | Google Drive (`gdrive`)                                                 |
| `/mnt/decrypted/gdrive/backup` | Google Drive encrypted backup (`gdrive-encrypted-backup`, rclone crypt) |

All three host-path directories (`/mnt/remote`, `/mnt/webdav`, `/mnt/decrypted`) use `mountPropagation: Bidirectional` so that mounts created inside the pod are visible to other pods on the same node. The pre-stop hook runs `umount /mnt/webdav/pikpak-readonly` before the pod terminates.

The rclone config template (`rclone/config.template/rclone.conf`) defines all six remote stanzas with secrets injected via template variables at deploy time. Bandwidth is capped at 20 MB/s per remote.

### plex

Extends `plexinc/pms-docker:latest` with a custom health-check script. The deployment requires strict pod affinity with the rclone pod (`topologyKey: kubernetes.io/hostname`) so it lands on `nur` where the FUSE mounts live.

### jellyfin

Extends `jellyfin/jellyfin` with the `fonts-noto-cjk-extra` package for CJK subtitle rendering. Like Plex, it carries a required pod affinity to rclone and an identical 60-second initContainer wait on its pikpak hero-video directory. The hero library merges `nfs-disk-user-data-slight` local paths with pikpak `p/hero/video`. Config and cache are stored on encrypted PVCs (`default-common-encrypted`, `default-slight-encrypted`). The pod runs as UID/GID 1000.

### openlist

Runs `openlistteam/openlist:v4.1.1` pinned to `nur`. The Traefik ingress applies a custom `openlistsizelimit` Middleware that raises the maximum request body to 20 GB, enabling large direct uploads through the browser. The pod mounts upload directories from all three user-data PVCs plus `share-point` and private hero data paths from `nfs-disk-user-data-slight`. A 30 GiB emptyDir is provided for temporary upload buffering.

### bliss

Runs `romancin/bliss` at **replica 0** — the pod is never scheduled until a user triggers it through the ingress. A Sablier `Middleware` (`sablierbliss`) intercepts every ingress request; if no session is active it spins the pod up and shows a loading page, then forwards the request once the pod is ready. Sessions last one hour. An initContainer copies a seed config from the shared `commondata` volume into a 1 GiB emptyDir so the persistent config directory is always fresh. Music for tagging is mounted from `/mnt/user-data-music/tagging`.

### roon

Runs `steefdebruijn/docker-roonserver` with `hostNetwork: true` and `dnsPolicy: ClusterFirstWithHostNet` so Roon can bind its discovery and audio transport ports directly on the host network interface. Pinned to `nur`. Data, app, and backup subdirectories are mounted from `/mnt/runtime-data-app/roon` via hostPath. Local music is provided from `nfs-disk-user-data-music` (subPath `music/warehouse`, read-only). The ingress forwards the Roon web display at port 9330.

### samba

Extends `dperson/samba:latest`. Uses a `LoadBalancer` Service that exposes UDP 137/138 (NetBIOS) and TCP 139/445 (SMB) directly onto the LAN. Pod affinity to rclone is required so the same `/mnt/remote`, `/mnt/webdav`, and `/mnt/decrypted` host-path mounts are visible inside the Samba container. Two users are injected via template variables (`infra.common-users:f:hh`, `infra.common-users:f:cc`). Notable SMB globals set via environment variables: `smb encrypt = desired`, ACL xattr support, 100 max connections, and optimized TCP socket options.

The shared `LocalSharePoint` share is exported from `nfs-disk-user-data-slight` subPath `share-point`. Additional writable mounts cover upload staging areas for music and others data.

### backup

The backup pipeline consists of two CronJobs and a dedicated `backup-sa` ServiceAccount with RBAC permission to `get/list pods` and `create pods/exec` against the rclone Deployment.

**backup-prepare** (runs at 18:00 UTC daily, node `nur`) — executes `backup/scripts/backup.py` which performs three conditional tasks, each skipped if the output archive is less than 24 hours old and within its rotation cycle:

- Plex database: `tar czf` the Plex Media Server directory (excluding Cache, Logs, Crash Reports) every 7 days → `ebackup/upload/auto-generated/apps/plex/plex.day-NN.tar.gz`
- Codebase: `rsync --filter=:- .gitignore` the coder-workspaces tree into a cache directory, then `tar czf` → `ebackup/upload/auto-generated/coder-workspaces/coder-workspaces.day-NN.tar.gz` (every 5 days)
- PostgreSQL: `pg_dumpall | gzip` connecting to `postgres.database:5432` → `ebackup/upload/auto-generated/databases/postgres/day-NN.sql.gz` (every 10 days)

**backup-upload** (runs at 08:00 UTC daily) — uses `kubectl exec deploy/rclone` to run `rclone.py backup-latest --offset-days 7` inside the rclone pod. The script copies recently modified files from four monitored source directories to their remote destinations:

| Source directory                  | Remotes                                                                         |
|-----------------------------------|---------------------------------------------------------------------------------|
| `user-data-music/music/warehouse` | `pikpak:backup/p/music/warehouse`, `gdrive:SharePoint/backup/p/music/warehouse` |
| `user-data-others/data`           | `pikpak:backup/p/data`, `gdrive:SharePoint/backup/p/data`                       |
| `user-data-others/backup/upload`  | `pikpak:backup/p/backup`, `gdrive:SharePoint/backup/p/backup`                   |
| `user-data-others/ebackup/upload` | `pikpak-encrypted-backup:` (crypt), `gdrive-encrypted-backup:` (crypt)          |

Files older than 3 days that exist on all remotes at matching size are moved to corresponding `uploaded/` directories to avoid re-upload.

### aliyunpan

A custom Docker image built on Ubuntu 22.04 that installs `aliyunpan v0.3.0` (tickstep/aliyunpan) and `encfs` for filesystem-level encryption. The image bundles helper Python scripts and an `encfs6.xml` config; the encfs password is injected via the `ENCFS_PWD` environment variable at build time.

Two deployment modes exist:

**aliyunpan-decrypt** (Deployment) — mounts two WebDAV paths from the openlist remote (`workspace/e` and `archived-encrypted`) and decrypts them via `encfs` (non-reverse) into `/mnt/decrypted/aliyunpan/workspace` and `/mnt/decrypted/aliyunpan/archived`. This provides a transparent plaintext view of encrypted Aliyun content to other pods via bidirectional mount propagation. Uses pod affinity to rclone. The liveness probe checks `/mnt/decrypted/aliyunpan/workspace/hero/video`.

**aliyunpan-upload** (CronJob, 12:00 UTC daily) — uses `encfs --reverse` to encrypt three source directories (`plain/hero`, `plain/eupload`, `plain/others-eupload`) into `backup-encrypted/` mount points, then runs `upload-latest.py` to upload recently modified files (within 3 days) from multiple monitored paths to Aliyun Drive. Runs on node `nur` with `privileged: true` and `SYS_ADMIN`. The decrypt deployment is active by default; the upload CronJob manifest is present but commented out in `deploy.sh`.

## Storage Layout

All media pods draw from a common set of persistent volumes and hostPath mounts:

| Volume                               | Type            | Contents                                                                   |
|--------------------------------------|-----------------|----------------------------------------------------------------------------|
| `nfs-disk-user-data-music`           | NFS PVC         | `music/warehouse` (local music library), `upload/`                         |
| `nfs-disk-user-data-others`          | NFS PVC         | `data/` (read-only media), `upload/`, `ebackup/`, `backup/`, `workspace/`  |
| `nfs-disk-user-data-slight`          | NFS PVC         | `private-data/hero`, `private-data/hero-pending`, `share-point`, `upload/` |
| `nfs-data-download`                  | NFS PVC         | Download staging area (Samba only)                                         |
| `/mnt/webdav/pikpak-readonly`        | hostPath (FUSE) | PikPak WebDAV tree; subPaths expose pikpak library paths                   |
| `/mnt/remote/pikpak`                 | hostPath (FUSE) | PikPak native rclone remote                                                |
| `/mnt/remote/gdrive`                 | hostPath (FUSE) | Google Drive rclone remote                                                 |
| `/mnt/decrypted/pikpak/backup`       | hostPath (FUSE) | rclone crypt decrypted view of PikPak backup                               |
| `/mnt/decrypted/gdrive/backup`       | hostPath (FUSE) | rclone crypt decrypted view of Google Drive backup                         |
| `/mnt/decrypted/aliyunpan/workspace` | hostPath (FUSE) | encfs decrypted view of Aliyun Drive                                       |
| `/mnt/runtime-data-app/{plex,roon}`  | hostPath        | Application state on `nur` local disk                                      |

## Networking and Security

- All Traefik ingress endpoints use cert-manager (`cluster-letsencrypt-dns-cloudflare`) for automatic TLS.
- Plex, Jellyfin, openlist, bliss, and roon are gated behind the cluster SSO middleware (`sso-domainx@kubernetescrd`) and anti-MITM middleware.
- Samba exposes a `LoadBalancer` service directly onto the LAN rather than routing through Traefik; encryption is negotiated at the SMB level (`smb encrypt = desired`).
- Roon uses `hostNetwork: true` to satisfy its low-latency audio transport requirements, with `ClusterFirstWithHostNet` DNS so cluster service resolution still works.
- Bliss is additionally gated by Sablier (`default-sablierbliss@kubernetescrd`) with a 1-hour session window, keeping it at zero replicas when not in use.
- All backup archives destined for cloud storage are encrypted before upload: rclone crypt (filename + directory name encryption) for pikpak and Google Drive, and encfs reverse encryption for Aliyun Drive.
- Secrets (claim tokens, credentials, encryption passwords) are never stored in manifests; they are rendered at deploy time via the cluster-wide Vaultwarden-backed template system (`__{{…}}__` syntax).

## Related

- [../](../README.md) — Koishi cluster overview
