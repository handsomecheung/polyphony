#!/usr/bin/env python3
# -*- coding: utf-8 -*-

# python -u rclone.py backup-latest --offset-days=7
# python -u rclone.py backup-all --path=/mnt/user-data-others/ebackup/upload
# python -u rclone.py backup-all --path=mnt/user-data-others/data
# python -u rclone.py backup-all --path=mnt/user-data-others/backup


import os
import sys
import shutil
from datetime import datetime

import common


def try_ebackup_check(backup_file, backup_day):
    day_of_month = datetime.now().day
    if day_of_month % backup_day != 0:
        return False, f"today({day_of_month}) is not backup day, skip"

    if os.path.exists(backup_file):
        file_mtime = os.path.getmtime(backup_file)
        file_age = int(datetime.now().timestamp() - file_mtime)
        if file_age < 24 * 60 * 60:  # Less than 24 hours
            return (
                False,
                f"backup file {backup_file} already exists and file age ({file_age} seconds) is less than 1 day old, skip",
            )
        else:
            return (
                True,
                f"backup file {backup_file} exists but it's age ({file_age} seconds) is older than 1 day, will recreate",
            )
    else:
        return True, f"backup file {backup_file} does not exist, create it ..."


def try_prepare_plex_for_ebackup():
    common.print_log("try to backup plex data with encryption ...")

    os.makedirs(common.BACKUP_DIR_PREPARE_TARGET_PLEX, exist_ok=True)
    backup_file = f"{common.BACKUP_DIR_PREPARE_TARGET_PLEX}/plex.day-{datetime.now().strftime('%d')}.tar.gz"

    will_backup, message = try_ebackup_check(backup_file, 7)
    common.print_log(message)
    if not will_backup:
        return

    ignore_dirs = [
        "Cache",
        "Logs",
        "Crash Reports",
    ]

    tar_dir = os.path.dirname(common.BACKUP_DIR_PREPARE_SOURCE_PLEX)
    tar_target = os.path.basename(common.BACKUP_DIR_PREPARE_SOURCE_PLEX)
    tar_ignore = [f"--exclude='{tar_target}/{idir}'" for idir in ignore_dirs]

    cmd = ["tar", "czf", backup_file, "-C", tar_dir, *tar_ignore, tar_target]
    result = common.run_shell_realtime(cmd)

    if result != 0:
        common.print_log(f"plex backup failed with return code: {result}")
        sys.exit(1)


def try_prepare_coder_for_ebackup():
    common.print_log("try to backup coder data with encryption ...")

    os.makedirs(common.BACKUP_DIR_PREPARE_TARGET_CODER, exist_ok=True)
    coder_backup = (
        f"{common.BACKUP_DIR_PREPARE_TARGET_CODER}/coder-workspaces.day-{datetime.now().strftime('%d')}.tar.gz"
    )

    will_backup, message = try_ebackup_check(coder_backup, 5)
    common.print_log(message)
    if not will_backup:
        return

    os.makedirs(common.BACKUP_DIR_CACHE, exist_ok=True)
    cache_dir = f"{common.BACKUP_DIR_CACHE}/coder-workspaces"
    if os.path.exists(cache_dir):
        shutil.rmtree(cache_dir)

    rsync_cmd = [
        "rsync",
        "--ignore-existing",
        "--size-only",
        "-rvh",
        "--delete",
        "--mkpath",
        "--links",
        "--filter=:- .gitignore",
        "--exclude=lost+found",
        "--exclude=*.sock",
        "--exclude=public-workspace/cache/",
        "--exclude=public-workspace/cache-global",
        "--exclude=public-workspace/editor/emacs.d/",
        "--exclude=public-workspace/editor/cursor-server/",
        "--exclude=public-workspace/editor/neovim/state/",
        "--exclude=public-workspace/system/bins/bin/android-sdk/",
        "--exclude=public-workspace/system/bins/bin/flutter/",
        "--exclude=public-workspace/system/bins/go/",
        "--exclude=private-workspace/workspace/encrypted-configs/.ssh/authorized_keys",
        "--exclude=private-workspace/workspace/encrypted-configs/.ssh/id_rsa",
        "--exclude=private-workspace/workspace/encrypted-configs/.ssh/id_rsa.pub",
        common.BACKUP_DIR_PREPARE_SOURCE_CODER,
        os.path.dirname(cache_dir),
    ]
    rsync_result = common.run_shell_realtime(rsync_cmd)
    if rsync_result != 0:
        common.print_log(f"rsync failed with return code: {rsync_result}")
        sys.exit(2)

    tar_cmd = ["tar", "czf", coder_backup, "-C", os.path.dirname(cache_dir), os.path.basename(cache_dir)]
    tar_result = common.run_shell_realtime(tar_cmd)
    if tar_result != 0:
        common.print_log(f"tar failed with return code: {tar_result}")
        sys.exit(3)

    shutil.rmtree(cache_dir)


def try_prepare_postgres_for_ebackup():
    common.print_log("try to dump postgres data with encryption ...")

    os.makedirs(common.BACKUP_DIR_PREPARE_TARGET_POSTGRES, exist_ok=True)
    backup_file = f"{common.BACKUP_DIR_PREPARE_TARGET_POSTGRES}/day-{datetime.now().strftime('%d')}.sql.gz"

    will_backup, message = try_ebackup_check(backup_file, 10)
    common.print_log(message)
    if not will_backup:
        return

    keys = [
        "POSTGRES_HOST",
        "POSTGRES_PORT",
        "POSTGRES_USER",
        "POSTGRES_PASSWORD",
    ]
    values = {}
    for key in keys:
        value = os.getenv(key)
        if not value:
            raise ValueError(f"{key} environment variable is required")
        values[key] = value

    host = values["POSTGRES_HOST"]
    port = values["POSTGRES_PORT"]
    username = values["POSTGRES_USER"]
    password = values["POSTGRES_PASSWORD"]
    cmd = [
        "bash",
        "-c",
        f"PGPASSWORD='{password}' pg_dumpall -h {host} -p {port} -U {username} | gzip > {backup_file}",
    ]
    result = common.run_shell_realtime(cmd)
    if result != 0:
        common.print_log(f"postgres backup failed with return code: {result}")
        sys.exit(1)


def main():
    try_prepare_plex_for_ebackup()
    try_prepare_coder_for_ebackup()
    try_prepare_postgres_for_ebackup()


if __name__ == "__main__":
    main()
