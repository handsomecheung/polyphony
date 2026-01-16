#!/usr/bin/env python3

import re
import os
import sys
import time
import pathlib
import subprocess

COMMON_DIR = "/mnt/remote/aliyunpan-workspace-decrypted/hero"

DEBUG = False


def print_log(msg):
    t = time.strftime("%Y-%m-%d %H:%M:%S")
    print(f"[upload script] [{t}] {msg}")


def print_debug(msg):
    if DEBUG:
        print_log(msg)


def pad_string(s, length):
    if len(s) > length:
        return s[0 : length - 3] + "..."
    return s.ljust(length)


def run_shell(cmd):
    print_debug(f"run command [{' '.join(cmd)}] ...")
    job = subprocess.run(cmd, capture_output=True, env=os.environ.copy())
    if job.returncode != 0:
        err = job.stderr.decode("utf-8")
        print(f'command "{cmd}" failed, error: "{err}"')
        sys.exit(3)

    return job.stdout.decode("utf-8").strip()


def run_shell_sync(cmd):
    print_debug(f"run command [{' '.join(cmd)}] ...")
    return subprocess.Popen(cmd, env=os.environ.copy()).wait()


def run_encfsctl(action, root_dir, names):
    cmd = [
        "encfsctl",
        action,
        "--extpass=echo ${ENCFS_PWD}",
        str(root_dir),
        "--",
        *[str(name) for name in names],
    ]
    return run_shell(cmd)


def encode_names(root_dir, names):
    return run_encfsctl("encode", root_dir, names)


def decode_names(root_dir, names):
    return run_encfsctl("decode", root_dir, names)


def decode_paths(root_dir, path_dirs, graceful=False):
    result = {}

    if len(path_dirs) == 0:
        return result

    out = decode_names(root_dir, path_dirs)
    plain_dirs = out.split("\n")
    if len(path_dirs) != len(plain_dirs):
        if graceful:
            return result
        else:
            print(f"length of encrypted dirs ({len(path_dirs)}) not match length of plain dirs ({len(plain_dirs)})")
            print("encrypted dirs: ", path_dirs)
            print("plain dirs: ", plain_dirs)
            sys.exit(4)

    for i, path_dir in enumerate(path_dirs):
        result[path_dir.name] = plain_dirs[i]

    return result


def in_encrypted_dir(path):
    if not path.endswith("/"):
        path = f"{path}/"
    return re.search("^/backup-encrypted/[^/]+", path) or "/archived-encrypted/" in path or "/workspace/e/" in path


def aliyunpan_upload_base(source, target):
    cmd = [
        "aliyunpan",
        "upload",
        "--ow",
        "--bs",
        "30720",
        str(source),
        str(target),
    ]
    code = run_shell_sync(cmd)
    print_log(f"command return code: {code}")
    return code


def aliyunpan_upload(entry):
    return aliyunpan_upload_base(entry, entry.parent)


def aliyunpan_list(dir_path):
    cmd = ["aliyunpan", "ls", dir_path]

    items = []
    for line in run_shell(cmd).split("\n"):
        line = line.strip()
        m = re.match(f"^[0-9]+ +([-0-9KMGTB\.]+) +(\d\d\d\d-\d\d-\d\d \d\d:\d\d:\d\d) +(.*)$", line)
        if not m:
            continue

        size = m.group(1)
        time = m.group(2)
        path = m.group(3)
        if path.endswith("/"):
            path = path.rstrip("/")
        items.append(
            {
                "size": size,
                "time": time,
                "path": path,
                "path_decode": "",
            }
        )

    if len(items) == 0:
        print("empty direcotry")
        return

    if in_encrypted_dir(dir_path):
        result = decode_paths(COMMON_DIR, [pathlib.Path(item["path"]) for item in items], True)
        if len(result) > 0:
            for item in items:
                item["path_decode"] = result[item["path"]]

    max_size = max([len(item["size"]) for item in items])
    max_path = max([len(item["path"]) for item in items])
    for item in items:
        print(
            f"{item['time']} {pad_string(item['size'], max_size)} {pad_string(item['path'], max_path)} {item['path_decode']}"
        )
