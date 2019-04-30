#!/usr/bin/env sh

root_path=$(dirname $0)/..
docker build -t ccr.ccs.tencentyun.com/tkeimages/tke-cni-agent:v0.0.4-eni -f ${root_path}/scripts/Dockerfile.eni .
docker push ccr.ccs.tencentyun.com/tkeimages/tke-cni-agent:v0.0.4-eni
