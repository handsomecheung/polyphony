#!/usr/bin/env bash
set -e

width=$1
if [ -z "$width" ]; then
    width=1920
fi

height=$2
if [ -z "$height" ]; then
    height=1080
fi

echo "Resizing display to ${width}x${height} ..."
xrandr --output default --fb "${width}x${height}" 2>/dev/null
