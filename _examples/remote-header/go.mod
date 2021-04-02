module remoteheader

go 1.13

replace github.com/AsynkronIT/protoactor-go => ../..

replace remote-benchmark => ../remote-benchmark

require (
	github.com/AsynkronIT/goconsole v0.0.0-20160504192649-bfa12eebf716
	github.com/AsynkronIT/protoactor-go v0.0.0-00010101000000-000000000000
	github.com/gogo/protobuf v1.3.1
	remote-benchmark v0.0.0
)
