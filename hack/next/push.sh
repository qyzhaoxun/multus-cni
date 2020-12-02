#!/usr/bin/env sh

set -e

arch=$1
if [[ $arch == "" ]]; then
    arch=amd64
fi
BASEIMAGE=amd64/alpine:3.6
if [[ $arch == "arm64" ]]; then
    BASEIMAGE=arm64v8/alpine:3.6
fi

root_path=$(dirname $0)/../..
cni_tag=$(cat ${root_path}/version/next/CNI_Plugins)
agent_tag=$(cat ${root_path}/version/next/CNI_Agent)
image=ccr.ccs.tencentyun.com/tkeimages/tke-cni-agent
echo "build cni-agent ${arch}-${agent_tag} with cni-plugins ${cni_tag}"
docker build --build-arg BASEIMAGE=${BASEIMAGE} --build-arg arch=${arch} --build-arg cni_tag=${cni_tag} -t ${image}:${arch}-${agent_tag} -f ${root_path}/scripts/next/Dockerfile .
docker push ccr.ccs.tencentyun.com/tkeimages/tke-cni-agent:${arch}-${agent_tag}
