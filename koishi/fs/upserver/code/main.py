#!/usr/bin/env python3.12

import os

from flask import Flask, request, render_template, make_response, send_file, jsonify
from werkzeug.utils import secure_filename


app = Flask(__name__)


PORT = 80
UPLOAD_DIR = "/files"

SHOW_URL = os.getenv("SHOW_URL", "false").lower() == "true"
ROOT_URL = os.getenv("ROOT_URL", "")

_raw_allowed = os.getenv("UPSERVER_ALLOWED_SUBDIRS", "")
ALLOWED_SUBDIRS = [s.strip().strip("/") for s in _raw_allowed.split(",") if s.strip()] if _raw_allowed.strip() else []

ALLOWED_ROOT = os.getenv("UPSERVER_ALLOWED_ROOT", "true").lower() != "false"

os.makedirs(UPLOAD_DIR, exist_ok=True)


@app.route("/ok", methods=["GET"])
def ok():
    return "OK"


@app.route("/static/<path:filename>")
def server_static(filename):
    return send_file(os.path.join("static", filename))


@app.route("/", methods=["GET"])
def upload():
    return make_response(render_template("upload.html", allowed_subdirs=ALLOWED_SUBDIRS, allowed_root=ALLOWED_ROOT))


@app.route("/", methods=["POST"])
def upload_files():
    if "file" not in request.files:
        return jsonify({"status": "failed", "message": "No file part"}), 400

    subdir = request.args.get("subdir", "").strip("/")
    if subdir and ".." in subdir.split("/"):
        return jsonify({"status": "failed", "message": "Invalid subdir"}), 400

    if not subdir and not ALLOWED_ROOT:
        return jsonify({"status": "failed", "message": "Uploading to root is not allowed"}), 403

    if ALLOWED_SUBDIRS:
        if subdir not in ALLOWED_SUBDIRS:
            return jsonify({"status": "failed", "message": "Subdir not allowed"}), 403

    target_dir = os.path.join(UPLOAD_DIR, subdir) if subdir else UPLOAD_DIR
    os.makedirs(target_dir, exist_ok=True)

    files = request.files.getlist("file")
    saved_files = []

    try:
        for file in files:
            if file.filename:
                filename = secure_filename(file.filename)
                if filename:
                    print(f"Uploading file: {os.path.join(subdir, filename) if subdir else filename}")
                    file.save(os.path.join(target_dir, filename))
                    saved_files.append(filename)

        relative_paths = [os.path.join(subdir, f) if subdir else f for f in saved_files]
        response_data = {"status": "success", "files": relative_paths}
        if SHOW_URL and saved_files:
            base_url = ROOT_URL
            if base_url and not base_url.endswith("/"):
                base_url += "/"

            response_data["urls"] = [f"{base_url}{p}" for p in relative_paths]

        return jsonify(response_data)
    except IOError:
        return jsonify({"status": "failed", "message": "Can't upload files"}), 500


def main():
    app.run(host="0.0.0.0", port=PORT)


if __name__ == "__main__":
    main()
