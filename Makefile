.PHONY: build docker clean

VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS ?= -X main.version=$(VERSION)

# Default to build the Linux binary
build:
	GOOS=linux CGO_ENABLED=0 go build -o ./bin/multus -ldflags "$(LDFLAGS)" ./multus/

docker:
	docker run --rm -v $(shell pwd):/go/src/github.com/qyzhaoxun/multus-cni \
		--workdir=/go/src/github.com/qyzhaoxun/multus-cni \
		golang:1.10 make build

clean:
	rm -rf bin
