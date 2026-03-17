#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

pipenv install -r requirements.txt
pipenv run python indexer.py
