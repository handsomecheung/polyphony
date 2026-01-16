#!/usr/bin/env python3

from pathlib import Path
from datetime import datetime, timedelta

import common

MONITOR_DIRS = [
    "/backup/user-data-music/music",
    "/backup/user-data-others/data",
    "/backup/user-data-others/workspace",
    "/backup/user-data-app/data",
    "/backup/runtime-data-app/data",
    "/backup/runtime-data-app/global-workspace/data/nur/bin",
    "/backup/user-data-slight/private-data/upload",
    "/backup-encrypted/hero",
]
MTIME_OFFSET = timedelta(days=3)


def find_new_dirs(directory):
    now = datetime.now()
    dirs = {}
    for f in Path(directory).glob("**/*"):
        if f.is_dir():
            continue

        fmtime = datetime.fromtimestamp(f.stat().st_mtime)
        if now - fmtime > MTIME_OFFSET:
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


def get_upload_entries(directory):
    dirs = find_new_dirs(directory)
    entries = []
    for d, files in dirs.items():
        entries.extend(merge_dir(d, files))

    for entry in entries:
        yield entry


def upload():
    for mdir in MONITOR_DIRS:
        common.print_log(f"start to upload dir {mdir}")
        for entry in get_upload_entries(mdir):
            common.aliyunpan_upload(entry)


def main():
    common.print_log("upload ...")
    upload()
    common.print_log("exit")


if __name__ == "__main__":
    main()
