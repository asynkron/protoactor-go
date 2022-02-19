protoc -I=. -I=%GOPATH%\src --gogoslick_out=plugins=grpc:. protos.proto 
protoc -I=. -I=$GOPATH/src --gogoslick_out=\
    Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types,\
    plugins=grpc:. gossip.proto
