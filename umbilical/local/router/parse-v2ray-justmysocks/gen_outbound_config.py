#!/usr/bin/python3
# -*- coding: utf-8 -*-

import os
import glob
import json
import socket
import subprocess

import requests

import convert_ss
import convert_ssr
import convert_vmess

current_dir = os.path.dirname(os.path.abspath(__file__))
v2ray_config_dir = os.path.abspath(os.path.join(current_dir, "../v2ray-client/configs"))
ssr_config_dir = os.path.abspath(os.path.join(current_dir, "../shadowsocks-r/configs"))
v2ray_config_prefix = "06_outbounds_02_tail_"
ssr_config_prefix = "config_dynamic_"
NODE_PREFIX = "node"


def run_shell(cmd):
    process = subprocess.Popen(["bash", "-c", cmd], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    stdout, stderr = process.communicate()
    if process.returncode != 0:
        raise RuntimeError(f"failed to run cmd `{cmd}`, error: `{stderr}`, output: `{stdout}`")

    return str(stdout, "utf-8").strip(), str(stderr, "utf-8").strip()


def parse_subscription(subscription):
    cmd = f"curl '{subscription}' | base64 -d"
    stdout, _ = run_shell(cmd)
    return stdout.split("\n")


def write_config(config, filename):
    content = json.dumps(config, indent=4, ensure_ascii=False)
    with open(filename, "w") as f:
        f.write(content)


def gen_v2ray_config_filename(index, tag):
    return os.path.join(v2ray_config_dir, v2ray_config_prefix + f"{index}_{tag}.json")


def gen_ssr_config_filename(index, tag):
    return os.path.join(ssr_config_dir, ssr_config_prefix + f"{index}_{tag}.json")


def delete_v2ray_configs(index):
    for f in glob.glob(gen_v2ray_config_filename(index, "*")):
        os.remove(f)


def delete_ssr_configs(index):
    for f in glob.glob(gen_ssr_config_filename(index, "*")):
        os.remove(f)


def generate_v2ray(uri, name, index, sub_index):
    raw_config = convert_vmess.convert(uri, name)
    if len(raw_config) == 0:
        return

    country = get_country(raw_config["settings"]["vnext"][0]["address"], raw_config["remark"])
    raw_config["tag"] = f"{NODE_PREFIX}-{country}-{name}-{raw_config['tag']}-{sub_index}"
    config = {"outbounds": [raw_config]}

    write_config(config, gen_v2ray_config_filename(index, raw_config["tag"]))


def generate_v2ray_socks(index, tag, local_port, remark):
    config = {
        "outbounds": [
            {
                "tag": tag,
                "protocol": "socks",
                "settings": {"servers": [{"address": "router", "port": local_port}]},
                "streamSettings": {"sockopt": {"mark": 255}},
            }
        ]
    }
    filename = gen_v2ray_config_filename(index, tag)
    write_config(config, filename)


def generate_ssr(uri, name, index, sub_index):
    raw_config = convert_ssr.convert(uri)
    if len(raw_config) == 0:
        return
    config = raw_config["config"]
    config["local_address"] = "0.0.0.0"
    config["local_port"] = int(f"4{str(index).zfill(2)}{str(sub_index).zfill(2)}")
    tag = f"{name}_{config['server']}_{config['server_port']}"
    filename = gen_ssr_config_filename(index, tag)
    write_config(config, filename)

    generate_v2ray_socks(index, tag, config["local_port"], raw_config["remark"])


def generate_ss(uri, name, index, sub_index):
    raw_config = convert_ss.convert(uri)
    if len(raw_config) == 0:
        return

    sconfig = raw_config["config"]
    country = get_country(sconfig["server"], raw_config["remark"])
    tag = f"{NODE_PREFIX}-{country}-{name}-{sconfig['server']}-{sub_index}"
    config = {
        "outbounds": [
            {
                "tag": tag,
                "protocol": "shadowsocks",
                "settings": {
                    "servers": [
                        {
                            "address": sconfig["server"],
                            "method": sconfig["method"],
                            "password": sconfig["password"],
                            "port": sconfig["server_port"],
                        }
                    ]
                },
                "streamSettings": {"sockopt": {"mark": 255}},
            }
        ]
    }
    write_config(config, gen_v2ray_config_filename(index, tag))


def generate(subscription):
    delete_v2ray_configs(subscription["index"])
    delete_ssr_configs(subscription["index"])
    for sub_index, uri in enumerate(parse_subscription(subscription["url"])):
        if uri == "":
            continue

        if uri.startswith("vmess://"):
            generate_v2ray(uri, subscription["name"], subscription["index"], sub_index)
        elif uri.startswith("ssr://"):
            generate_ssr(uri, subscription["name"], subscription["index"], sub_index)
        elif uri.startswith("ss://"):
            generate_ss(uri, subscription["name"], subscription["index"], sub_index)
        else:
            raise ValueError(f"invalid uri `{uri}`")


def get_country_by_remark(remark):
    country_info = {
        "美国": "US",
        "德国": "DE",
        "英国": "GB",
        "日本": "JP",
        "香港": "HK",
        "台湾": "TW",
        "韩国": "KR",
        "印度": "IN",
        "泰国": "TH",
        "马来西亚": "MY",
        "新加坡": "SG",
    }

    for country, code in country_info.items():
        if country in remark:
            return code

    return None


def get_country(addr, remark):
    if not is_ip(addr):
        addr = lookup_ip(addr)

    country = get_country_by_remark(remark)
    if country is None:
        country = get_country_by_ip(addr)

    return country


def get_country_by_ip(ip):
    r = requests.get(f"http://ip2location.default/info?ip={ip}")
    return r.json()["country_code"]


def lookup_ip(addr):
    return socket.gethostbyname(addr)


def is_ip(addr):
    try:
        socket.inet_aton(addr)
        return True
    except socket.error:
        return False


if __name__ == "__main__":
    subscriptions = [
        {
            "index": "02",
            "name": "justmysocks",
            "url": "",
        },
    ]

    for subscription in subscriptions:
        generate(subscription)
