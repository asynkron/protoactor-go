protoc -I=. -I=$GOPATH/src --gogoslick_out=. protos.proto 
go build