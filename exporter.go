package instana

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var _ sdktrace.SpanExporter = (*InstanaExporter)(nil)

type instanaHttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// The InstanaExporter implements the OTel `sdktrace.SpanExporter` interface and it's responsible for converting otlp spans
// into Instana spans and upload them to the Instana backend.
type InstanaExporter struct {
	// An interface that implements Do. It's supposed to use http.Client, but the interface is used for tests.
	client instanaHttpClient
	// The serverless acceptor endpoint URL.
	endpointUrl string
	// The agent key.
	agentKey string
	// An error should be set here in case the ExportSpans method returns an error.
	err error
	// The logger utility
	logger *Logger
	// Ensures that shutdownCh is closed only once
	shutdownOnce sync.Once
	// A channel used to notify that the exporter was shutdown
	shutdownCh chan struct{}
	// Used to make shared attributes synchronous when they are set
	mu sync.Mutex
}

// New returns an instance of InstanaExporter
func New() *InstanaExporter {
	return &InstanaExporter{
		client:      http.DefaultClient,
		endpointUrl: os.Getenv("INSTANA_ENDPOINT_URL"),
		agentKey:    os.Getenv("INSTANA_AGENT_KEY"),
		logger:      newLogger(),
		shutdownCh:  make(chan struct{}),
	}
}

// handleErrors sets InstanaExporter.err and returns the error itself
func (e *InstanaExporter) handleError(err error) error {
	e.mu.Lock()
	e.err = err
	e.mu.Unlock()

	e.logger.error(err)

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
func (e *InstanaExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.logger.info("ExportSpans called!")

	select {
	case <-ctx.Done():
		err := ctx.Err()
		e.logger.error("Export failed due to context error:", err)
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
			e.logger.info("context was cancelled")
			cancel()
		}
	}(ctx, cancel)

	if len(spans) == 0 {
		e.logger.info("No spans to export")
		return nil
	}

	if e.endpointUrl == "" {
		return e.handleError(errors.New("the endpoint URL cannot be empty"))
	}

	if e.agentKey == "" {
		return e.handleError(errors.New("the agent key cannot be empty"))
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

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		// Request is successful.
		buf := bytes.Buffer{}
		json.Indent(&buf, jsonData, "", "  ")
		e.logger.info(string(buf.Bytes()))
		return nil
	}

	return e.handleError(fmt.Errorf("failed to send spans to the backend. error: %s", resp.Status))
}

// Shutdown notifies the exporter of a pending halt to operations. The
// exporter is expected to preform any cleanup or synchronization it
// requires while honoring all timeouts and cancellations contained in
// the passed context.
func (e *InstanaExporter) Shutdown(ctx context.Context) error {
	e.logger.info("The exporter is shutting down.")

	e.shutdownOnce.Do(func() {
		e.logger.info("Notifying the exporter to stop accepting spans")
		close(e.shutdownCh)
	})

	select {
	case <-ctx.Done():
		err := ctx.Err()
		e.logger.error("Exporter shutting down with error from context:", err)

		return err
	default:
	}

	return nil
}
