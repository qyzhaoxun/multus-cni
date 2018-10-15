#!/usr/bin/env sh
echo "=====Installing multus ========="
cp /multus /host/opt/cni/bin/
cp /00-multus.conf /host/etc/cni/net.d/

while sleep 3600; do :; done