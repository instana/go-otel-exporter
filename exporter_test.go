package instana

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type FakeHttpClient struct {
	err         error
	requestData string
}

func (c *FakeHttpClient) Do(req *http.Request) (*http.Response, error) {
	data, err := io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		c.err = err
		return nil, err
	}

	buf := bytes.Buffer{}
	json.Indent(&buf, data, "", "  ")

	c.requestData = string(buf.Bytes())

	res := &http.Response{
		StatusCode: 200,
	}

	return res, nil
}

func newFakeHttpClient() *FakeHttpClient {
	return &FakeHttpClient{}
}

func initTracer(exporter sdktrace.SpanExporter) *sdktrace.TracerProvider {
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)

	otel.SetTracerProvider(tracerProvider)
	return tracerProvider
}

func TestExporter(t *testing.T) {
	ctx := context.Background()
	httpClient := newFakeHttpClient()

	// In production ready environments, New() should be used
	exporter := &InstanaExporter{
		logger: newLogger(),
		client: httpClient,
	}

	exporter.agentKey = "some hey"
	exporter.endpointUrl = "http://valid.com"
	os.Setenv("INSTANA_LOG_LEVEL", "info")

	tp := initTracer(exporter)

	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			t.Fatalf("shutdown error: %s", err)
		}
	}()

	tracer := otel.Tracer("my-test01")
	_, span := tracer.Start(ctx, "my_span", trace.WithSpanKind(trace.SpanKindServer))
	time.Sleep(time.Millisecond * 400)
	span.End()

	tp.ForceFlush(ctx)

	if exporter.err != nil {
		t.Fatalf("exporter error: %s", exporter.err)
	}

	if httpClient.err != nil {
		t.Fatalf("data upload error: %s", httpClient.err)
	}
}

func TestExporterNoEndpointURL(t *testing.T) {
	ctx := context.Background()
	httpClient := newFakeHttpClient()

	// In production ready environments, New() should be used
	exporter := &InstanaExporter{
		logger: newLogger(),
		client: httpClient,
	}

	exporter.agentKey = "some hey"

	tp := initTracer(exporter)

	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			t.Fatalf("shutdown error: %s", err)
		}
	}()

	tracer := otel.Tracer("my-test02")
	_, span := tracer.Start(ctx, "my_span", trace.WithSpanKind(trace.SpanKindServer))
	time.Sleep(time.Millisecond * 400)
	span.End()

	tp.ForceFlush(ctx)

	if exporter.err == nil {
		t.Fatal("expected exporter to throw an error about missing endpoint")
	}

	if httpClient.err != nil {
		t.Fatalf("data upload error: %s", httpClient.err)
	}
}

func TestExporterNoAgentKey(t *testing.T) {
	ctx := context.Background()
	httpClient := newFakeHttpClient()

	// In production ready environments, New() should be used
	exporter := &InstanaExporter{
		logger: newLogger(),
		client: httpClient,
	}

	exporter.endpointUrl = "http://valid.com"

	tp := initTracer(exporter)

	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			t.Fatalf("shutdown error: %s", err)
		}
	}()

	tracer := otel.Tracer("my-test02")
	_, span := tracer.Start(ctx, "my_span", trace.WithSpanKind(trace.SpanKindServer))
	time.Sleep(time.Millisecond * 400)
	span.End()

	tp.ForceFlush(ctx)

	if exporter.err == nil {
		t.Fatal("expected exporter to throw an error about missing agent key")
	}

	if httpClient.err != nil {
		t.Fatalf("data upload error: %s", httpClient.err)
	}
}
