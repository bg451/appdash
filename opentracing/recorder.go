package opentracing

import (
	"fmt"

	basictracer "github.com/opentracing/basictracer-go"
	"sourcegraph.com/sourcegraph/appdash"
)

// Recorder is a struct that implements basictracer.Recorder and contains
// an appdash.Collector which it writes to.
type Recorder struct {
	collector appdash.Collector
}

// NewRecorder forwards basictracer.RawSpans to an appdash.Collector.
func NewRecorder(collector appdash.Collector) *Recorder {
	return &Recorder{collector}
}

func (r *Recorder) RecordSpan(sp basictracer.RawSpan) {
	spanID := appdash.SpanID{
		Span:   appdash.ID(uint64(sp.SpanID)),
		Trace:  appdash.ID(uint64(sp.TraceID)),
		Parent: appdash.ID(uint64(sp.ParentSpanID)),
	}

	// XXX: What happens if the collector's connection to the server fails and
	// nothing gets written? Do we just keep silently failing? RecordSpan() and
	// Finish() don't return any possible errors.
	r.collectEvent(spanID, appdash.SpanName(sp.Operation)) // Record the Span name

	// Record all of the logs. Payloads are thrown out.
	for _, log := range sp.Logs {
		r.collectEvent(spanID, appdash.LogWithTimestamp(log.Event, log.Timestamp))
	}

	// Record the tags.
	for key, value := range sp.Tags {
		val := []byte(fmt.Sprintf("%v", value))
		r.collector.Collect(spanID, appdash.Annotation{Key: key, Value: val})
	}

	// Record the baggage.
	for key, val := range sp.Baggage {
		r.collector.Collect(spanID, appdash.Annotation{Key: key, Value: []byte(val)})
	}

	// Add the duration to the start time to get an approximate end time.
	approxEndTime := sp.Start.Add(sp.Duration)
	r.collectEvent(spanID, appdash.Timespan{S: sp.Start, E: approxEndTime})
}

// collectEvent marshals and collects the Event or an error produced by
// by marshalling.
func (r *Recorder) collectEvent(spanID appdash.SpanID, e appdash.Event) {
	ans, err := appdash.MarshalEvent(e)
	if err != nil {
		r.logError(spanID, err)
		return
	}
	r.collector.Collect(spanID, ans...)
}

// logError converts an error into a log event and collects it.
func (r *Recorder) logError(spanID appdash.SpanID, err error) {
	an, _ := appdash.MarshalEvent(appdash.Log(err.Error()))
	r.collector.Collect(spanID, an...)
}
