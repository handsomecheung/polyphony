#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

list=/config/stream-videos.txt
./concat.py "mp4,mkv" /data/mediamtx/default "${list}"
ffmpeg -re -stream_loop -1 -f concat -safe 0 -i "${list}" -c:v libx264 -preset fast -crf 22 -g 120 -keyint_min 120 -c:a aac -b:a 256k -ar 44100 -async 1 -vsync 1 -fflags +genpts -f rtsp "rtsps://localhost:8322/tv"
