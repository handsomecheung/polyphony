#!/usr/bin/env python3

import common


def read_liens(filename):
    with open(filename, "r") as f:
        return f.readlines()


def remove_prefix(line1, prefix):
    line = line1.strip()
    if line.startswith(prefix):
        return remove_prefix(line[len(prefix) :], prefix)
    else:
        return line + "\n"


class SSH:
    def __init__(self):
        self.prefix = "#"
        self.authorized_keys_list = [
            "/data/authorized_keys",
        ]

    def disprove_one(self, filename, email):
        lines = read_liens(filename)
        with open(filename, "w") as f:
            for line in lines:
                if email in line:
                    if not line.strip().startswith(self.prefix):
                        line = f"{self.prefix} {line}"
                f.write(line)
        common.print_log(f"SSH key {email} disproved from {filename}")

    def authenticate_one(self, filename, email):
        lines = read_liens(filename)
        with open(filename, "w") as f:
            for line in lines:
                if email in line:
                    if line.strip().startswith(self.prefix):
                        line = remove_prefix(line, self.prefix)
                f.write(line)
        common.print_log(f"SSH key {email} authenticated from {filename}")

    def disprove(self, email):
        for authorized_keys in self.authorized_keys_list:
            self.disprove_one(authorized_keys, email)

    def authenticate(self, email):
        for authorized_keys in self.authorized_keys_list:
            self.authenticate_one(authorized_keys, email)
