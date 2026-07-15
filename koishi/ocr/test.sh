#!/usr/bin/env bash
set -e


cd "$(dirname "${BASH_SOURCE[0]}")"
curl -f -X POST -F "file=@test.1.pdf" -F "lang=jpn+eng" https://ocr.home4p.example.com/ocr > test.result.json
