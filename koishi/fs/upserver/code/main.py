#!/usr/bin/env python3.12

import os

from flask import Flask, request, render_template, make_response, send_file


app = Flask(__name__)


PORT = 80
UPLOAD_DIR = "/files"

SHOW_URL = os.getenv("SHOW_URL", "false").lower() == "true"
ROOT_URL = os.getenv("ROOT_URL", "")

os.makedirs(UPLOAD_DIR, exist_ok=True)


@app.route("/ok", methods=["GET"])
def ok():
    return "OK"


@app.route("/static/<path:filename>")
def server_static(filename):
    return send_file(os.path.join("static", filename))


@app.route("/", methods=["GET"])
def upload():
    return make_response(render_template("upload.html"))


@app.route("/", methods=["POST"])
def upload_files():
    if "file" not in request.files:
        return "Failed: No file part", 400

    files = request.files.getlist("file")
    saved_files = []

    try:
        for file in files:
            if file.filename:
                print(f"Uploading file: {file.filename}")
                file.save(os.path.join(UPLOAD_DIR, file.filename))
                saved_files.append(file.filename)

        response_text = "Success<br>\n"
        if SHOW_URL and saved_files:
            base_url = ROOT_URL
            if base_url and not base_url.endswith("/"):
                base_url += "/"
            
            links = [f'<a href="{base_url}{filename}" target="_blank">{base_url}{filename}</a>' for filename in saved_files]
            response_text += "<br>\n<br>\n".join(links) + "<br>\n"

        return response_text
    except IOError:
        return "Failed: Can't upload files\n", 500


def main():
    app.run(host="0.0.0.0", port=PORT)


if __name__ == "__main__":
    main()
