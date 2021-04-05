.PHONY: all test

all: build


build: protogen
	go build ./...

# {{{ protobuf

# Protobuf definitions
PROTO_FILES := $(shell find . \( -path "./languages" -o -path "./specification" \) -prune -o -type f -name '*.proto' -print)
# Protobuf Go files
PROTO_GEN_FILES = $(patsubst %.proto, %.pb.go, $(PROTO_FILES))

# Protobuf generator
PROTO_MAKER := protoc --gogoslick_out=Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types,plugins=grpc:.

protogen: $(PROTO_GEN_FILES)

%.pb.go: %.proto
	cd $(dir $<); $(PROTO_MAKER) --proto_path=. --proto_path=$(GOPATH)/src ./*.proto
	# sed -i '' -En -e '/^package [[:alpha:]]+/,$$p' $@

# }}} Protobuf end

# {{{ cleanup
clean: protoclean

protoclean:
	rm -rf $(PROTO_GEN_FILES)
# }}} Cleanup end

# {{{ test

PACKAGES := $(shell go list ./... | grep -v "/examples/")

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
