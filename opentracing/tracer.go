package opentracing

import (
	opentracing "github.com/opentracing/opentracing-go"
	"sourcegraph.com/sourcegraph/appdash"
)

// Tracer is the appdash implementation of the opentracing-go API.
type Tracer struct {
	recorder         *appdash.Recorder
	textPropagator   *splitTextPropagator
	binaryPropagator *splitBinaryPropagator
	goHTTPPropagator *goHTTPPropagator
}

// NewTracer returns a new Tracer that implements the `opentracing.Tracer`
// interface.
//
// NewAppdashTracer requires an `appdash.Recorder` in order to serialize and
// write events to an Appdash store.
func NewTracer(r *appdash.Recorder) *Tracer {
	t := &Tracer{recorder: r}
	t.textPropagator = &splitTextPropagator{t}
	t.binaryPropagator = &splitBinaryPropagator{t}
	t.goHTTPPropagator = &goHTTPPropagator{t.binaryPropagator}
	return t
}

func (t *Tracer) StartSpan(operationName string) opentracing.Span {
	return t.StartSpanWithOptions(opentracing.StartSpanOptions{OperationName: operationName})
}

func (t *Tracer) StartSpanWithOptions(opts opentracing.StartSpanOptions) opentracing.Span {
	r := t.recorder.Child()
	spanID := appdash.NewRootSpanID()
	r.SpanID = spanID
	return newAppdashSpan(opts.OperationName, t, r, true)
}

func (t *Tracer) Extractor(format interface{}) opentracing.Extractor {
	switch format {
	case opentracing.SplitText:
		return t.textPropagator
	case opentracing.SplitBinary:
		return t.binaryPropagator
	case opentracing.GoHTTPHeader:
		return t.goHTTPPropagator
	}
	return nil
}

func (t *Tracer) Injector(format interface{}) opentracing.Injector {
	switch format {
	case opentracing.SplitText:
		return t.textPropagator
	case opentracing.SplitBinary:
		return t.binaryPropagator
	case opentracing.GoHTTPHeader:
		return t.goHTTPPropagator
	}
	return nil
}

func (t *Tracer) newChildRecorder(parentSpanID, traceID uint64) *appdash.Recorder {
	rec := t.recorder.Child()
	spanID := appdash.NewSpanID(
		appdash.SpanID{Trace: appdash.ID(traceID), Span: appdash.ID(parentSpanID)})
	rec.SpanID = spanID
	return rec
}
