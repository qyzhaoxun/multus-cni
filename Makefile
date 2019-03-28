
.PHONY: build docker-build docker push clean

PKG := github.com/qyzhaoxun/multus-cni

BINARY ?= multus

CONTAINER_BUILD_PATH ?= /go/src/$(PKG)
BIN_PATH ?= ./bin/$(BINARY)

REGISTRY ?= ccr.ccs.tencentyun.com/tkeimages
IMG_BINARY ?= multus-cni
IMAGE ?= $(REGISTRY)/$(IMG_BINARY)

VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS ?= -X main.version=$(VERSION)

# Default to build the Linux binary
build:
	GOOS=linux CGO_ENABLED=0 go build -o $(BIN_PATH) -ldflags "$(LDFLAGS)" ./

docker-build:
	docker run --rm -v $(shell pwd):$(CONTAINER_BUILD_PATH) \
		--workdir=$(CONTAINER_BUILD_PATH) \
		golang:1.10 make build

docker: docker-build
	@docker build -f scripts/Dockerfile.multus -t "$(IMAGE):$(VERSION)" .
	@echo "Built Docker image \"$(IMAGE):$(VERSION)\""

push: docker
	docker push "$(IMAGE):$(VERSION)"

clean:
	rm -rf bin
