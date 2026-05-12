#!/usr/bin/env python3

import os
import re
import socket
import time
from urllib.parse import urljoin
import urllib3.util.connection as urllib_conn

import requests


# Force IPv4 only - monkey patch urllib3 to only use IPv4
def _force_ipv4():
    return socket.AF_INET


# Apply the patch globally
urllib_conn.allowed_gai_family = _force_ipv4


IPv4APIs = [
    "https://ifconfig.me/ip",
    "https://api.ipify.org",
    "https://ipinfo.io/ip",
    "https://icanhazip.com",
    "https://ident.me",
]


def get_os_env_force(name):
    value = os.getenv(name)
    if value is None:
        raise EnvironmentError(f"Environment variable {name} is not set")
    return value


COMAPI_PATH_IPV6 = get_os_env_force("COMAPI_PATH_IPV6")
COMAPI_HOST_NUR = get_os_env_force("COMAPI_HOST_NUR")
COMAPI_HOST_NIPPON = get_os_env_force("COMAPI_HOST_NIPPON")
COMAPI_HOST_MINIBA = get_os_env_force("COMAPI_HOST_MINIBA")


IPv6APIs = {
    "nur": urljoin(COMAPI_HOST_NUR, COMAPI_PATH_IPV6),
    "nippon": urljoin(COMAPI_HOST_NIPPON, COMAPI_PATH_IPV6),
    "miniba": urljoin(COMAPI_HOST_MINIBA, COMAPI_PATH_IPV6),
}


def print_log(msg):
    print(f'[{time.strftime("%Y-%m-%d %H:%M:%S")}] {msg}')


def get_valid_ipv4(ip):
    ip = ip.strip()
    if re.match(r"^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$", ip):
        return ip
    else:
        return None

def get_current_ipv4():
    for api in IPv4APIs:
        try:
            r = requests.get(api, timeout=(2, 2))
            ip = r.text
            print_log(f"got IPv4 from {api}: {ip}")
            valid_ip = get_valid_ipv4(ip)
            if valid_ip:
                return valid_ip
        except Exception as e:
            print_log(f"Failed to get IPv4 from {api}: {e}")
    return None


def get_current_ipv6(name="nippon"):
    ip = None
    api = IPv6APIs[name]
    print_log(f"start to get IPv6 from {name} ...")
    try:
        r = requests.get(api, timeout=(2, 2))
        ip = r.text.strip()
        print_log(f"got IPv6 from {name}, {ip}")
    except Exception as e:
        # maybe node is not running
        print_log(f"Failed to get IPv6 from {name}: {e}")

    return ip
