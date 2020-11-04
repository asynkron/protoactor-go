# Remote SSL Example

In this example we'll use SSL/TLS to authenticate and encrypt exchanges between remote clients and servers using Proto.Actor-Go.

# Requirements

* OpenSSL 1.1.0g+
* GNU Make 4.1+

# Setup

The `remote` package in Proto.Actor-Go utilizes [gRPC][0] under the hood to enable remote connections between nodes, and when creating a server with `remote.Start()` it is possible to pass in several [ServerOption][1] arguments which can be used to pass [TransportCredentials][2] to the [gRPC Server][3].

For this example we'll create an SSL certificate using [OpenSSL][4]. You can either use the local [Makefile](https://www.gnu.org/software/make/manual/html_node/Introduction.html) provided:

```shell
make ssl
```

Or you can do it manually:

```shell
	@openssl req \
		-config cert/localhost.conf \
		-new \
		-newkey rsa:4096 \
		-days 365 \
		-nodes \
		-x509 \
		-subj "/C=US/ST=California/L=SanFrancisco/O=Dis/CN=localhost" \
		-keyout cert/localhost.key \
		-out cert/localhost.crt
```

This will place the files `cert/localhost.key` and `cert/localhost.crt` which both nodes will use to communicate with one another via TLS.

Now you can use the Makefile to compile the nodes:

```
make nodes
```

Or run `go build` manually:

```
go build -o node1 nodes/node1/main.go
go build -o node2 nodes/node2/main.go
```

# Running

For this demo, `node2` will send a message to `node1`, which `node1` will respond to, all over TLS.

You'll want to make sure `node1` is up first:

```shell
./node1
```

And then run `node2` in another terminal:

```shell
./node2
```

If everything is working properly you should see output like the following from `node1`:

```shell
127.0.0.1:8090/node1 received SYN from 127.0.0.1:8091/node2
```

And similarly for `node2`:

```shell
127.0.0.1:8091/node2 received ACK from 127.0.0.1:8090/node1
```

[0]:https://google.golang.org/grpc
[1]:https://godoc.org/google.golang.org/grpc#ServerOption
[2]:https://godoc.org/google.golang.org/grpc/credentials#TransportCredentials
[3]:https://godoc.org/google.golang.org/grpc#Server
[4]:https://www.openssl.org/
