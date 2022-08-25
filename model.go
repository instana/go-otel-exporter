package instana

// bundle represents the JSON bundle expected by the Instana Serverless Acceptor
type bundle struct {
	Spans []span `json:"spans,omitempty"`
}

// batchInfo displays information about span batching. Only to be added to spans that represent multiple similar calls
type batchInfo struct {
	Size int `json:"s"`
}

// fromS is about attributes in span.f (also known as "from section").
type fromS struct {
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
	From            *fromS          `json:"f"`
	Batch           *batchInfo      `json:"b,omitempty"`
	Ec              int             `json:"ec,omitempty"`
	Synthetic       bool            `json:"sy,omitempty"`
	CorrelationType string          `json:"crtp,omitempty"`
	CorrelationID   string          `json:"crid,omitempty"`
	ForeignTrace    bool            `json:"tp,omitempty"`
	Ancestor        *traceReference `json:"ia,omitempty"`
	Data            oTelSpanData    `json:"data,omitempty"`
}
