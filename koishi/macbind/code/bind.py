#! /usr/bin/env python3

import os
import re
import json
import requests

import config


HOST = os.environ.get("NTT_ROUTER")
USERNAME = os.environ.get("NTT_ROUTER_USERNAME")
PASSWORD = os.environ.get("NTT_ROUTER_PASSWORD")


PATH_PAGE_INDEX = "/index.cgi/adm/network/dhcp_assign"
PATH_PAGE_ADD = "/index.cgi/adm/network/dhcp_assign_add"


class Client:
    def __init__(self, host, username, password):
        self.host = host
        self.session = requests.Session()
        self.session.auth = (username, password)

    def get_session_id(self, path, args):
        url = f"{self.host}{path}"
        if args:
            url = f"{url}?{args}"

        response = self.session.get(url, timeout=10)
        response.raise_for_status()
        html_content = response.text
        match = re.search(r"id='SESSION_ID' value='([a-f0-9]+)'", html_content)
        if match:
            sid = match.group(1)
        else:
            raise Exception("Error: SESSION_ID input not found")

        if sid is None:
            raise Exception("Error: SESSION_ID is None")

        return sid

    def request_ntt_control(self, path, method, params, session_args=None):
        sid = self.get_session_id(path, session_args)

        url = f"{self.host}{path}/control"
        payload = {
            "method": method,
            "sid": sid,
            "config": "",
            "section": "",
            "params": params,
        }
        response = self.session.post(url, json=payload, timeout=10)
        response.raise_for_status()
        return response.json()

    def get_current_entries(self):
        res = self.request_ntt_control(PATH_PAGE_INDEX, "get", {})
        entries = res["params"]
        print(f"current entries: {[entry for entry in entries if 'MAC' in entry]}")

        return entries

    def delete_entry(self, entry):
        print(f"delete entry: {entry}")
        res = self.request_ntt_control(PATH_PAGE_INDEX, "del", {"ENTRY": entry["ENTRY"]})
        print(f"result of delete: {res}")

        return res

    def enable_entries(self, entries):
        print(f"enable entries: {entries}")
        res = self.request_ntt_control(PATH_PAGE_INDEX, "set", entries)
        print(f"result of enable: {res}")

        return res

    def add_entry(self, number, entry):
        print(f"adding entry: {entry} to {number}")
        res = self.request_ntt_control(
            PATH_PAGE_ADD, "set", {"ENTRY": number, "MAC": entry["MAC"], "IP": entry["IP"]}, f"entry={number}"
        )
        print(f"Result of add: {res}")

        return res


def is_local_mac(mac):
    first_octet = int(mac.split(":")[0], 16)
    return (first_octet & 0x02) == 2  # 00000010


def check_config_duplicated_ip(ips):
    dic = {}
    for ip in ips:
        if ip in dic:
            raise Exception(f"duplicated ip {ip} found")
        dic[ip] = True


def check_config_local_mac(macs):
    for mac in macs:
        if is_local_mac(mac):
            raise Exception(f"MAC address {mac} is a locally administered address.")


def get_config_entries():
    check_config_local_mac(config.MAC_LIST.keys())

    entries = []
    for mac, info in config.MAC_LIST.items():
        ip = info.get("ip")
        if ip is not None:
            entries.append({"MAC": mac, "IP": ip})

    check_config_duplicated_ip(map(lambda e: e["IP"], entries))
    return entries


def get_needto_delete(current_entries, config_entries):
    to_delete = []
    config_macs = {entry["MAC"].upper(): entry["IP"] for entry in config_entries}
    for entry in current_entries:
        if "MAC" not in entry:
            continue

        mac = entry["MAC"].upper()
        ip = entry["IP"]
        if mac not in config_macs or ip != config_macs[mac]:
            to_delete.append(entry)

    print(f"entries to delete: {json.dumps(to_delete, indent=2)}")
    return to_delete


def get_needto_add(current_entries, config_entries):
    to_add = []
    current_macs = {entry["MAC"].upper() for entry in current_entries if "MAC" in entry}
    for entry in config_entries:
        if entry["MAC"].upper() not in current_macs:
            to_add.append(entry)

    print(f"entries to add: {json.dumps(to_add, indent=2)}")
    return to_add


def try_delete(client, config_entries):
    current_entries = client.get_current_entries()

    to_delete_entries = get_needto_delete(current_entries, config_entries)

    for entry in to_delete_entries:
        client.delete_entry(entry)


def try_add(client, config_entries):
    current_entries = client.get_current_entries()
    to_add_entries = get_needto_add(current_entries, config_entries)
    cursor = 0
    for entry in current_entries:
        if "MAC" in entry:
            continue

        if cursor >= len(to_add_entries):
            break

        client.add_entry(entry["ENTRY"], to_add_entries[cursor])
        cursor = cursor + 1


def try_enable_all(client):
    current_entries = client.get_current_entries()

    entries = []
    for entry in current_entries:
        if "MAC" not in entry:
            continue
        entries.append({"ENTRY": entry["ENTRY"], "ENABLED": "1", "IP": entry["IP"]})

    client.enable_entries({"table": entries})


def main():
    client = Client(HOST, USERNAME, PASSWORD)
    config_entries = get_config_entries()

    try_delete(client, config_entries)
    try_add(client, config_entries)
    try_enable_all(client)


if __name__ == "__main__":
    main()
