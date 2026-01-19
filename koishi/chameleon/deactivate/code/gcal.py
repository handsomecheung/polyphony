#!/usr/bin/env python3

import os
import datetime

from google.oauth2 import service_account
from googleapiclient.discovery import build

import common


class GCal:
    WORK_CALENDAR_ID = os.getenv("GCAL_WORK_CALENDAR_ID")
    OFFICE_LOCATIONS = os.getenv("GCAL_WORK_LOCATIONS").split(";")
    CREDENTIALS_FILE = "/data/gcp-service-account/deactivate-read-calendar.json"

    def __init__(self):
        self.credentials = service_account.Credentials.from_service_account_file(
            self.CREDENTIALS_FILE,
            scopes=[
                "https://www.googleapis.com/auth/calendar.readonly",
            ],
        )
        self.service = build("calendar", "v3", credentials=self.credentials)

    def get_work_events(self):
        now = common.get_now()
        efrom = (now - datetime.timedelta(days=2)).isoformat()
        eto = (now + datetime.timedelta(days=5)).isoformat()

        events_result = (
            self.service.events()
            .list(
                calendarId=self.WORK_CALENDAR_ID,
                timeMin=efrom,
                timeMax=eto,
                singleEvents=True,
                orderBy="startTime",
            )
            .execute()
        )
        return events_result.get("items", [])

    def get_work_events_in_office(self):
        return [
            event
            for event in self.get_work_events()
            if any(location in event.get("location", "") for location in self.OFFICE_LOCATIONS)
        ]
