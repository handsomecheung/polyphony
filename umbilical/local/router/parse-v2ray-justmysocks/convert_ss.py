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

    remark = re.findall(r"#[^@]+", raw)[0]
    raw = raw.replace(remark, "", 1)
    remark = remark.replace("#", "", 1)
    remark = unquote(remark)

    host = re.findall(r"@[^#]+", raw)[0]
    raw = raw.replace(host, "", 1)
    host = host.replace("@", "", 1)

    b64_str = raw
    mpsp = b64decode(b64_str)

    if "@" in mpsp:
        mp, _sp = mpsp.split("@")
    else:
        mp = mpsp
    method, password = mp.split(":")
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
