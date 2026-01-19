#!/usr/bin/env python3

import os
import datetime

import common
from gcal import GCal
from kubectl import Kubectl
from homego import HomeGo
from coder import Coder
from ssh import SSH


EMAIL_SSH = os.getenv("EMAIL_SSH")
EMAIL_CODER = os.getenv("EMAIL_CODER")


def is_or_will_work():
    events = GCal().get_work_events_in_office()

    if not events:
        common.print_log("No work events found.")
        return False

    now = common.get_now()
    start_offset = datetime.timedelta(minutes=30)
    end_offset = datetime.timedelta(minutes=30)
    common.print_log(f"now: {now}, start_offset: {start_offset}, end_offset: {end_offset}")

    for event in events:
        start_time = datetime.datetime.fromisoformat(event["start"]["dateTime"])
        end_time = datetime.datetime.fromisoformat(event["end"]["dateTime"])

        common.print_log(f"event: {event['summary']}, start: {start_time}, end: {end_time}")

        if start_time - start_offset <= now <= end_time + end_offset:
            common.print_log(
                f"hit work event. summary: {event['summary']}, start: {event['start']['dateTime']}, end: {event['end']['dateTime']}"
            )
            return True

    common.print_log("No work events are happening now.")
    return False


def main():
    coder = Coder()
    ssh = SSH()
    kubectl = Kubectl()
    homego = HomeGo(kubectl)

    if is_or_will_work():
        kubectl.up_mbdeployments()
        coder.activate(EMAIL_CODER)
        # ssh.authenticate(EMAIL_SSH)
    else:
        kubectl.down_mbdeployments()
        homego.destrory_session()
        coder.deactivate(EMAIL_CODER)
        ssh.disprove(EMAIL_SSH)


if __name__ == "__main__":
    main()
