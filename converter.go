package instana

import (
	"encoding/hex"
	"os"
	"strconv"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	OTEL_SPAN_TYPE = "otel"

	INSTANA_SPAN_KIND_SERVER   = "server"
	INSTANA_SPAN_KIND_CLIENT   = "client"
	INSTANA_SPAN_KIND_PRODUCER = "producer"
	INSTANA_SPAN_KIND_CONSUMER = "consumer"
	INSTANA_SPAN_KIND_INTERNAL = "internal"

	INSTANA_DATA_SERVICE      = "service"
	INSTANA_DATA_OPERATION    = "operation"
	INSTANA_DATA_TRACE_STATE  = "trace_state"
	INSTANA_DATA_ERROR        = "error"
	INSTANA_DATA_ERROR_DETAIL = "error_detail"
)

// convertTraceId converts a [16]byte trace id into a hex string
func convertTraceId(traceId trace.TraceID) string {
	return hex.EncodeToString(traceId[:])
}

// convertTraceId converts a [8]byte span id into a hex string
func convertSpanId(spanId trace.SpanID) string {
	return hex.EncodeToString(spanId[:])
}

// convertKind converts an int based OTel span into a string based Instana span.
// It returns the span kind as a string and a boolean indicating if it's an entry span
func convertKind(otelKind trace.SpanKind) (string, bool) {
	switch otelKind {
	case trace.SpanKindServer:
		return INSTANA_SPAN_KIND_SERVER, true
	case trace.SpanKindClient:
		return INSTANA_SPAN_KIND_CLIENT, false
	case trace.SpanKindProducer:
		return INSTANA_SPAN_KIND_PRODUCER, false
	case trace.SpanKindConsumer:
		return INSTANA_SPAN_KIND_CONSUMER, true
	case trace.SpanKindInternal:
		return INSTANA_SPAN_KIND_INTERNAL, false
	default:
		return "unknown", false
	}
}

// bundle represents the JSON bundle expected by the Instana Serverless Acceptor
type bundle struct {
	Spans []span `json:"spans,omitempty"`
}

// batchInfo displays information about span batching. Only to be added to spans that represent multiple similar calls
type batchInfo struct {
	Size int `json:"s"`
}

// FromS is about attributes in span.f (also known as "from section").
type FromS struct {
	EntityID string `json:"e"`
	// Serverless agents fields
	Hostless      bool   `json:"hl,omitempty"`
	CloudProvider string `json:"cp,omitempty"`
	// Host agent fields
	HostID string `json:"h,omitempty"`
}

// traceReference is the reference to the closest Instana ancestor span.
// See W3C Trace Context for more details. MUST NOT be added to spans that are not entry spans.
type traceReference struct {
	TraceID  string `json:"t"`
	ParentID string `json:"p,omitempty"`
}

type oTelSpanData struct {
	Kind           string            `json:"kind"`
	HasTraceParent bool              `json:"tp,omitempty"`
	ServiceName    string            `json:"service"`
	Operation      string            `json:"operation"`
	TraceState     string            `json:"trace_state,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
}

type span struct {
	traceReference

	SpanID          string          `json:"s"`
	LongTraceID     string          `json:"lt,omitempty"`
	Timestamp       uint64          `json:"ts"`
	Duration        uint64          `json:"d"`
	Name            string          `json:"n"`
	From            *FromS          `json:"f"`
	Batch           *batchInfo      `json:"b,omitempty"`
	Ec              int             `json:"ec,omitempty"`
	Synthetic       bool            `json:"sy,omitempty"`
	CorrelationType string          `json:"crtp,omitempty"`
	CorrelationID   string          `json:"crid,omitempty"`
	ForeignTrace    bool            `json:"tp,omitempty"`
	Ancestor        *traceReference `json:"ia,omitempty"`
	Data            oTelSpanData    `json:"data,omitempty"`
}

func convertSpan(otelSpan sdktrace.ReadOnlySpan, serviceName string) span {
	traceId := convertTraceId(otelSpan.SpanContext().TraceID())

	instanaSpan := span{
		Name:           OTEL_SPAN_TYPE,
		traceReference: traceReference{},
		Timestamp:      uint64(otelSpan.StartTime().UnixMilli()),
		Duration:       uint64(otelSpan.EndTime().Sub(otelSpan.StartTime()).Milliseconds()),
		Data: oTelSpanData{
			Tags: make(map[string]string),
		},
		From: &FromS{
			EntityID: strconv.Itoa(os.Getpid()),
		},
	}

	instanaSpan.traceReference.TraceID = traceId[16:32]
	instanaSpan.LongTraceID = traceId

	if otelSpan.Parent().SpanID().IsValid() {
		instanaSpan.traceReference.ParentID = convertSpanId(otelSpan.Parent().SpanID())
	}

	instanaSpan.SpanID = convertSpanId(otelSpan.SpanContext().SpanID())

	kind, isEntry := convertKind(otelSpan.SpanKind())
	instanaSpan.Data.Kind = kind

	if otelSpan.Parent().SpanID().IsValid() && isEntry {
		instanaSpan.Data.HasTraceParent = true
	}

	instanaSpan.Data.ServiceName = serviceName

	instanaSpan.Data.Operation = otelSpan.Name()

	if otelSpan.SpanContext().TraceState().Len() > 0 {
		instanaSpan.Data.TraceState = otelSpan.SpanContext().TraceState().String()
	}

	attrs := otelSpan.Attributes()

	for _, attr := range attrs {
		instanaSpan.Data.Tags[string(attr.Key)] = attr.Value.AsString()
	}

	errornous := false
	if otelSpan.Status().Code == codes.Error {
		errornous = true
		instanaSpan.Data.Tags[INSTANA_DATA_ERROR] = otelSpan.Status().Code.String()
		instanaSpan.Data.Tags[INSTANA_DATA_ERROR_DETAIL] = otelSpan.Status().Description
	}

	if errornous {
		instanaSpan.Ec = 1
	}

	return instanaSpan
}
