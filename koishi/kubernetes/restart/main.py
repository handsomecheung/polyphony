#!/usr/bin/env python3

import os
import json
import subprocess
from datetime import datetime, timedelta, timezone

MIN_AGE_MINUTES = 5
MAX_BACKOFF_MINUTES = 60
STATE_FILE = os.environ.get("STATE_FILE")
DEFAULT_STATE = {"backoff_minutes": 1, "last_restart_time": None}

CHECK_DEPLOYMENTS = {
    "default": ["plex"],
}

if STATE_FILE is None:
    raise Exception("env STATE_FILE not set")


def run_command(command):
    result = subprocess.run(
        command, shell=True, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, encoding="utf-8"
    )
    return result.stdout.strip()


def get_pod_status(pod):
    if pod["status"].get("initContainerStatuses"):
        for c in pod["status"]["initContainerStatuses"]:
            if c.get("state", {}).get("waiting"):
                return c["state"]["waiting"].get("reason", "Init:Waiting")
            if c.get("state", {}).get("terminated", {}).get("reason") != "Completed":
                return c["state"]["terminated"].get("reason", "Init:Error")

    if not pod["status"].get("containerStatuses"):
        # If no container statuses, pod is likely pending. The reason might be in conditions.
        return pod["status"].get("reason", pod["status"].get("phase", "Unknown"))

    for c in pod["status"]["containerStatuses"]:
        if c.get("state", {}).get("waiting"):
            # This will catch CreateContainerConfigError, ImagePullBackOff, etc.
            return c["state"]["waiting"].get("reason", "Waiting")
        if c.get("state", {}).get("terminated"):
            # This will catch containers that have failed.
            return c["state"]["terminated"].get("reason", "Terminated")
        if not c.get("state", {}).get("running"):
            # If a container is neither waiting, terminated, nor running, the pod is not healthy.
            return pod["status"].get("phase", "NotRunning")

    # If all containers are running, we can consider the pod as Running.
    return "Running"


def get_deployment_info(namespace, deployment_name):
    cmd = f"kubectl -n {namespace} get deployment {deployment_name} -o json"
    deployment_data = json.loads(run_command(cmd))
    match_labels = deployment_data.get("spec", {}).get("selector", {}).get("matchLabels")
    if not match_labels:
        raise Exception(f"No selector labels found for deployment {deployment_name}")

    selector = ",".join([f"{k}={v}" for k, v in match_labels.items()])

    cmd = f"kubectl -n {namespace} get pod -l {selector} -o json"
    pod_data = json.loads(run_command(cmd))
    if not pod_data.get("items"):
        raise Exception(f"No pods found for deployment {deployment_name} with selector {selector}")

    latest_pod = sorted(pod_data["items"], key=lambda p: p["metadata"]["creationTimestamp"], reverse=True)[0]

    return {
        "status": get_pod_status(latest_pod),
        "creation_time": datetime.strptime(
            latest_pod["metadata"]["creationTimestamp"], "%Y-%m-%dT%H:%M:%SZ"
        ).replace(tzinfo=timezone.utc),
    }


def get_deployment_state(namespace, deployment_name):
    key = f"{namespace}/{deployment_name}"
    try:
        with open(STATE_FILE, "r") as f:
            full_state = json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        full_state = {}
    return full_state.get(key, DEFAULT_STATE)


def save_deployment_state(namespace, deployment_name, state=DEFAULT_STATE):
    key = f"{namespace}/{deployment_name}"
    try:
        with open(STATE_FILE, "r") as f:
            full_state = json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        full_state = {}

    full_state[key] = state

    with open(STATE_FILE, "w") as f:
        json.dump(full_state, f, indent=2)


def restart_deployment(namespace, deployment_name):
    print(f"Restarting deployment {deployment_name} in namespace {namespace}...")
    command = f"kubectl -n {namespace} rollout restart deployment/{deployment_name}"
    run_command(command)
    print("Restart command issued.")


def check_deployment(namespace, deployment_name):
    print(f"\n--- Checking {namespace}/{deployment_name} ---")

    pod_info = get_deployment_info(namespace, deployment_name)
    status = pod_info["status"]
    creation_time = pod_info["creation_time"]
    age = datetime.now(timezone.utc) - creation_time

    print(f"Pod status: {status}, Age: {age}")

    if status == "Running":
        print("Pod is running correctly. Resetting backoff state.")
        save_deployment_state(namespace, deployment_name)
        return

    if status == "Pending":
        print("Pod is in Pending state, likely waiting for a node. No action will be taken.")
        save_deployment_state(namespace, deployment_name)
        return

    if age < timedelta(minutes=MIN_AGE_MINUTES):
        print(f"Pod is in a non-running state but is younger than {MIN_AGE_MINUTES} minutes. Waiting.")
        return

    state = get_deployment_state(namespace, deployment_name)
    last_restart_time_str = state.get("last_restart_time")
    backoff_minutes = state.get("backoff_minutes", 1)

    if last_restart_time_str:
        last_restart_time = datetime.fromisoformat(last_restart_time_str)
        if last_restart_time.tzinfo is None:
            # Treat naive datetimes from old state files as UTC.
            last_restart_time = last_restart_time.replace(tzinfo=timezone.utc)
        next_restart_time = last_restart_time + timedelta(minutes=backoff_minutes)
        if datetime.now(timezone.utc) < next_restart_time:
            print(f"In backoff period. Next restart possible after {next_restart_time}.")
            return

    restart_deployment(namespace, deployment_name)

    # Update state for exponential backoff
    new_backoff = min(backoff_minutes * 2, MAX_BACKOFF_MINUTES)
    new_state = {"backoff_minutes": new_backoff, "last_restart_time": datetime.now(timezone.utc).isoformat()}
    save_deployment_state(namespace, deployment_name, new_state)
    print(f"State updated. Next backoff period will be {new_backoff} minutes.")


def main():
    print(f"Starting Deployment Check at {datetime.now(timezone.utc)}")
    for namespace, deployments in CHECK_DEPLOYMENTS.items():
        for deployment_name in deployments:
            check_deployment(namespace, deployment_name)
    print("\nFinished Deployment Check")


if __name__ == "__main__":
    main()