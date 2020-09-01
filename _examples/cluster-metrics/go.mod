module cluster-metrics

go 1.13

replace github.com/AsynkronIT/protoactor-go => ../..

replace cluster => ../cluster

require (
	cluster v0.0.0-00010101000000-000000000000 // indirect
	github.com/AsynkronIT/goconsole v0.0.0-20160504192649-bfa12eebf716
	github.com/AsynkronIT/protoactor-go v0.0.0-00010101000000-000000000000
)
