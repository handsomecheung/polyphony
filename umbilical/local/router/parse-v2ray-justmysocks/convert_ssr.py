#!/usr/bin/python3
# -*- coding: utf-8 -*-

import json
import base64
import fileinput


def b64decode(s):
    return str(base64.urlsafe_b64decode(s + "=" * (4 - len(s) % 4)), "utf-8")


def convert_raw(uri):
    prefix = "ssr://"
    if not uri.startswith(prefix):
        raise ValueError(f"invalid ssr uri {uri}")

    return b64decode(uri.replace(prefix, "").strip())


def convert(uri):
    raw = convert_raw(uri)
    terms_str, params_str = raw.split("/?")
    terms = terms_str.split(":")
    params = {k: b64decode(v) for k, v in [param.split("=") for param in params_str.split("&")]}

    if "剩余流量" in params["remarks"] or "过期时间" in params["remarks"]:
        return {}

    data = {
        "remark": params["remarks"],
        "config": {
            "server": terms[0],
            "server_port": int(terms[1]),
            "method": terms[3],
            "password": b64decode(terms[5]),
            "timeout": 300,
            "obfs": terms[4],
            "obfs_param": params["obfsparam"],
            "protocol": terms[2],
            "protocol_param": params["protoparam"],
        },
    }

    return data


# echo 'ssr://' | python3 convert_ssr.py
if __name__ == "__main__":
    for uri in fileinput.input():
        print(json.dumps(convert(uri), ensure_ascii=False))
