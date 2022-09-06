# Instana Go OpenTelemetry Exporter

An [OpenTelemetry exporter](https://opentelemetry.io/docs/js/exporters/) that converts and sends Instana spans to the backend.

## Installation

    $ go get github.com/instana/go-otel-exporter

## Usage

Once you have your application properly configured to be monitored by the OpenTelemetry SDK, the tracer provider expects an exporter.
Import and instantiate the exporter to be used by the tracer provider.

The code sample below demonstrates how the tracing module could look like with the Instana exporter:

```go
package main

import (
	"context"
	"time"

	instana "github.com/instana/go-otel-exporter"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// This example application demonstrates how to use the Instana OTel Exporter.
// Make sure to provide the required environment variables before run the application:
// * INSTANA_ENDPOINT_URL
// * INSTANA_AGENT_KEY
func main() {
	ch := make(chan bool)
	// Acquire an instance of the Instana OTel Exporter
	exporter := instana.New()

	// Setup and bootstrap the tracer provider
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)

	otel.SetTracerProvider(tracerProvider)

	ctx := context.Background()

	// Instrument something with OTel
	tracer := otel.Tracer("my-traced-tech")
	_, span := tracer.Start(ctx, "my_span", trace.WithSpanKind(trace.SpanKindServer))
	time.Sleep(time.Millisecond * 400)
	span.End()

	<-ch
}
```
That's all you have to do.
Your spans will be properly exported from your OpenTelemetry tracer to the Instana backend.
