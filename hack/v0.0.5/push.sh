#!/usr/bin/env sh

set -e

plugins_tag=v0.8.1
root_path=$(dirname $0)/../..
docker build -t ccr.ccs.tencentyun.com/tkeimages/tke-cni-agent:v0.0.5 -f ${root_path}/scripts/v0.0.5/Dockerfile .
docker push ccr.ccs.tencentyun.com/tkeimages/tke-cni-agent:v0.0.5
