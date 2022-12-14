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
	ch := make(chan struct{})
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
