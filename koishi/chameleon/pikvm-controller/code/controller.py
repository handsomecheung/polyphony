#! /usr/bin/env python3

import os
import time
import ssl
import json
import threading
import subprocess
import pathlib
from datetime import datetime

from websocket import create_connection

import common


HOST = os.environ.get("PIKVM_HOST")
URI = f"wss://{HOST}/api/ws?stream=0&events=hid"
USERNAME = os.environ.get("PIKVM_USERNAME")
PASSWORD = os.environ.get("PIKVM_PASSWORD")
INTERVAL = 1

page_active = False


screenshot_dir = pathlib.Path(common.SCREENSHOTS_DIR)
screenshot_dir.mkdir(parents=True, exist_ok=True)

always_simulate_file = pathlib.Path(common.ALWAYS_SIMULATE_FILE)


def get_now_readable() -> str:
    return time.strftime("%Y-%m-%d %H:%M:%S")


def print_log(msg: str):
    print(f"[{get_now_readable()}] {msg}")


def create_ws():
    return create_connection(
        URI, header={"X-KVMD-User": USERNAME, "X-KVMD-Passwd": PASSWORD}, sslopt={"cert_reqs": ssl.CERT_NONE}
    )


def activity_loop():
    ws = None

    while True:
        ws = activity_loop_check(ws)


def activity_loop_check(ws):
    try:
        if ws is None:
            ws = create_ws()
            print_log("Activity check WebSocket connected")
        check_activity(ws)
    except Exception as e:
        print_log(f"activity loop check error: {str(e)}")
        check_activity_loop_close(ws)
        ws = None
        time.sleep(30)

    return ws


def check_activity_loop_close(ws):
    try:
        if ws is not None:
            ws.close()
    except Exception as e:
        print_log(f"check activity loop close error: {str(e)}")


def check_activity(ws):
    message = ws.recv()
    data = json.loads(message)

    if "event_type" in data and data["event_type"] == "streamer_state":
        global page_active
        if data["event"]["streamer"]:
            if not page_active:
                page_active = True
                print_log(f"Activity detected at {datetime.now()}")
        else:
            if page_active:
                page_active = False
                print_log(f"Activity ended at {datetime.now()}")


def is_work_time() -> bool:
    now = datetime.now()
    if now.weekday() >= 5:  # 5=Saturday, 6=Sunday
        return False

    work_start = now.replace(hour=9, minute=0, second=0, microsecond=0)
    work_end = now.replace(hour=18, minute=0, second=0, microsecond=0)
    return work_start <= now <= work_end


def is_always_simulate() -> bool:
    if always_simulate_file.exists():
        with open(always_simulate_file, "r", encoding="utf-8") as f:
            value = f.read().strip()
            return value == "1"
    return False


def capture_screenshot():
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    output = screenshot_dir / f"{timestamp}.jpg"

    cmd = ["ssh", "pikvm", "curl --unix-socket /run/kvmd/ustreamer.sock http://localhost/snapshot 2>/dev/null"]

    ssh_process = subprocess.Popen(cmd, stdout=subprocess.PIPE)
    with open(output, "wb") as f:
        f.write(ssh_process.communicate()[0])

    print_log(f"Screenshot saved to {output}")
    return output


def try_simulate():
    try:
        if not is_always_simulate():
            if page_active:
                print_log("Skipping action due to recent activity")
                return

            if not is_work_time():
                print_log("Skipping action: outside of work hours (Mon-Fri 9:00-18:00)")
                return

        simulate()
    except Exception as e:
        print_log(f"error: {str(e)}")


def simulate():
    print_log("Simulate ...")

    ws = create_ws()

    def move_mouse(x, y):
        ws.send(json.dumps({"event_type": "mouse_move", "event": {"to": {"x": x, "y": y}}}))
        print_log(f"Mouse moved to {x}, {y}")
        time.sleep(0.5)

    def press_key(key: str):
        ws.send(json.dumps({"event_type": "key", "event": {"key": key, "state": True}}))
        time.sleep(0.05)
        ws.send(json.dumps({"event_type": "key", "event": {"key": key, "state": False}}))
        time.sleep(0.05)
        print_log(f"Windows key {key} pressed.")

    move_mouse(0, 0)
    move_mouse(0.5, 0.5)
    move_mouse(1.0, 1.0)
    press_key("ShiftRight")
    time.sleep(0.1)
    press_key("ShiftRight")

    ws.close()


def main():
    print_log("Starting pikvm-controller ...")
    activity_thread = threading.Thread(target=activity_loop, daemon=True)
    activity_thread.start()

    while True:
        print_log("Try to simulate ...")
        try_simulate()
        capture_screenshot()
        time.sleep(INTERVAL * 60)


if __name__ == "__main__":
    main()
