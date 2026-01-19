#!/usr/bin/env python3

import os

import requests

import common
from kubectl import Kubectl


class HomeGo:
    URL_DELETE_ALL = os.getenv("HOMEGO_URL_DELETE_ALL")

    def __init__(self, kubectl: Kubectl):
        self.kubectl = kubectl

    def destrory_session(self):
        response = requests.delete(self.URL_DELETE_ALL, timeout=5)
        print(f"Destroy session response: {response.status_code}, {response.text}")

        if response.status_code == 200 and response.json()["message"] == "logged out":
            print("Successfully destroyed HomeGo session. restart HomeGo deployment ...")
            self.kubectl.deployment_restart(deployment=common.DEPLOYMENT_HOMEGO)
