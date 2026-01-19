#!/usr/bin/env python3

import os
import requests

import common


class Coder:
    URL = os.getenv("CODER_URL")
    TOKEN = os.getenv("CODER_TOKEN")

    def __init__(self):
        self.url = self.URL
        self.headers = {"Coder-Session-Token": self.TOKEN, "Accept": "application/json"}

    def request(self, method, path):
        response = getattr(requests, method)(self.url + path, headers=self.headers)

        data = response.json()
        if "message" in data and "detail" in data:
            raise Exception(data)

        return data

    def request_get(self, path):
        return self.request("get", path)

    def request_put(self, path):
        return self.request("put", path)

    def get_users(self):
        return self.request_get("/api/v2/users")

    def suspend_user(self, user_id):
        return self.request_put(f"/api/v2/users/{user_id}/status/suspend")

    def activate_user(self, user_id):
        return self.request_put(f"/api/v2/users/{user_id}/status/activate")

    def get_user(self, email):
        users = [user for user in self.get_users()["users"] if user["email"] == email]
        if len(users) == 0:
            common.print_log(f"Coder User {email} not found")
            return None
        return users[0]

    def activate(self, email):
        user = self.get_user(email)
        if user is None:
            common.print_log(f"Coder User {email} not exist, noting to do")
            return

        if user["status"] == "active":
            common.print_log(f"Coder User {email} already actived")
            return

        self.activate_user(user["id"])
        common.print_log(f"Coder User {email} activated")

    def deactivate(self, email):
        user = self.get_user(email)
        if user is None:
            common.print_log(f"Coder User {email} not exist, noting to do")
            return

        if user["status"] == "suspended":
            common.print_log(f"Coder User {email} already suspended")
            return

        self.suspend_user(user["id"])
        common.print_log(f"Coder User {email} suspended")
