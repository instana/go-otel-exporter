package instana

import (
	"os"
	"strconv"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	otelSpanType = "otel"

	spanKindServer   = "server"
	spanKindClient   = "client"
	spanKindProduce  = "producer"
	spanKindConsumer = "consumer"
	spanKindInternal = "internal"

	dataError       = "error"
	dataErrorDetail = "error_detail"
)

// convertKind converts an int based OTel span into a string based Instana span.
// It returns the span kind as a string and a boolean indicating if it's an entry span
func convertKind(otelKind trace.SpanKind) (string, bool) {
	switch otelKind {
	case trace.SpanKindServer:
		return spanKindServer, true
	case trace.SpanKindClient:
		return spanKindClient, false
	case trace.SpanKindProducer:
		return spanKindProduce, false
	case trace.SpanKindConsumer:
		return spanKindConsumer, true
	case trace.SpanKindInternal:
		return spanKindInternal, false
	default:
		return "unknown", false
	}
}

// convertSpan converts an OTel span into an Instana Span of type otel
func convertSpan(otelSpan sdktrace.ReadOnlySpan, serviceName string) span {
	traceId := otelSpan.SpanContext().TraceID().String()

	instanaSpan := span{
		Name:           otelSpanType,
		traceReference: traceReference{},
		Timestamp:      uint64(otelSpan.StartTime().UnixMilli()),
		Duration:       uint64(otelSpan.EndTime().Sub(otelSpan.StartTime()).Milliseconds()),
		Data: oTelSpanData{
			Tags: make(map[string]string),
		},
		From: &fromS{
			EntityID: strconv.Itoa(os.Getpid()),
		},
	}

	instanaSpan.traceReference.TraceID = traceId[16:32]
	instanaSpan.LongTraceID = traceId

	if otelSpan.Parent().SpanID().IsValid() {
		instanaSpan.traceReference.ParentID = otelSpan.Parent().SpanID().String()
	}

	instanaSpan.SpanID = otelSpan.SpanContext().SpanID().String()

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

	if otelSpan.Status().Code == codes.Error {
		instanaSpan.Ec = 1
		instanaSpan.Data.Tags[dataError] = otelSpan.Status().Code.String()
		instanaSpan.Data.Tags[dataErrorDetail] = otelSpan.Status().Description
	}

	return instanaSpan
}
