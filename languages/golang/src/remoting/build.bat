protoc -I=. -I=%GOPATH%\src --gogoslick_out=plugins=grpc:. protos.proto 
go build
