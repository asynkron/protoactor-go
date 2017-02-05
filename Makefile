.PHONY: all test

PACKAGES_TO_BUILD=$(shell go list ./... | grep -v "/vendor/")
PACKAGES_TO_TEST := $(shell go list ./... | grep -v "/examples/"| grep -v "/vendor/")
PROJECT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

all: build


build: protogen
	go build ${PACKAGES_TO_BUILD}

# {{{ Protobuf

# Protobuf definitions
PROTO_FILES := $(shell find . \( -path "./languages" -o -path "./specification" -o -path "./vendor" \) -prune -o -type f -name '*.proto' -print)
# Protobuf Go files
PROTO_GEN_FILES = $(patsubst %.proto, %.pb.go, $(PROTO_FILES))

# Protobuf generator
PROTO_MAKER := protoc --gogoslick_out=Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types,plugins=grpc:.

protogen: $(PROTO_GEN_FILES)

%.pb.go: %.proto
	cd $(dir $<); $(PROTO_MAKER) --proto_path=. --proto_path=$(PROJECT_DIR)/vendor --proto_path=$(GOPATH)/src ./*.proto

# }}} Protobuf end


# {{{ Cleanup
clean: protoclean

protoclean:
	rm -rf $(PROTO_GEN_FILES)
# }}} Cleanup end

# {{{ test

test:
	go test $(PACKAGES_TO_TEST)

test-short:
	go test -short $(PACKAGES_TO_TEST)

# }}} test
