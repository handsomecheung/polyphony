#!/usr/bin/env python3

import time
import json

import requests
from aliyunsdkcore.client import AcsClient
from aliyunsdkcore.acs_exception.exceptions import ClientException
from aliyunsdkcore.acs_exception.exceptions import ServerException
from aliyunsdkalidns.request.v20150109.DescribeDomainRecordsRequest import DescribeDomainRecordsRequest
from aliyunsdkalidns.request.v20150109.UpdateDomainRecordRequest import UpdateDomainRecordRequest

import config


def print_log(msg):
    print(f'[{time.strftime("%Y-%m-%d %H:%M:%S")}] {msg}')

def get_public_ip():
    # return requests.get("https://4.ipw.cn").text.strip()
    return requests.get("http://members.3322.org/dyndns/getip").text.strip()


def main():
    client = AcsClient(config.ALIYUN_ACCESSKEY_ID, config.ALIYUN_ACCESSKEY_SECRET, "cn-hangzhou")

    while True:
        current_ip = get_public_ip()
        ips = get_dns_ips(client)
        print_log(f"current info: {ips}")

        for rr, info in ips.items():
            record_id, dns_ip = info
            if current_ip != dns_ip:
                print_log(f"set dns ip for {info}")
                res = set_dns_ip(client, rr, record_id, current_ip)
                print_log(f"set dns ip, result: {res}")

        print_log("sleep ...")
        time.sleep(120)


def get_dns_ips(client):
    ips = {}
    for record in get_all_records(client)["DomainRecords"]["Record"]:
        if record["RR"] in config.SUBDOMAINS:
            ips[record["RR"]] = [record["RecordId"], record["Value"]]

    if len(ips) != len(config.SUBDOMAINS):
        raise Exception(f"some records not found by sub domain, ips: {ips}, {config.SUBDOMAINS}")

    return ips


def set_dns_ip(client, rr, record_id, ip):
    request = UpdateDomainRecordRequest()
    request.set_RecordId(record_id)
    request.set_RR(rr)
    request.set_Type("A")
    request.set_Value(ip)
    request.set_TTL(600)

    return json.loads(client.do_action_with_exception(request))


def get_all_records(client):
    request = DescribeDomainRecordsRequest()
    request.set_Type("A")
    request.set_DomainName(config.DOMAIN)

    return json.loads(client.do_action_with_exception(request))


if __name__ == "__main__":
    main()
