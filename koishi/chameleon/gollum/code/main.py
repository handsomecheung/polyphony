#! /usr/bin/env python3

import os
import time
import subprocess
from typing import cast

INDEX_FILE = "Home.md"
WDIR = cast(str, os.getenv("WIKI_DIR", None))
if WDIR is None:
    raise ValueError("WIKI_DIR is not set")


def gen_line(filename):
    return f"# [{filename}]({filename}.md)"


def get_filenames(subdir):
    files = [
        (f, os.path.getctime(os.path.join(WDIR, subdir, f)))
        for f in os.listdir(os.path.join(WDIR, subdir))
        if f.endswith(".md") and f != INDEX_FILE
    ]
    files.sort(key=lambda x: x[1], reverse=True)
    return [f[0].replace(".md", "") for f in files]


def generate(subdir):
    with open(os.path.join(WDIR, subdir, INDEX_FILE), "w", encoding="utf-8") as f:
        for filename in get_filenames(subdir):
            f.write(gen_line(filename))
            f.write("\n")


def gitcommit():
    os.chdir(WDIR)
    result = subprocess.run(["git", "status", "--porcelain"], capture_output=True, text=True, check=False)
    if not result.stdout.strip():
        print("No changes to commit")
        return

    os.system("git add .")
    os.system("git commit -m 'commited by script'")


def main():
    while True:
        print("start to generate.")
        generate("articles/linux")
        generate("articles/ps1")
        generate("mb")
        gitcommit()
        print("generation done. sleep ...")

        time.sleep(60)


if __name__ == "__main__":
    main()
