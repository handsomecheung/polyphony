#!/usr/bin/env python3

import os
import time
import subprocess

DEBUG = True

EXIT_IMMEDIATELY = False

BACKUP_DIR_CACHE = "/mnt/backup-cache"

BACKUP_DIR_BACKUP = "/mnt/user-data-others/backup/upload"
BACKUP_DIR_BACKUP_UPLOADED = "/mnt/user-data-others/backup/uploaded"

BACKUP_DIR_EBACKUP = "/mnt/user-data-others/ebackup/upload"
BACKUP_DIR_EBACKUP_UPLOADED = "/mnt/user-data-others/ebackup/uploaded"

BACKUP_DIR_PREPARE_SOURCE_PLEX = "/mnt/backup-source/plex"
BACKUP_DIR_PREPARE_TARGET_PLEX = f"{BACKUP_DIR_EBACKUP}/auto-generated/apps/plex"

BACKUP_DIR_PREPARE_SOURCE_CODER = "/mnt/backup-source/coder-workspaces"
BACKUP_DIR_PREPARE_TARGET_CODER = f"{BACKUP_DIR_EBACKUP}/auto-generated/coder-workspaces"

BACKUP_DIR_PREPARE_TARGET_POSTGRES = f"{BACKUP_DIR_EBACKUP}/auto-generated/databases/postgres"

MONITOR_COPY_DIRS = {
    "/mnt/user-data-music/music/warehouse": {
        "remotes": [
            {"name": "pikpak", "path": "backup/p/music/warehouse"},
        ],
    },
    "/mnt/user-data-others/data": {
        "remotes": [
            {"name": "pikpak", "path": "backup/p/data"},
            {"name": "gdrive", "path": "SharePoint/backup/p/data"},
        ],
    },
    BACKUP_DIR_BACKUP: {
        "remotes": [
            {"name": "pikpak", "path": "backup/p/backup"},
            {"name": "gdrive", "path": "SharePoint/backup/p/backup"},
        ],
    },
    BACKUP_DIR_EBACKUP: {
        "remotes": [
            {"name": "pikpak-encrypted-backup", "path": ""},
            {"name": "gdrive-encrypted-backup", "path": ""},
        ],
    },
}

DEFAULT_OFFSET_DAYS = 7


RCLONE_ARGS_COMMON = [
    "-v",
    "--stats",
    "30s",
    "--stats-one-line",
    "--stats-one-line-date",
]


def print_log(msg):
    t = time.strftime("%Y-%m-%d %H:%M:%S")
    print(f"[{t}] {msg}")


def print_debug(msg):
    if DEBUG:
        print_log(msg)


def run_shell_realtime(cmd):
    print_debug(f"run command realtime [{' '.join(cmd)}] ...")
    process = subprocess.Popen(
        cmd, env=os.environ.copy(), stdout=subprocess.PIPE, stderr=subprocess.STDOUT, universal_newlines=True
    )

    for line in process.stdout:
        print(line.rstrip())

    process.wait()
    return process.returncode


def run_shell_output(cmd):
    print_debug(f"run command output [{' '.join(cmd)}] ...")

    process = subprocess.Popen(
        cmd, env=os.environ.copy(), stdout=subprocess.PIPE, stderr=subprocess.PIPE, universal_newlines=True
    )
    stdout, stderr = process.communicate()

    return process.returncode, stdout.strip(), stderr.strip()


def rclone_get(action, args, remote, target):
    cmd = ["rclone", action] + args + [f"{remote}:{target}"]
    return run_shell_output(cmd)


def rclone_run(action, args, remote, source, target):
    cmd = ["rclone", action] + args + RCLONE_ARGS_COMMON + [str(source), f"{remote}:{target}"]
    code = run_shell_realtime(cmd)
    print_log(f"command return code: {code}")
    return code, " ".join(cmd)


def rclone_sync_delete_remote_only(remote, source, target):
    return rclone_run("sync", ["--delete-before", "--max-transfer=1B"], remote, source, target)


def rclone_sync(remote, source, target):
    return rclone_run("sync", ["--delete-before"], remote, source, target)


def rclone_copy_file(remote, source, target):
    return rclone_run("copyto", [], remote, source, target)


def rclone_copy_dir(remote, source, target):
    return rclone_run("copy", [], remote, source, target)


def rclone_copy(remote, source, target):
    if source.is_dir():
        return rclone_copy_dir(remote, source, target)
    else:
        return rclone_copy_file(remote, source, target)


def rclone_check(remote, source, target):
    return rclone_run("check", [], remote, source, target)
