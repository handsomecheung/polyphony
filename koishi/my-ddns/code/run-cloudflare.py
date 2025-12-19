#!/usr/bin/env python3
# https://github.com/cloudflare/python-cloudflare

import CloudFlare

import common

DOMAIN_X = common.get_os_env_force("DOMAIN_X")
DOMAIN_Y = common.get_os_env_force("DOMAIN_Y")
DOMAIN_P = common.get_os_env_force("DOMAIN_P")
DOMAIN_T = common.get_os_env_force("DOMAIN_T")
DOMAIN_C = common.get_os_env_force("DOMAIN_C")

DEFAULT_TTL = 60
DOMAINS_IPV4 = {
    DOMAIN_X: [
        ["", True],
        ["*.", True],
        ["home.", False],
        ["*.home.", False],
        ["nur.home.", False],
        ["*.nur.home.", False],
    ],
    DOMAIN_Y: [
        ["", True],
        ["*.", True],
    ],
    DOMAIN_T: [
        ["", True],
        ["*.", True],
    ],
    DOMAIN_P: [
        ["", True],
        ["*.", True],
    ],
    DOMAIN_C: [
        ["", True],
        ["*.", True],
    ],
}

DOMAINS_IPV6 = {
    # DOMAIN_X: {
    #     "nippon": [
    #         "home6.",
    #         "nippon.home6.",
    #     ],
    #     "nur": [
    #         "nur.home6.",
    #     ],
    #     "miniba": [
    #         "miniba.home6.",
    #     ],
    # },
}


def set_ip(client, zone, domain, rtype, ip, proxied=True):
    common.print_log(f"set ip, domain: {domain}, rtype: {rtype}, ip: {ip}")

    zone_id = zone["id"]
    records = [
        record
        for record in client.zones.dns_records.get(zone_id)
        if record["name"] == domain and record["type"] == rtype
    ]

    if len(records) == 0:
        common.print_log("add record due to empty records")
        add_record(client, zone_id, domain, rtype, ip, proxied)
    else:
        matched = False
        for record in records:
            if record["content"] != ip:
                common.print_log(f"delete record. {record['name']}, {record['type']}, {record['content']}")
                delete_record(client, zone_id, record["id"])
            else:
                common.print_log(f"record match. {record['name']}, {record['content']}")
                matched = True
        if not matched:
            common.print_log("add record due to unmatched")
            add_record(client, zone_id, domain, rtype, ip, proxied)


def add_record(client, zone_id, domain, rtype, ip, proxied):
    config = {"type": rtype, "name": domain, "content": ip, "ttl": DEFAULT_TTL, "proxied": proxied}
    client.zones.dns_records.post(zone_id, data=config)


def delete_record(client, zone_id, record_id):
    client.zones.dns_records.delete(zone_id, record_id)


def main():
    ipv4 = common.get_current_ipv4()
    client = CloudFlare.CloudFlare()
    zones = client.zones.get()

    for root_domain, sub_domain_infos in DOMAINS_IPV4.items():
        common.print_log(f"set ip, root domain: {root_domain}, ipv4: {ipv4}")
        zone = [zone for zone in zones if zone["name"] == root_domain][0]
        for sub_domain_info in sub_domain_infos:
            sub_domain, proxied = sub_domain_info
            domain = sub_domain + root_domain
            set_ip(client, zone, domain, "A", ipv4, proxied)

    for root_domain, sub_domain_map in DOMAINS_IPV6.items():
        zone = [zone for zone in zones if zone["name"] == root_domain][0]
        for node, sub_domains in sub_domain_map.items():
            ipv6 = common.get_current_ipv6(node)
            if ipv6 is None:
                common.print_log(f"maybe node `{node}` is not running. skip.")
                continue

            for sub_domain in sub_domains:
                domain = sub_domain + root_domain
                common.print_log(f"set ip, root domain: {root_domain}, ipv6: {ipv6}, domain: {domain}")
                set_ip(client, zone, domain, "AAAA", ipv6)


if __name__ == "__main__":
    main()
