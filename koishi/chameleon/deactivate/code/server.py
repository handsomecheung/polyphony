#! /usr/bin/env python3

from flask import Flask, jsonify

from kubectl import Kubectl

app = Flask(__name__)
kubectl = Kubectl()

allowed_deployments = [
    "gotty",
    "novnc",
]


@app.route("/ok", methods=["GET"])
def ok():
    return "ok", 200


@app.route("/kubernetes/deployment/<name>", methods=["PUT"])
def restart(name):
    if name not in allowed_deployments:
        return jsonify({"status": False, "message": "deployment not allowed"}), 200

    status, message = kubectl.deployment_restart(name)
    return jsonify({"status": True, "message": message}), 200


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8000)
