#!/usr/bin/env python3

from pathlib import Path

from flask import Flask, send_file, jsonify

import common

app = Flask(__name__)
screenshot_dir = Path(common.SCREENSHOTS_DIR)
always_simulate_file = Path(common.ALWAYS_SIMULATE_FILE)


def get_latest_screenshot():
    try:
        files = list(screenshot_dir.glob("*.jpg"))
        if not files:
            return None
        return max(files, key=lambda x: x.stat().st_mtime)
    except Exception as e:
        print(f"Error getting latest screenshot: {e}")
        return None


@app.route("/")
def index():
    return send_file("templates/index.html")


@app.route("/latest_screenshot")
def latest_screenshot():
    latest = get_latest_screenshot()
    if latest:
        return send_file(str(latest))
    return jsonify({"error": "No screenshots available"}), 404


@app.route("/get_always_simulate")
def get_always_simulate():
    try:
        if always_simulate_file.exists():
            with open(always_simulate_file, "r", encoding="utf-8") as f:
                value = f.read().strip()
                return jsonify({"value": value})
        return jsonify({"value": "0"})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/set_always_simulate", methods=["POST"])
def set_always_simulate():
    try:
        current_value = "0"
        if always_simulate_file.exists():
            with open(always_simulate_file, "r", encoding="utf-8") as f:
                current_value = f.read().strip()

        new_value = "1" if current_value == "0" else "0"
        with open(always_simulate_file, "w", encoding="utf-8") as f:
            f.write(new_value)
        return jsonify({"status": "success", "value": new_value})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8000)
