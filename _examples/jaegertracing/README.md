# Jeager Tracing / OpenTracing example

To run the example an instance of Jaeger server is required running locally. The easiest way to run a jaeger server
instance is starting it using the included docker-compose file like this

```bash
docker-compose -f ./examples/jaegertracing/docker-compose.yaml up -d
``` 

And the just run the example:

```bash
go run ./examples/jaegertracing/main.go
```

After the test has run (and also during), traces can found using the Jaeger UI started at http://localhost:16686.
 
