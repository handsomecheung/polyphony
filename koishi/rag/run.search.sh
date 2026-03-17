#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

pipenv run python search.py "$1"
