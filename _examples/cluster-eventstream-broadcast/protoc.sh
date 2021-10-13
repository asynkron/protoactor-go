# To run these commands 
# 1. Install protoc compiler
#
# (on Windows)
# choco install protoc
#
# (on Ubuntu)
# apt install protobuf-compiler
#
# 2. Install plugin
# 
# go install github.com/gogo/protobuf/protoc-gen-gogoslick@latest
#
# (make sure go installation path is on your PATH)


protoc -I=. --gogoslick_out=. contract.proto
