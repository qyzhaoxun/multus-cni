#!/usr/bin/env sh

set -e

tag=v0.8.1
root_path=$(dirname $0)/../..
docker build -t ccr.ccs.tencentyun.com/tkeimages/cni-plugins-amd64:${tag} -f ${root_path}/scripts/v0.0.5/Dockerfile.cniplugins .
docker push ccr.ccs.tencentyun.com/tkeimages/cni-plugins-amd64:${tag}
