
.PHONY: build docker-build docker push clean

PKG := github.com/qyzhaoxun/multus-cni

BINARY ?= multus
GOARCH ?= amd64

CONTAINER_BUILD_PATH ?= /go/src/$(PKG)
BIN_PATH ?= ./bin/$(GOARCH)/$(BINARY)

REGISTRY ?= ccr.ccs.tencentyun.com/tkeimages
IMG_BINARY ?= multus-cni
IMAGE ?= $(REGISTRY)/$(IMG_BINARY)

VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS ?= -X main.version=$(VERSION)

AGENT_TAG := $(shell cat version/next/CNI_Agent)
AGENT_IMAGE := ccr.ccs.tencentyun.com/tkeimages/tke-cni-agent

# Default to build the Linux binary
build:
	GOOS=linux GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o $(BIN_PATH) -ldflags "$(LDFLAGS)" ./

docker-build:
	docker run --rm -v $(shell pwd):$(CONTAINER_BUILD_PATH) \
		--workdir=$(CONTAINER_BUILD_PATH) \
		golang:1.10 make build GOARCH=$(GOARCH)

docker: docker-build
	$(if $(filter amd64, $(GOARCH)), $(eval BASEIMAGE := amd64/alpine:3.6), $(if $(filter arm64, $(GOARCH)), $(eval BASEIMAGE := arm64v8/alpine:3.6),))
	@docker build --build-arg BASEIMAGE=$(BASEIMAGE) --build-arg GOARCH=$(GOARCH) -f scripts/Dockerfile.multus -t "$(IMAGE):$(GOARCH)-$(VERSION)" .
	@echo "Built Docker image \"$(IMAGE):$(GOARCH)-$(VERSION)\""

push: docker
	docker push "$(IMAGE):$(GOARCH)-$(VERSION)"

docker-arm64:
	GOARCH=arm64 make docker

push-arm64:
	GOARCH=arm64 make push

push-agent:
	hack/next/push.sh amd64

push-agent-arm64:
	hack/next/push.sh arm64

buildx-agent: push-agent push-agent-arm64
	$(eval IMAGE := $(AGENT_IMAGE):$(AGENT_TAG))
	$(eval IMAGES := $(foreach arch, amd64 arm64, $(AGENT_IMAGE):$(arch)-$(AGENT_TAG)))
	@echo "===========> push multi-arch image $(AGENT_IMAGE)"
	@docker buildx imagetools create -t $(IMAGE) $(IMAGES)

clean:
	rm -rf bin
