#!/usr/bin/env sh

docker build -t ccr.ccs.tencentyun.com/tkeimages/tke-cni-agent:v0.0.2-eni -f Dockerfile.eni .
docker push ccr.ccs.tencentyun.com/tkeimages/tke-cni-agent:v0.0.2-eni