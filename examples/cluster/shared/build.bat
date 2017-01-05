protoc -I=. -I=%GOPATH%\src --gogoslick_out=. protos.proto 
protoc -I=. -I=%GOPATH%\src --protoactor_out=. protos.proto 
