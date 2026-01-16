#!/usr/bin/env python3
import time
from pathlib import Path

import common


def main():
    with open("specific.txt", "r") as f:
        filenames = f.readlines()

    codes = {}
    for filename in filenames:
        filename = filename.strip()
        if filename.startswith("#") or filename == "":
            continue

        code = common.aliyunpan_upload(Path(filename))
        codes[filename] = code
        if code != 0:
            raise Exception(f"failed to upload {filename}")

        common.print_log("sleep 300 seconds to avoid rate limit...")
        time.sleep(300)

    common.print_log("all files uploaded.")
    for filename, code in codes.items():
        print(f"{filename}: {code}")


if __name__ == "__main__":
    main()
