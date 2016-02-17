package opentracing

import (
	"fmt"
	"sync"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"sourcegraph.com/sourcegraph/appdash"
)

// Span is the Appdash implemntation of the `opentracing.Span` interface.
type Span struct {
	sync.Mutex
	Recorder      *appdash.Recorder
	tracer        *Tracer
	operationName string
	startTime     time.Time
	sampled       bool
	attributes    map[string]string
	tags          map[string]interface{}

	logs []opentracing.LogData
}

func newAppdashSpan(operationName string, tracer *Tracer) *Span {
	return &Span{
		operationName: operationName,
		tracer:        tracer,
		startTime:     time.Now(),
		logs:          make([]opentracing.LogData, 0),
	}
}

// SetOperationName sets the name of the span, overwriting the previous value.
func (s *Span) SetOperationName(operationName string) opentracing.Span {
	s.operationName = operationName
	return s
}

// Tracer returns the interal opentracing.Tracer
func (s *Span) Tracer() opentracing.Tracer {
	return s.tracer
}

// Finish ends the span.
//
// Defers to s.FinishWithOptions() with FinishTime = time.Now()
func (s *Span) Finish() {
	s.FinishWithOptions(opentracing.FinishOptions{FinishTime: time.Now()})
}

// FinishWithOptions finishes the span with opentracing.FinishOptions.
//
// Internally, the `appdash.Reporter` reports the span's name, tags,
// attributes, and log events.
func (s *Span) FinishWithOptions(opts opentracing.FinishOptions) {
	if !s.sampled {
		return
	}

	s.Lock()
	defer s.Unlock()

	s.Recorder.Name(s.operationName) // Set the span's name

	// Convert span tags to annotations.
	for key, value := range s.tags {
		val := []byte(fmt.Sprintf("%v", value))
		s.Recorder.Annotation(appdash.Annotation{Key: key, Value: val})
	}

	// Record any trace attributes as annotations.
	for key, value := range s.attributes {
		s.Recorder.Annotation(appdash.Annotation{Key: key, Value: []byte(value)})
	}

	// XXX(bg): I'm not too sure how this works in Appdash, but there needs to
	// be a way to record a key value pair where the value is the payload.
	for _, log := range s.logs {
		s.Recorder.LogWithTimestamp(log.Event, log.Timestamp)
	}

	// Log all bulk log data
	for _, log := range opts.BulkLogData {
		s.Recorder.LogWithTimestamp(log.Event, log.Timestamp)
	}

	endTime := opts.FinishTime
	if endTime == timeZeroValue {
		endTime = time.Now()
	}

	// Send a SpanCompletionEvent, which satisfies the appdash.Timespan interface
	// By doing this, we can actually see how long spans took.
	s.Recorder.Event(spanCompletionEvent{s.startTime, endTime})
}

// SetTag sets a key value pair.
//
// The value is an arbritary type, but the system must know how to handle it,
// otherwise the behavior is undefined when reporting the tags.
func (s *Span) SetTag(key string, value interface{}) opentracing.Span {
	s.Lock()
	defer s.Unlock()

	s.tags[key] = value
	return s
}

// Log does not report the data right away, but instead stores it internally.
// Once (*Span).Finish() is called, all of the data is reported.
// See `opentracing.LogData` for more details on the semantics of the data.
func (s *Span) Log(data opentracing.LogData) {
	s.Lock()
	s.Unlock()
	s.logs = append(s.logs, data)
}

// LogEvent is short for Log(opentracing.LogData{Event: event, ...})
func (s *Span) LogEvent(event string) {
	s.Log(opentracing.LogData{Event: event, Timestamp: time.Now()})
}

// LogEventWithPayload is short for
// Log(opentracing.LogData{Event: event, Payload: payload, ...}).
func (s *Span) LogEventWithPayload(event string, payload interface{}) {
	s.Log(opentracing.LogData{Event: event, Timestamp: time.Now(), Payload: payload})
}

// SetTraceAttribute adds a key value pair to the trace's attributes.
//
// If the supplied key doesn't match opentracing.CanonicalizeTraceAttributeKey,
// the key will still be used, however the behavior is undefined.
func (s *Span) SetTraceAttribute(restrictedKey, value string) opentracing.Span {
	key, valid := opentracing.CanonicalizeTraceAttributeKey(restrictedKey)
	if !valid {
		key = restrictedKey
	}

	s.Lock()
	defer s.Unlock()

	s.attributes[key] = value
	return s
}

// TraceAttribute retuns the value for a given key. If the key doesn't exist,
// an empty string is returned. It will attempt to canoncicalize the key,
// however if it doesn't match the match the expected pattern it will use the
// provided key.
func (s *Span) TraceAttribute(restrictedKey string) (value string) {
	key, valid := opentracing.CanonicalizeTraceAttributeKey(restrictedKey)
	if !valid {
		key = restrictedKey
	}

	s.Lock()
	value, ok := s.attributes[key]
	s.Unlock()
	if !ok {
		return ""
	}
	return value
}
