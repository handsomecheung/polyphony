#!/usr/bin/env python3

import sys
from pathlib import Path

import common


def main():
    if len(sys.argv) != 2:
        print("invalid arguments")
        sys.exit(1)

    common.aliyunpan_upload(Path(sys.argv[1]))


if __name__ == "__main__":
    main()
