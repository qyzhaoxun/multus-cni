#!/usr/bin/env sh
echo "=====Starting installing cni ========="
cp /opt/cni/bin/* /host/opt/cni/bin/

echo "=====Starting install cni conf ==========="
cp /etc/tke-cni-agent-conf/* /host/etc/cni/net.d/

echo "=====Done==========="

while sleep 3600; do :; done
