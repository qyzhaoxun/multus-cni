#!/usr/bin/env sh

docker build -t ccr.ccs.tencentyun.com/tke-cni/tke-cni-agent:v0.0.1 -f Dockerfile .
docker push ccr.ccs.tencentyun.com/tke-cni/tke-cni-agent:v0.0.1