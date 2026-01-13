#!/usr/bin/env python3

import os

from flask import Flask, request, render_template, make_response, send_file


app = Flask(__name__)


PORT = 80
UPLOAD_DIR = "/files"

os.makedirs(UPLOAD_DIR, exist_ok=True)


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

    try:
        for file in files:
            if file.filename:
                print(f"Uploading file: {file.filename}")
                file.save(os.path.join(UPLOAD_DIR, file.filename))
        return "Success\n"
    except IOError:
        return "Failed: Can't upload files\n", 500


def main():
    app.run(host="0.0.0.0", port=PORT)


if __name__ == "__main__":
    main()
