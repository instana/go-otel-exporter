# Instana Go OpenTelemetry Exporter

An [OpenTelemetry exporter](https://opentelemetry.io/docs/js/exporters/) to Instana specific span format.

## Installation

    $ go get github.com/instana/go-otel-exporter

## Instana and OpenTelemetry Versions

Even though the Instana Go SDK supports several versions of Golang, users of the Instana Exporter must take into consideration the
[versions supported by the OpenTelemetry instrumentator](https://github.com/open-telemetry/opentelemetry-go#compatibility).

Making sure that the Go version used in your application fulfills both Instana Exporter and OpenTelemetry SDK versions
is particularly important for Instana customers who wish to migrate from the Instana Collector to OpenTelemetry.

## Usage

The Instana Go OpenTelemetry exporter for serverless works just like any OpenTelemetry exporter.
Once you have your application properly set to be monitored by the OpenTelemetry SDK, the injected tracing module
expects an exporter. In our case, the Instana exporter must be imported and instantiated to be used by the
OpenTelemetry tracing.

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
// You can also use the INSTANA_LOG_LEVEL environment variable to set the log level. Available options are:
// * debug
// * info
// * warn
// * error
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
Assuming that your main application is already importing the tracing module, this is all you have to do.
Your spans will be properly exported from your OpenTelemetry tracer to the Instana backend.
