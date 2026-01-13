#!/bin/sh
set -e


if [ ! -f /aria2/data/aria2.session ]; then
    touch /aria2/data/aria2.session
fi

if [ ! -f /aria2/data/dht.dat ]; then
    touch /aria2/data/dht.dat
fi

aria2c --conf-path=/aria2/conf/aria2.conf
