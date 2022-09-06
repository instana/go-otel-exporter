package instana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var _ sdktrace.SpanExporter = (*Exporter)(nil)

type instanaHttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// The Exporter implements the OTel `sdktrace.SpanExporter` interface and it's responsible for converting otlp spans
// into Instana spans and upload them to the Instana backend.
type Exporter struct {
	// An interface that implements Do. It's supposed to use http.Client, but the interface is used for tests.
	client instanaHttpClient
	// The serverless acceptor endpoint URL.
	endpointUrl string
	// The agent key.
	agentKey string
	// State that controls whether the exporter has been shut down or not.
	shutdown atomic.Value
}

// New returns an instance of instana.Exporter
func New() *Exporter {
	var eurl, akey string
	var ok bool

	if eurl, ok = os.LookupEnv("INSTANA_ENDPOINT_URL"); !ok {
		panic("The environment variable 'INSTANA_ENDPOINT_URL' is not set")
	}

	if akey, ok = os.LookupEnv("INSTANA_AGENT_KEY"); !ok {
		panic("The environment variable 'INSTANA_AGENT_KEY' is not set")
	}

	return &Exporter{
		client:      http.DefaultClient,
		endpointUrl: eurl,
		agentKey:    akey,
	}
}

// ExportSpans exports a batch of spans.
//
// This function is called synchronously, so there is no concurrency
// safety requirement. However, due to the synchronous calling pattern,
// it is critical that all timeouts and cancellations contained in the
// passed context must be honored.
//
// Any retry logic must be contained in this function. The SDK that
// calls this function will not implement any retry logic. All errors
// returned by this function are considered unrecoverable and will be
// reported to a configured error Handler.
func (e *Exporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if isShutdown, ok := e.shutdown.Load().(bool); ok && isShutdown {
		return nil
	}

	select {
	case <-ctx.Done():
		err := ctx.Err()
		return err
	default:
	}

	// Cancel export if Exporter is shutdown.
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	if len(spans) == 0 {
		return nil
	}

	instanaSpans := make([]span, len(spans))

	for idx, sp := range spans {
		serviceName := sp.InstrumentationScope().Name

		if serviceName == "" {
			serviceName = "unknown_service_name"
		}

		instanaSpan := convertSpan(sp, serviceName)
		instanaSpans[idx] = instanaSpan
	}

	bundle := bundle{
		Spans: instanaSpans,
	}

	jsonData, err := json.Marshal(bundle)

	if err != nil {
		return err
	}

	url := strings.TrimSuffix(e.endpointUrl, "/") + "/bundle"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("error setting http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "host")

	req.Header.Set("x-instana-key", e.agentKey)
	req.Header.Set("x-instana-host", "host")
	req.Header.Set("x-instana-time", "0")

	resp, err := e.client.Do(req)

	if err != nil {
		return fmt.Errorf("failed to make an HTTP request: %w", err)
	}

	// check backend code and see all http codes returned by it
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		// Request is successful.
		return nil
	}

	return fmt.Errorf("failed to send spans to the backend. error: %s", resp.Status)
}

// Shutdown notifies the exporter of a pending halt to operations. The
// exporter is expected to preform any cleanup or synchronization it
// requires while honoring all timeouts and cancellations contained in
// the passed context.
func (e *Exporter) Shutdown(ctx context.Context) error {
	e.shutdown.Store(true)

	select {
	case <-ctx.Done():
		err := ctx.Err()

		return err
	default:
	}

	return nil
}
