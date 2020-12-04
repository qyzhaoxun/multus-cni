#!/usr/bin/env sh

set -e

root_path=$(dirname $0)/../..
cni_tag=$(cat ${root_path}/version/next/CNI_Plugins)
echo "build cni-plugins ${cni_tag}"
docker build --build-arg cni_tag=${cni_tag} -t ccr.ccs.tencentyun.com/tkeimages/cni-plugins-arm64:${cni_tag} -f ${root_path}/scripts/next/Dockerfile.cnipluginsarm64 .
docker push ccr.ccs.tencentyun.com/tkeimages/cni-plugins-arm64:${cni_tag}
