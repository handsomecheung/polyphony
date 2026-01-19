#!/usr/bin/python3
# -*- coding: utf-8 -*-

import json
import base64
import fileinput


def b64decode(s):
    return str(base64.urlsafe_b64decode(s + "=" * (4 - len(s) % 4)), "utf-8")


def convert_raw(uri):
    prefix = "vmess://"
    if not uri.startswith(prefix):
        raise ValueError(f"invalid vmess uri {uri}")

    return json.loads(b64decode(uri.replace(prefix, "")))


def assert_values(v1, v2):
    if v1 != v2:
        raise ValueError(f"value [{v1}] should equals value [{v2}]")


def check_raw_config(raw):
    assert_values(raw.get("headerType", "none"), "none")
    assert_values(raw["v"], "2")
    assert_values(raw["type"], "none")
    assert_values(raw.get("class", 1), 1)

    allow_keys = [
        "host",
        "add",
        "ps",
        "remark",
        "headerType",
        "v",
        "type",
        "class",
        "path",
        "tls",
        "verify_cert",
        "port",
        "aid",
        "net",
        "id",
        "node_area",
    ]
    for key in raw.keys():
        if key not in allow_keys:
            raise ValueError(f"invalid key {key}")


def convert(uri, name):
    raw = convert_raw(uri)
    if "剩余流量" in raw.get("ps", "") or "过期时间" in raw.get("ps", ""):
        return {}

    if (
        "剩余流量" in raw.get("remark", "")
        or "过期时间" in raw.get("remark", "")
        or "最新域名" in raw.get("remark", "")
    ):
        return {}

    # check_raw_config(raw)
    if name == "justmysocks":
        raw["add"], _ = raw["ps"].split("@")[-1].split(":")
        raw["host"] = raw["add"]

    config = {
        "tag": f"{raw['add']}",
        "remark": raw["ps"],
        "protocol": "vmess",
        "settings": {
            "vnext": [
                {
                    "address": raw["add"],
                    "port": int(raw["port"]),
                    "users": [{"id": raw["id"], "alterId": int(raw["aid"])}],
                }
            ]
        },
        "streamSettings": {"sockopt": {"mark": 255}},
        "mux": {"enabled": True},
    }

    if name != "justmysocks" and name != "popocloud-qiqi":
        streamSettings = {
            "network": raw["net"],
            "security": raw.get("tls", ""),
            "wsSettings": {
                "headers": {"Host": raw["host"]},
                "path": raw["path"],
            },
        }

        config["streamSettings"] = {**config["streamSettings"], **streamSettings}
        config["streamSettings"]["tlsSettings"] = {"allowInsecure": not raw.get("verify_cert", True)}

    return config


# echo 'vmess://' | python3 convert_vmess.py
if __name__ == "__main__":
    for uri in fileinput.input():
        print(json.dumps(convert(uri, "justmysocks"), ensure_ascii=False))
