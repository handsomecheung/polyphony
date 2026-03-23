#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

list=/config/stream-audio.txt
./concat.py "mp3,mp4,mkv" /data/mediamtx/default "${list}"
ffmpeg -re -stream_loop -1 -f concat -safe 0 -i "${list}" -vn -c:a aac -b:a 256k -ar 44100 -async 1 -vsync 1 -fflags +genpts -f rtsp "rtsps://localhost:8322/radio"
