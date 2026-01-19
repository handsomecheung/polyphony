#!/usr/bin/env python3

import subprocess

import common


class Kubectl:
    def deployment_down(self, deployment):
        try:
            cmd = [
                "kubectl",
                "scale",
                f"deployment/{deployment}",
                "--replicas=0",
                f"-n={common.NAMESPACE}",
            ]
            subprocess.run(cmd, capture_output=True, text=True, check=True)
            common.print_log(f"Successfully scaled {deployment} to 0 replicas")
            return True
        except subprocess.CalledProcessError as e:
            common.print_log(f"Failed to scale {deployment}: {e.stderr}")
            return False
        except Exception as e:
            common.print_log(f"Unexpected error while scaling {deployment}: {str(e)}")
            return False

    def deployment_up(self, deployment, replicas=1):
        try:
            cmd = [
                "kubectl",
                "scale",
                f"deployment/{deployment}",
                f"--replicas={replicas}",
                f"-n={common.NAMESPACE}",
            ]
            subprocess.run(cmd, capture_output=True, text=True, check=True)
            common.print_log(f"Successfully scaled {deployment} to {replicas} replicas")
            return True
        except subprocess.CalledProcessError as e:
            common.print_log(f"Failed to scale {deployment}: {e.stderr}")
            return False
        except Exception as e:
            common.print_log(f"Unexpected error while scaling {deployment}: {str(e)}")
            return False

    def deployment_restart(self, deployment):
        try:
            cmd = [
                "kubectl",
                "rollout",
                "restart",
                "deployment",
                deployment,
                f"-n={common.NAMESPACE}",
            ]
            subprocess.run(cmd, capture_output=True, text=True, check=True)
            return True, f"Successfully restart {deployment}"
        except subprocess.CalledProcessError as e:
            return False, f"Failed to restart {deployment}: {e.stderr}"
        except Exception as e:
            return False, f"Unexpected error while restart {deployment}: {str(e)}"

    def up_mbdeployments(self):
        self.deployment_up(deployment=common.DEPLOYMENT_MBTTY, replicas=1)
        self.deployment_up(deployment=common.DEPLOYMENT_MBVNC, replicas=1)

    def down_mbdeployments(self):
        self.deployment_down(deployment=common.DEPLOYMENT_MBTTY)
        self.deployment_down(deployment=common.DEPLOYMENT_MBVNC)
