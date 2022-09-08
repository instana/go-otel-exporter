package instana

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type FakeHttpClient struct {
	err          error
	requestData  string
	requestCount uint32
}

func (c *FakeHttpClient) Do(req *http.Request) (*http.Response, error) {
	atomic.AddUint32(&c.requestCount, 1)

	data, err := io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		c.err = err
		return nil, err
	}

	buf := bytes.Buffer{}
	if err = json.Indent(&buf, data, "", "  "); err != nil {
		return nil, err
	}

	c.requestData = buf.String()

	res := &http.Response{
		StatusCode: 200,
	}

	return res, nil
}

func newFakeHttpClient() *FakeHttpClient {
	return &FakeHttpClient{}
}

func newTestExporter(httpClient *FakeHttpClient) *Exporter {
	sd := atomic.Value{}
	sd.Store(false)

	exporter := &Exporter{
		client:   httpClient,
		shutdown: sd,
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

func Test_Success(t *testing.T) {
	ctx := context.Background()
	httpClient := newFakeHttpClient()

	exporter := newTestExporter(httpClient)

	exporter.agentKey = "some key"
	exporter.endpointUrl = "http://valid.com"

	tp := initTracer(exporter)

	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			t.Fatalf("shutdown error: %s", err)
		}
	}()

	tracer := otel.Tracer("my-test01")
	_, span := tracer.Start(ctx, "my_span", trace.WithSpanKind(trace.SpanKindServer))
	time.Sleep(time.Millisecond * 50)
	span.End()

	if err := tp.ForceFlush(ctx); err != nil {
		t.Fatalf("exporter error: %s", err)
	}

	if atomic.LoadUint32(&httpClient.requestCount) != 1 {
		t.Fatalf("expected HTTP request count to be 1 but receveived %d", httpClient.requestCount)
	}

	if httpClient.err != nil {
		t.Fatalf("data upload error: %s", httpClient.err)
	}
}

func Test_No_Agent_Key(t *testing.T) {
	os.Setenv("INSTANA_ENDPOINT_URL", "http://example.com")
	defer os.Unsetenv("INSTANA_ENDPOINT_URL")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected exporter to throw an error about missing agent key")
		}
	}()

	_ = New()
}

func Test_No_Endpoint_URL(t *testing.T) {
	os.Setenv("INSTANA_AGENT_KEY", "some_key")
	defer os.Unsetenv("INSTANA_AGENT_KEY")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected exporter to throw an error about missing the endpoint URL")
		}
	}()

	_ = New()
}

func Test_Cancelled_Context(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	httpClient := newFakeHttpClient()
	exporter := newTestExporter(httpClient)

	exporter.agentKey = "some hey"
	exporter.endpointUrl = "http://valid.com"

	tp := initTracer(exporter)

	tracer := otel.Tracer("my-test01")
	_, span := tracer.Start(ctx, "my_span", trace.WithSpanKind(trace.SpanKindServer))
	time.Sleep(time.Millisecond * 50)
	span.End()

	err := tp.ForceFlush(ctx)

	if err == nil {
		t.Fatal("expected shutdown to throw a 'context deadline exceeded' error")
	}

	if atomic.LoadUint32(&httpClient.requestCount) != 0 {
		t.Fatalf("expected HTTP request count to be 0 but receveived %d", httpClient.requestCount)
	}

	if httpClient.err != nil {
		t.Fatalf("data upload error: %s", httpClient.err)
	}
}

// Make sure to run go test with the -race flag to cover this test
func Test_Race_Condition(t *testing.T) {
	ctx := context.Background()
	httpClient := newFakeHttpClient()
	exporter := newTestExporter(httpClient)

	tp := initTracer(exporter)

	tracer := otel.Tracer("my-test01")
	_, span := tracer.Start(ctx, "my_span", trace.WithSpanKind(trace.SpanKindServer))
	time.Sleep(time.Millisecond * 50)
	span.End()

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		_ = exporter.Shutdown(ctx)
		wg.Done()
	}()

	tp.ForceFlush(ctx)

	wg.Wait()
}
