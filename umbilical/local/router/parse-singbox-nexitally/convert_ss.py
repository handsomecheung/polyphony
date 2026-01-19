#!/usr/bin/python3
# -*- coding: utf-8 -*-

import re
import json
import base64
import fileinput
from urllib.parse import unquote


def b64decode(s):
    return str(base64.urlsafe_b64decode(s + "=" * (4 - len(s) % 4)), "utf-8")


def convert_raw(uri):
    prefix = "ss://"
    if not uri.startswith(prefix):
        raise ValueError(f"invalid ssr uri {uri}")

    return uri.replace(prefix, "").strip()


def convert(uri):
    raw = convert_raw(uri)

    # Extract remark (fragment after #)
    if "#" in raw:
        remark_match = re.findall(r"#(.+)$", raw)
        if remark_match:
            remark = unquote(remark_match[0])
            raw = raw.split("#")[0]
        else:
            remark = ""
    else:
        remark = ""

    # Extract host:port (between @ and ? or end of string)
    if "?" in raw:
        # New format with query parameters
        host_match = re.findall(r"@([^?]+)", raw)
    else:
        # Old format without query parameters
        host_match = re.findall(r"@(.+)$", raw)

    if not host_match:
        raise ValueError(f"cannot extract host from uri: {uri}")

    host = host_match[0]
    raw = raw.split("@")[0]

    b64_str = raw
    mpsp = b64decode(b64_str)

    if "@" in mpsp:
        mp, _sp = mpsp.split("@")
    else:
        mp = mpsp

    # Handle both classic SS format (method:password) and SS 2022 format (method:key1:key2)
    parts = mp.split(":")
    if len(parts) == 2:
        # Classic format: method:password
        method, password = parts
    elif len(parts) >= 3:
        # SS 2022 format: method:key1:key2 or more
        # For SS 2022, we combine all keys with : separator
        method = parts[0]
        password = ":".join(parts[1:])
    else:
        raise ValueError(f"invalid SS format: {mp}")

    # Remove trailing slash if present
    host = host.rstrip("/")
    server, port = host.split(":")

    data = {
        "remark": remark + "@" + host,
        "config": {
            "server": server,
            "server_port": int(port),
            "method": method,
            "password": password,
        },
    }

    return data


# echo 'ss://' | python3 convert_ss.py
if __name__ == "__main__":
    for uri in fileinput.input():
        print(json.dumps(convert(uri), ensure_ascii=False))
