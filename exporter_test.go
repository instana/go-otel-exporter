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

func newTestExporter(httpClient *FakeHttpClient) *Exporter {
	exporter := &Exporter{
		client:     httpClient,
		shutdownCh: make(chan struct{}),
	}

	return exporter
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

	exporter := newTestExporter(httpClient)

	exporter.agentKey = "some hey"
	exporter.endpointUrl = "http://valid.com"

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

	if err := tp.ForceFlush(ctx); err != nil {
		t.Fatalf("exporter error: %s", err)
	}

	if httpClient.err != nil {
		t.Fatalf("data upload error: %s", httpClient.err)
	}
}

func TestExporterNoAgentKey(t *testing.T) {
	os.Setenv("INSTANA_ENDPOINT_URL", "http://example.com")
	defer os.Unsetenv("INSTANA_ENDPOINT_URL")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected exporter to throw an error about missing agent key")
		}
	}()

	_ = New()
}

func TestExporterNoEndpointUrl(t *testing.T) {
	os.Setenv("INSTANA_AGENT_KEY", "some_key")
	defer os.Unsetenv("INSTANA_AGENT_KEY")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected exporter to throw an error about missing the endpoint URL")
		}
	}()

	_ = New()
}

func TestExporterCancelledContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	httpClient := newFakeHttpClient()

	exporter := newTestExporter(httpClient)

	exporter.agentKey = "some hey"
	exporter.endpointUrl = "http://valid.com"

	tp := initTracer(exporter)

	tracer := otel.Tracer("my-test01")
	_, span := tracer.Start(ctx, "my_span", trace.WithSpanKind(trace.SpanKindServer))
	time.Sleep(time.Millisecond * 400)
	span.End()

	err := tp.ForceFlush(ctx)

	if err == nil {
		t.Fatal("expected shutdown to throw a 'context deadline exceeded' error")
	}

	if httpClient.err != nil {
		t.Fatalf("data upload error: %s", httpClient.err)
	}
}
