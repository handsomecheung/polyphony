#!/usr/bin/env python3

"""
./decode-name.py 'oeIyXLTa3F65lS1VrX4xiIr7/1fXXaVfsPbXmAj2dR0lV59Vc'
"""

import sys

import common


def main():
    if len(sys.argv) != 2:
        print("invalid arguments")
        sys.exit(1)

    result = common.decode_names(common.COMMON_DIR, [sys.argv[1]])
    print(result)


if __name__ == "__main__":
    main()
