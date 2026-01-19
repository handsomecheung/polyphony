#!/usr/bin/env python3

import os

def get_os_env_force(name):
    value = os.getenv(name)
    if value is None:
        raise EnvironmentError(f"Environment variable {name} is not set")
    return value

DOMAIN = get_os_env_force("DOMAIN")
SUBDOMAINS = [
    "direct",
    "*.direct",
]
ALIYUN_ACCESSKEY_ID = get_os_env_force("ALIYUN_ACCESSKEY_ID")
ALIYUN_ACCESSKEY_SECRET = get_os_env_force("ALIYUN_ACCESSKEY_SECRET")
