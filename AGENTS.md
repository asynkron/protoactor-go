# Agent Instructions

## Coding Guidelines

### Always do this
- Lint the code, there is a script for this, "make lint", then fix some of the reported issues
- Maintenance task, scan actor, remote, cluster, testkit for bad variable / parameter naming. use as idiomatic names as possible for Golang.
- Ensure all exported code has proper comments
- Run all tests at the end of each task

## Consul requirement for tests

Some test packages require a local Consul agent. Start Consul in dev mode before running these tests:

```bash
# install Consul if needed
CONSUL_VERSION=1.21.4
curl -sLo /tmp/consul.zip https://releases.hashicorp.com/consul/${CONSUL_VERSION}/consul_${CONSUL_VERSION}_linux_amd64.zip
unzip -o /tmp/consul.zip -d /usr/local/bin

# start Consul in dev mode
consul agent -dev > /tmp/consul.log 2>&1 &

# verify
curl -s localhost:8500/v1/status/leader
```

With Consul running, you can execute the test suites:

```bash
curl -s localhost:8500/v1/status/leader
go test ./cluster/... -count=1
go test ./scheduler -run TestNewTimerScheduler -v
go test ./remote/... -count=1
go test ./persistence/... -count=1
```


