#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
source ../.env
ruby client.example.rb
