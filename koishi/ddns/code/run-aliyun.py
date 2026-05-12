#!/usr/bin/env python3

import json

from aliyunsdkcore.client import AcsClient
from aliyunsdkalidns.request.v20150109.DescribeDomainRecordsRequest import DescribeDomainRecordsRequest
from aliyunsdkalidns.request.v20150109.UpdateDomainRecordRequest import UpdateDomainRecordRequest

import common


DOMAIN = common.get_os_env_force("DOMAIN_U")
SUBDOMAINS = ["*.j"]

ALIYUN_ACCESSKEY_ID = common.get_os_env_force("ACCESS_ID")
ALIYUN_ACCESSKEY_SECRET = common.get_os_env_force("ACCESS_SECRET")


def main():
    client = AcsClient(ALIYUN_ACCESSKEY_ID, ALIYUN_ACCESSKEY_SECRET, "cn-hangzhou")

    current_ip = common.get_current_ipv4()
    if not current_ip:
        common.print_log("IPv4 is invalid or could not be obtained. Skip Aliyun updates.")
        return

    ips = get_dns_ips(client)
    common.print_log(f"current info: {ips}")

    for rr, info in ips.items():
        record_id, dns_ip = info
        if current_ip != dns_ip:
            common.print_log(f"set dns ip for {info}")
            res = set_dns_ip(client, rr, record_id, current_ip)
            common.print_log(f"set dns ip, result: {res}")


def get_dns_ips(client):
    ips = {}
    for record in get_all_records(client)["DomainRecords"]["Record"]:
        if record["RR"] in SUBDOMAINS:
            ips[record["RR"]] = [record["RecordId"], record["Value"]]

    if len(ips) != len(SUBDOMAINS):
        raise Exception(f"some records not found by sub domain, ips: {ips}, {SUBDOMAINS}")

    return ips


def set_dns_ip(client, rr, record_id, ip):
    request = UpdateDomainRecordRequest()
    request.set_TTL(600)
    request.set_Value(ip)
    request.set_Type("A")
    request.set_RR(rr)
    request.set_RecordId(record_id)

    return json.loads(client.do_action_with_exception(request))


def get_all_records(client):
    request = DescribeDomainRecordsRequest()
    request.set_Type("A")
    request.set_DomainName(DOMAIN)

    return json.loads(client.do_action_with_exception(request))


if __name__ == "__main__":
    main()
