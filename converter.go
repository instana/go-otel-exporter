package instana

import (
	"encoding/hex"
	"fmt"

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

// TODO: refactor me!
func convertTraceId(traceId trace.TraceID) string {
	const byteLength = 16

	bytes := [16]byte(traceId)
	traceBytes := make([]byte, 0)

	for (len(traceBytes) + len(bytes)) < byteLength {
		traceBytes = append(traceBytes, 0)
	}

	for _, byte := range bytes {
		traceBytes = append(traceBytes, byte)
	}

	return hex.EncodeToString(traceBytes)
}

// TODO: refactor me!
func convertSpanId(spanId trace.SpanID) string {
	const byteLength = 8

	bytes := [8]byte(spanId)
	spanBytes := make([]byte, 0)

	for (len(spanBytes) + len(bytes)) < byteLength {
		spanBytes = append(spanBytes, 0)
	}

	for _, byte := range bytes {
		spanBytes = append(spanBytes, byte)
	}

	return hex.EncodeToString(spanBytes)
}

func oTelKindToInstanaKind(otelKind trace.SpanKind) (string, bool) {
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

type Bundle struct {
	Spans []Span `json:"spans,omitempty"`
}

type BatchInfo struct {
	Size int `json:"s"`
}

type FromS struct {
	EntityID string `json:"e"`
	// Serverless agents fields
	Hostless      bool   `json:"hl,omitempty"`
	CloudProvider string `json:"cp,omitempty"`
	// Host agent fields
	HostID string `json:"h,omitempty"`
}

type TraceReference struct {
	TraceID  string `json:"t"`
	ParentID string `json:"p,omitempty"`
}

type OTelSpanData struct {
	Kind           string            `json:"kind"`
	HasTraceParent bool              `json:"tp,omitempty"`
	ServiceName    string            `json:"service"`
	Operation      string            `json:"operation"`
	TraceState     string            `json:"trace_state,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
}

type Span struct {
	TraceReference

	SpanID          string          `json:"s"`
	LongTraceID     string          `json:"lt,omitempty"`
	Timestamp       uint64          `json:"ts"`
	Duration        uint64          `json:"d"`
	Name            string          `json:"n"`
	From            *FromS          `json:"f"`
	Batch           *BatchInfo      `json:"b,omitempty"`
	Ec              int             `json:"ec,omitempty"`
	Synthetic       bool            `json:"sy,omitempty"`
	CorrelationType string          `json:"crtp,omitempty"`
	CorrelationID   string          `json:"crid,omitempty"`
	ForeignTrace    bool            `json:"tp,omitempty"`
	Ancestor        *TraceReference `json:"ia,omitempty"`
	Data            OTelSpanData    `json:"data,omitempty"`
}

func convertSpan(fromS FromS, otelSpan sdktrace.ReadOnlySpan, serviceName string /* attributes pcommon.Map */) (Span, error) {
	traceId := convertTraceId(otelSpan.SpanContext().TraceID())

	instanaSpan := Span{
		Name:           OTEL_SPAN_TYPE,
		TraceReference: TraceReference{},
		Timestamp:      uint64(otelSpan.StartTime().UnixMilli()),
		Duration:       uint64(otelSpan.EndTime().Sub(otelSpan.StartTime()).Milliseconds()),
		Data: OTelSpanData{
			Tags: make(map[string]string),
		},
		From: &fromS,
	}

	if len(traceId) != 32 {
		return Span{}, fmt.Errorf("failed parsing span, length of TraceId should be 32, but got %d", len(traceId))
	}

	instanaSpan.TraceReference.TraceID = traceId[16:32]
	instanaSpan.LongTraceID = traceId

	if otelSpan.Parent().SpanID().IsValid() {
		instanaSpan.TraceReference.ParentID = convertSpanId(otelSpan.Parent().SpanID())
	}

	instanaSpan.SpanID = convertSpanId(otelSpan.SpanContext().SpanID())

	kind, isEntry := oTelKindToInstanaKind(otelSpan.SpanKind())
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

	return instanaSpan, nil
}
