all: nodes

clean:
	@rm -f cert/localhost.key
	@rm -f cert/localhost.crt
	@rm -f node1
	@rm -f node2

ssl:
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

nodes:
	go build -o node1 nodes/node1/main.go
	go build -o node2 nodes/node2/main.go

.PHONY: clean ssl nodes
