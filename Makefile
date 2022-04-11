.PHONY: all test

all: build

proto:
	@./buildall.sh

build:
	@go build ./...

# {{{ test

PACKAGES := $(shell go list ./... | grep -v "/_examples/" | grep -v "/etcd" | grep -v "/k8s" | grep -v "/persistence" | grep -v "/cluster" | grep -v "/scheduler")

test:
	@go test $(PACKAGES) -timeout=30s

test-short:
	@go test $(PACKAGES) -timeout=30s -short

test-race:
	@go test $(PACKAGES) -timeout=30s -race

# }}} test

# {{{ benchmark

packages_benchmark := $(shell go list ./... | grep -v "/log")

benchmark:
	go test -benchmem -run=^$ $(packages_benchmark) -bench ^Benchmark$(t).*$
# }}}

# {{{ docker-env
root_dir := $(abspath $(CURDIR)/)
docker-env:
	sudo docker run -it --rm \
		-v $(root_dir)/:/go/src/AsncronIT/protoactor-go \
		-w /go/src/AsncronIT/protoactor-go \
		-e GOPATH=/go \
		--entrypoint /bin/bash \
		cupen/protoc:3.9.1-1
# }}}
