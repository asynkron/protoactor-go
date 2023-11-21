## AI Generated Content. Please report issues

# Go Example: Jaeger Tracing with Proto.Actor

## Introduction
This Go example demonstrates how to integrate Jaeger tracing with the Proto.Actor framework. It provides a practical demonstration of implementing distributed tracing in a microservices architecture using Jaeger.

## Description
The program initializes a Jaeger tracer and sets up a simple actor system with Proto.Actor. It sends messages between actors and traces these interactions using Jaeger. This example is particularly useful for understanding how distributed tracing works in complex systems where understanding the flow of requests and responses is crucial.

## Setup
Ensure you have the following to run this example:
- Go programming environment
- Proto.Actor for Go
- Jaeger Tracing libraries

Install the necessary libraries using:
```bash
go get -u github.com/asynkron/protoactor-go
go get -u github.com/uber/jaeger-client-go
```

## Running the Example

```bash
go run main.go
```

To run the example an instance of Jaeger server is required running locally. The easiest way to run a jaeger server
instance is starting it using the included docker-compose file like this

```bash
docker-compose -f ./examples/jaegertracing/docker-compose.yaml up -d
``` 


## Additional Notes
- The `initJaeger` function sets up Jaeger tracing with basic configuration.
- The program creates an actor system and uses Jaeger to trace the message flow between actors.
- This example is ideal for those looking to implement distributed tracing in their Go applications using Jaeger and Proto.Actor.

This example is an insightful demonstration of using Jaeger for distributed tracing in a Go application with the Proto.Actor framework.
