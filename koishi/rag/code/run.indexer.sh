#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

pipenv install -r requirements.txt

PIPENV_DOTENV_LOCATION=../.env pipenv run python indexer.py
