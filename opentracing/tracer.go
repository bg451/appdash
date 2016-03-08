package opentracing

import (
	basictracer "github.com/opentracing/basictracer-go"
	opentracing "github.com/opentracing/opentracing-go"
	"sourcegraph.com/sourcegraph/appdash"
)

var _ opentracing.Tracer = NewTracer(nil) // Compile time check.

type Options struct {
	// ShouldSample is a function that allows deterministic sampling of a trace
	// using the randomly generated Trace ID. The decision is made when a new Trace
	// is created and is propagated to all of the trace's spans. For example,
	//
	//   func(traceID int64) { return traceID % 128 == 0 }
	//
	// samples 1 in every 128 traces, approximately.
	ShouldSample func(traceID int64) bool
}

func DefaultOptions() Options {
	return Options{
		ShouldSample: func(_ int64) bool { return true },
	}
}

// NewTracer creates a new opentracing.Tracer implementation that reports
// spans to an Appdash collector. NewTracer reports all spans. If you want to
// sample 1 in every n spans, see NewTracerWithOptions.
// Spans are written to the underlying collector when Finish() is called on the
// span. It is possible to buffer and write span on a time interval using
// appdash.ChunkedCollector. For example,
//
//   collector := appdash.NewLocalCollector(myAppdashStore)
//   chunkedCollector := appdash.ChunkedCollector{
//     Collector: collector,
//     MinInterval: 1 * time.Minute,
//   }
//
//   tracer := NewTracer(chunkedCollector)
//
// If writing traces a remote Appdash collector, an appdash.RemoteCollector would
// be needed, for example:
//
//   collector := appdash.NewRemoteCollector("localhost:8700")
//   tracer := NewTracer(collector)
//
// will record all spans to a collector server on localhost:8700.
func NewTracer(c appdash.Collector) opentracing.Tracer {
	return NewTracerWithOptions(c, DefaultOptions())
}

// NewTracerWithOptions creates a new opentracing.Tracer that records spans to
// the given appdash.Collector.
func NewTracerWithOptions(c appdash.Collector, options Options) opentracing.Tracer {
	opts := basictracer.DefaultOptions()
	opts.ShouldSample = options.ShouldSample
	opts.Recorder = NewRecorder(c)
	return basictracer.NewWithOptions(opts)
}
