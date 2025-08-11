## AI Generated Content. Please report issues

# Go Example: OpenTelemetry Tracing with Proto.Actor

## Introduction
This example demonstrates how to use Proto.Actor's OpenTelemetry middleware to trace actor interactions. Spans are exported over OTLP to the default gRPC port (4317).

## Running the Example
1. Start Jaeger with OTLP enabled:
   ```bash
   docker run --rm -it -p 16686:16686 -p 4317:4317 \
     -e COLLECTOR_OTLP_ENABLED=true jaegertracing/all-in-one:latest
   ```
2. Run the example:
   ```bash
   go run main.go
   ```
3. View traces at [http://localhost:16686](http://localhost:16686).

## Files
- `main.go` – Sample actor system sending messages through several levels of actors while the middleware creates spans.
- `docker-compose.yaml` – Optional helper to start Jaeger locally.
