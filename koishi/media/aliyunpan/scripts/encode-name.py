#!/usr/bin/env python3

"""
./encode-name.py 'video'
"""

import sys

import common


def main():
    if len(sys.argv) != 2:
        print("invalid arguments")
        sys.exit(1)

    result = common.encode_names(common.COMMON_DIR, [sys.argv[1]])
    print(result)


if __name__ == "__main__":
    main()
