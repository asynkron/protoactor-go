#!/bin/bash
protoc --go_out=. --go_opt=paths=source_relative -I. protos.proto
