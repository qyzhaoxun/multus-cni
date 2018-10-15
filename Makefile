.PHONY: build docker-build docker push clean

VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS ?= -X main.version=$(VERSION)

# Default to build the Linux binary
build:
	GOOS=linux CGO_ENABLED=0 go build -o ./bin/multus -ldflags "$(LDFLAGS)" ./multus/

docker-build:
	docker run --rm -v $(shell pwd):/go/src/github.com/qyzhaoxun/multus-cni \
		--workdir=/go/src/github.com/qyzhaoxun/multus-cni \
		golang:1.10 make build

docker: docker-build
	@docker build -f scripts/Dockerfile.multus -t "ccr.ccs.tencentyun.com/tke-cni/multus-cni:$(VERSION)" .
	@echo "Built Docker image \"ccr.ccs.tencentyun.com/tke-cni/multus-cni:$(VERSION)\""

push: docker
	docker push "ccr.ccs.tencentyun.com/tke-cni/multus-cni:$(VERSION)"

clean:
	rm -rf bin
