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

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var _ sdktrace.SpanExporter = (*InstanaExporter)(nil)

type instanaHttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type InstanaExporter struct {
	// An interface that implements Do. It's supposed to use http.Client, but the interface is used for tests.
	client instanaHttpClient
	// The serverless acceptor endpoint URL.
	endpointUrl string
	// The agent key.
	agentKey string
	// An error should be set here in case the ExportSpans method returns an error.
	err error
}

func New() *InstanaExporter {
	return &InstanaExporter{
		client:      http.DefaultClient,
		endpointUrl: os.Getenv("INSTANA_ENDPOINT_URL"),
		agentKey:    os.Getenv("INSTANA_AGENT_KEY"),
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
func (e *InstanaExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	// handle lock?
	// handle exporter stopped

	if len(spans) == 0 {
		return nil
	}

	if e.endpointUrl == "" {
		err := errors.New("the endpoint URL cannot be empty")
		e.err = err

		return err
	}

	if e.agentKey == "" {
		err := errors.New("the agent key cannot be empty")
		e.err = err

		return err
	}

	instanaSpans := make([]span, len(spans))

	for idx, sp := range spans {
		instanaSpan := convertSpan(sp, "my_service")
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
		e.err = err
		return fmt.Errorf("error setting http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "host")

	req.Header.Set("x-instana-key", e.agentKey)
	req.Header.Set("x-instana-host", "host")
	req.Header.Set("x-instana-time", "0")

	resp, err := e.client.Do(req)

	if err != nil {
		e.err = err
		return fmt.Errorf("failed to make an HTTP request: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		// Request is successful.
		return nil
	}

	err = fmt.Errorf("failed to send spans to the backend. error: %s", resp.Status)
	e.err = err

	return err
}

// Shutdown notifies the exporter of a pending halt to operations. The
// exporter is expected to preform any cleanup or synchronization it
// requires while honoring all timeouts and cancellations contained in
// the passed context.
func (e InstanaExporter) Shutdown(ctx context.Context) error {
	return nil
}
