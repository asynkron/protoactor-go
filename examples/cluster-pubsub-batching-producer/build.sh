protoc --go_out=. --go_opt=paths=source_relative \
    --plugin=protoc-gen-go-grain=../../protobuf/protoc-gen-go-grain/protoc-gen-go-grain.sh --go-grain_out=. --go-grain_opt=paths=source_relative \
    -I../../ -I. protos.proto
