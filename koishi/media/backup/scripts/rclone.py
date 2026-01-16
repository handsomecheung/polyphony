#!/usr/bin/env python3
# -*- coding: utf-8 -*-

# python -u rclone.py backup-latest --offset-days=7
# python -u rclone.py backup-all --path=/mnt/user-data-others/data
# python -u rclone.py backup-all --path=/mnt/user-data-others/backup/upload
# python -u rclone.py backup-all --path=/mnt/user-data-others/ebackup/upload

import os
import sys
import argparse
import shutil
from pathlib import Path
from datetime import datetime, timedelta

import common


def parse_args():
    parser = argparse.ArgumentParser(description="Rclone sync/copy files")
    subparsers = parser.add_subparsers(dest="action", help="Action to perform")

    backup_latest_parser = subparsers.add_parser("backup-latest", help="Backup recent files")
    backup_latest_parser.add_argument(
        "--offset-days",
        type=int,
        default=common.DEFAULT_OFFSET_DAYS,
        help=f"Number of days to look back (default: {common.DEFAULT_OFFSET_DAYS})",
    )

    backup_all_parser = subparsers.add_parser("backup-all", help="Backup recent files")
    backup_all_parser.add_argument(
        "--path",
        type=str,
        help="path to backup",
    )

    args = parser.parse_args()
    if not args.action:
        parser.print_help()
        exit(1)

    return args


def find_new_dirs(directory, offset_days):
    now = datetime.now()
    dirs = {}
    for f in Path(directory).glob("**/*"):
        if f.is_dir():
            continue

        fmtime = datetime.fromtimestamp(f.stat().st_mtime)
        if now - fmtime > timedelta(days=offset_days):
            continue

        d = f.parent
        if d not in dirs:
            dirs[d] = []
        dirs[d].append(f)

    return dirs


def merge_dir(directory, files):
    all_files = []
    for df in directory.iterdir():
        if df.is_dir():
            # if sub directories exist, not merge
            return files

        all_files.append(df)

    if len(files) / len(all_files) > 0.9:
        return [directory]
    else:
        return files


def get_entries(directory, offset_days):
    dirs = find_new_dirs(directory, offset_days)
    entries = []
    for d, files in dirs.items():
        entries.extend(merge_dir(d, files))

    for entry in entries:
        yield entry


def run_rclone(action, source, mdir, remote, root_path):
    target = Path(str(source).replace(str(mdir), root_path))
    common.print_log(f"{action.__name__}: {source} to {target}")
    return action(remote, source, target)


def copy_rclone(source, mdir, remote, root_path):
    return run_rclone(common.rclone_copy, source, mdir, remote, root_path)


# TODO not used now, but keep it for future use
def sync_rclone(source, mdir, remote, root_path):
    return run_rclone(common.rclone_sync, source, mdir, remote, root_path)


# TODO not used now, but keep it for future use
def check_rclone(source, mdir, remote, root_path):
    return run_rclone(common.rclone_check, source, mdir, remote, root_path)


# TODO not used now, but keep it for future use
def sync_rclone_delete_remote_only(source, mdir, remote, root_path):
    return run_rclone(common.rclone_sync_delete_remote_only, source, mdir, remote, root_path)


def run(func):
    failed_cmds = []

    for [returncode, cmd] in func():
        if returncode != 0:
            if common.EXIT_IMMEDIATELY:
                sys.exit(1)
            else:
                failed_cmds.append(cmd)

    if len(failed_cmds) > 0:
        common.print_log("Some operations failed:")
        for cmd in failed_cmds:
            common.print_log(f"    command: {cmd}")
        sys.exit(1)


def copy_latest_files(offset_days):
    for mdir, info in common.MONITOR_COPY_DIRS.items():
        common.print_log(f"start to copy recent files in dir {mdir}")
        for source in get_entries(mdir, offset_days):
            for remote in info["remotes"]:
                returncode, cmd = copy_rclone(source, mdir, remote["name"], remote["path"])
                yield [returncode, cmd]


def copy_latest(offset_days):
    run(lambda: copy_latest_files(offset_days))


def copy_all_files(mdir):
    info = common.MONITOR_COPY_DIRS[mdir]
    common.print_log(f"start to copy all files in dir {mdir}")
    for remote in info["remotes"]:
        returncode, cmd = copy_rclone(Path(mdir), mdir, remote["name"], remote["path"])
        yield [returncode, cmd]


def copy_all(mdir):
    run(lambda: copy_all_files(mdir))


def get_old_filepaths(dirpath, days):
    old_filepaths = []
    now = datetime.now()
    cutoff_time = now - timedelta(days=days)

    for filepath in Path(dirpath).rglob("*"):
        if filepath.is_file():
            file_mtime = datetime.fromtimestamp(filepath.stat().st_mtime)
            if file_mtime < cutoff_time:
                old_filepaths.append(filepath)

    return old_filepaths


def get_size_local(filepath):
    return Path(filepath).stat().st_size


def get_size_remote(filepath, mdir, remote, remote_root_path):
    target = os.path.join(remote_root_path, os.path.relpath(filepath, mdir))
    returncode, stdout, _ = common.rclone_get("lsf", ["-R", "--format", "tsp"], remote, target)

    if returncode != 0:
        # maybe the file does not exist in remote
        return None

    # stdout format: 2025-08-22 12:07:43;167786095;backup.20250821-215306.portwarden.decrypted.zip.gpg
    parts = stdout.split(";")
    if len(parts) < 3:
        return None

    return int(parts[1])


def move_uploaded_old_file(old_filepath, upload_dir, uploaded_dir):
    relative_path = os.path.relpath(old_filepath, upload_dir)
    target_path = os.path.join(uploaded_dir, relative_path)
    os.makedirs(os.path.dirname(target_path), exist_ok=True)
    shutil.move(old_filepath, target_path)
    common.print_log(f"moved {old_filepath} to {target_path}")


def move_uploaded_old_files(offset_days, mdir, mdir_uploaded):
    info = common.MONITOR_COPY_DIRS[mdir]
    common.print_log(f"start to move old uploaded files in dir {mdir}")

    for old_filepath in get_old_filepaths(mdir, offset_days):
        size_local = get_size_local(old_filepath)
        size_remotes = [
            get_size_remote(old_filepath, mdir, remote["name"], remote["path"]) for remote in info["remotes"]
        ]
        size_remotes_unqie = list(set(size_remotes))
        common.print_log(f"old file {old_filepath} size: local={size_local}, remotes={size_remotes}")
        if len(size_remotes_unqie) == 1 and size_local == size_remotes_unqie[0]:
            common.print_log(
                f"old file {old_filepath} already exists in remote {size_local}, {size_remotes}, move it to uploaded dir {mdir_uploaded}"
            )
            move_uploaded_old_file(old_filepath, mdir, mdir_uploaded)


def move_uploaded_old_files_for_ebackup(offset_days_for_move):
    move_uploaded_old_files(offset_days_for_move, common.BACKUP_DIR_EBACKUP, common.BACKUP_DIR_EBACKUP_UPLOADED)


def move_uploaded_old_files_for_backup(offset_days_for_move):
    move_uploaded_old_files(offset_days_for_move, common.BACKUP_DIR_BACKUP, common.BACKUP_DIR_BACKUP_UPLOADED)


def backup_latest(offset_days):
    offset_days_move = 3
    move_uploaded_old_files_for_backup(offset_days_move)
    move_uploaded_old_files_for_ebackup(offset_days_move)
    copy_latest(offset_days)


def backup_all(path):
    copy_all(path)


def main():
    print("start ...")
    args = parse_args()
    if args.action == "backup-latest":
        backup_latest(args.offset_days)
    elif args.action == "backup-all":
        backup_all(args.path)
    common.print_log("exit")


if __name__ == "__main__":
    main()
