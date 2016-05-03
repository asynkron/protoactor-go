.PHONY: all

all: build

# Protobuf definitions
PROTO_FILES := $(shell find . -type f -name '*.proto')
# Protobuf Go files
PROTO_GEN_FILES = $(patsubst %.proto, %.pb.go, $(PROTO_FILES))

# Protobuf generator
PROTO_MAKER := protoc --gofast_out=plugins=grpc:.

build: protogen
	go build ./...

protogen: $(PROTO_GEN_FILES)

%.pb.go: %.proto
	cd $(dir $<); $(PROTO_MAKER) --proto_path=. --proto_path=$(GOPATH)/src ./*.proto

clean: protoclean

protoclean:
	rm -rf $(PROTO_GEN_FILES)
