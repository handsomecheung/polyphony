#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

PIPENV_DOTENV_LOCATION=../.env pipenv run python search.py "$1"
