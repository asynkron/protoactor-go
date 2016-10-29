protoc -I=. -I=%GOPATH%\src --gogoslick_out=grpc:. messages\protos.proto 
go build
