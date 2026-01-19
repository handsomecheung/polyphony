#!/usr/bin/env python3

import time
import datetime


NAMESPACE = "chameleon"
DEPLOYMENT_MBTTY = "mbtty"
DEPLOYMENT_MBVNC = "mbvnc"
DEPLOYMENT_HOMEGO = "homego"


def print_log(msg):
    print(f'[{time.strftime("%Y-%m-%d %H:%M:%S")}] {msg}')


def get_now():
    return datetime.datetime.now(datetime.timezone.utc)
