package instana

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func initTracer() *sdktrace.TracerProvider {
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(New()),
	)

	otel.SetTracerProvider(tracerProvider)
	return tracerProvider
}

func TestExporter(t *testing.T) {
	endpoint := os.Getenv("INSTANA_ENDPOINT_URL")
	agentKey := os.Getenv("INSTANA_AGENT_KEY")

	t.Setenv("INSTANA_ENDPOINT_URL", endpoint)
	t.Setenv("INSTANA_AGENT_KEY", agentKey)

	ctx := context.Background()

	tp := initTracer()

	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	tracer := otel.Tracer("my-webserver")
	_, span := tracer.Start(ctx, "my_span", trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()

	time.Sleep(time.Millisecond * 400)
}
