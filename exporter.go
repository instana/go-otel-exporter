package instana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	client      instanaHttpClient
	endpointUrl string
	agentKey    string
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
func (e InstanaExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	// handle lock?
	// handle exporter stopped

	if len(spans) == 0 {
		return nil
	}

	iss := make([]Span, len(spans))

	for _, sp := range spans {
		// TODO: proper From
		instanaSpan, err := convertSpan(FromS{}, sp, "my_service")

		if err == nil {
			iss = append(iss, instanaSpan)
		} else {
			// log error, but continue to the next span
		}
	}

	bundle := Bundle{
		Spans: iss,
	}

	jsonData, err := json.MarshalIndent(bundle, "", "  ")

	if err != nil {
		return err
	}

	url := strings.TrimSuffix(e.endpointUrl, "/") + "/bundle"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		fmt.Println("error setting http request", err)
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

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		// Request is successful.
		log.Println(">>> SPAN", string(jsonData))
		return nil
	}

	log.Println("HTTP request failed", resp)
	return nil
}

// Shutdown notifies the exporter of a pending halt to operations. The
// exporter is expected to preform any cleanup or synchronization it
// requires while honoring all timeouts and cancellations contained in
// the passed context.
func (e InstanaExporter) Shutdown(ctx context.Context) error {
	// log.Println("SHUTDOWN WAS CALLED")
	return nil
}
