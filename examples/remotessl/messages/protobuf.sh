#!/bin/bash
protoc -I=. -I=$GOPATH/src --gogoslick_out=plugins=grpc:. messages.proto
