package opentracing

import (
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"sourcegraph.com/sourcegraph/appdash"
)

var (
	// The zero value of an unitialized time.Time
	timeZeroValue = time.Time{}
)

// Tracer is the appdash implementation of the opentracing-go API.
type Tracer struct {
	recorder         *appdash.Recorder
	options          Options
	textPropagator   *splitTextPropagator
	binaryPropagator *splitBinaryPropagator
	goHTTPPropagator *goHTTPPropagator
}

type Options struct {
	SampleFunc func(int64) bool
}

func defaultOptions() Options {
	return Options{
		SampleFunc: func(int64) bool { return true },
	}
}

// NewTracer returns a new Tracer that implements the `opentracing.Tracer`
// interface.
//
// NewAppdashTracer requires an `appdash.Recorder` in order to serialize and
// write events to an Appdash store.
func NewTracer(r *appdash.Recorder) opentracing.Tracer {
	return NewTracerWithOptions(r, defaultOptions())
}

func NewTracerWithOptions(r *appdash.Recorder, opts Options) opentracing.Tracer {
	t := &Tracer{recorder: r, options: opts}
	t.textPropagator = &splitTextPropagator{t}
	t.binaryPropagator = &splitBinaryPropagator{t}
	t.goHTTPPropagator = &goHTTPPropagator{t.binaryPropagator}
	return t
}

func (t *Tracer) StartSpan(operationName string) opentracing.Span {
	return t.StartSpanWithOptions(opentracing.StartSpanOptions{OperationName: operationName})
}

func (t *Tracer) StartSpanWithOptions(opts opentracing.StartSpanOptions) opentracing.Span {
	sp := newAppdashSpan(opts.OperationName, t)

	if opts.Parent != nil {
		sp.Recorder = opts.Parent.(*Span).Recorder.Child()
		sp.sampled = opts.Parent.(*Span).sampled
	} else {
		sp.Recorder = t.recorder.Child()
		sp.Recorder.SpanID = appdash.NewRootSpanID()
		sp.sampled = t.options.SampleFunc(int64(sp.Recorder.SpanID.Trace))
	}

	if opts.StartTime != timeZeroValue {
		sp.startTime = opts.StartTime
	}

	if opts.Tags != nil {
		sp.tags = opts.Tags
	} else {
		sp.tags = make(map[string]interface{}, 0)
	}

	sp.attributes = make(map[string]string, 0)

	return sp
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
