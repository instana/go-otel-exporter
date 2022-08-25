module github.com/instana/go-otel-exporter/example

go 1.18

require (
	github.com/instana/go-otel-exporter v0.0.0-20220825144414-4d9179215e99
	go.opentelemetry.io/otel v1.9.0
	go.opentelemetry.io/otel/sdk v1.9.0
	go.opentelemetry.io/otel/trace v1.9.0
)

require (
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	golang.org/x/sys v0.0.0-20210423185535-09eb48e85fd7 // indirect
)

replace github.com/instana/go-otel-exporter => ../
