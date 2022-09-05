package instana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

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
	// Ensures that shutdownCh is closed only once
	shutdownOnce sync.Once
	// A channel used to notify that the exporter was shutdown
	shutdownCh chan struct{}
	// Used to make shared attributes synchronous when they are set
	mu sync.Mutex
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
		shutdownCh:  make(chan struct{}),
	}
}

// handleErrors sets Exporter.err and returns the error itself
func (e *Exporter) handleError(err error) error {
	return err
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
	select {
	case <-ctx.Done():
		err := ctx.Err()
		return err
	case <-e.shutdownCh:
		return nil
	default:
	}

	// Cancel export if Exporter is shutdown.
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	go func(ctx context.Context, cancel context.CancelFunc) {
		select {
		case <-ctx.Done():
		case <-e.shutdownCh:
			cancel()
		}
	}(ctx, cancel)

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
		return e.handleError(err)
	}

	url := strings.TrimSuffix(e.endpointUrl, "/") + "/bundle"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return e.handleError(fmt.Errorf("error setting http request: %w", err))
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "host")

	req.Header.Set("x-instana-key", e.agentKey)
	req.Header.Set("x-instana-host", "host")
	req.Header.Set("x-instana-time", "0")

	resp, err := e.client.Do(req)

	if err != nil {
		return e.handleError(fmt.Errorf("failed to make an HTTP request: %w", err))
	}

	// check backend code and see all http codes returned by it
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		// Request is successful.
		return nil
	}

	return e.handleError(fmt.Errorf("failed to send spans to the backend. error: %s", resp.Status))
}

// Shutdown notifies the exporter of a pending halt to operations. The
// exporter is expected to preform any cleanup or synchronization it
// requires while honoring all timeouts and cancellations contained in
// the passed context.
func (e *Exporter) Shutdown(ctx context.Context) error {
	e.shutdownOnce.Do(func() {
		close(e.shutdownCh)
	})

	select {
	case <-ctx.Done():
		err := ctx.Err()

		return err
	default:
	}

	return nil
}
