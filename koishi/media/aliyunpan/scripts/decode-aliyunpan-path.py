#!/usr/bin/env python3

"""
./decode-aliyunpan-path.py 'oeIyXLTa3F65lS1VrX4xiIr7/1fXXaVfsPbXmAj2dR0lV59Vc'
"""

import sys

import common


def main():
    if len(sys.argv) != 2:
        print("invalid arguments")
        sys.exit(1)

    common.aliyunpan_list(sys.argv[1])


if __name__ == "__main__":
    main()
