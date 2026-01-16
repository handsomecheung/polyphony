#!/usr/bin/env python3

"""
./decode-path.py /data/encrypted/global-workspace/oeIyXLTa3F65lS1VrX4xiIr7/1fXXaVfsPbXmAj2dR0lV59Vc
"""

import re
import sys
import pathlib

import common


def main():
    if len(sys.argv) != 2:
        print("invalid arguments")
        sys.exit(1)

    filename = sys.argv[1]
    m = re.match(f"^(/backup-encrypted/[^/]+/?)(.*)$", filename)
    if not m:
        print("invalid path")
        sys.exit(2)

    root_dir = pathlib.Path(m.group(1))
    path_dir = pathlib.Path(m.group(2))
    path_dirs = [path_dir]

    path = pathlib.Path(filename)
    if path.is_dir():
        path_dirs = [path_dir / d.name for d in path.glob("*")]

    decode_paths(root_dir, path_dirs)


def decode_paths(root_dir, path_dirs):
    """
    workaroud for an known issue: https://github.com/vgough/encfs/issues/574
    """
    limit = 99
    tmp_path_dirs = []
    for d in path_dirs:
        tmp_path_dirs.append(d)
        if len(tmp_path_dirs) == limit:
            result = common.decode_paths(root_dir, tmp_path_dirs)
            print_result(result)
            tmp_path_dirs = []

    if len(tmp_path_dirs) > 0:
        result = common.decode_paths(root_dir, tmp_path_dirs)
        print_result(result)


def print_result(result):
    for path, decode_path in result.items():
        print(f"{path} : {decode_path}")

    return result


if __name__ == "__main__":
    main()
