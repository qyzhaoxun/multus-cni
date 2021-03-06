#!/usr/bin/env sh

set -e

root_path=$(dirname $0)/../..
cni_tag=$(cat ${root_path}/version/current/CNI_Plugins)
agent_tag=$(cat ${root_path}/version/current/CNI_Agent)
echo "build cni-agent ${agent_tag} with cni-plugins ${cni_tag}"
docker build --build-arg cni_tag=${cni_tag} -t ccr.ccs.tencentyun.com/tkeimages/tke-cni-agent:${agent_tag} -f ${root_path}/scripts/current/Dockerfile .
docker push ccr.ccs.tencentyun.com/tkeimages/tke-cni-agent:${agent_tag}
