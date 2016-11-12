protoc -I=. -I=%GOPATH%\src --gogoslick_out=plugins=grpc:. messages\protos.proto 
go build
