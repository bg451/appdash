package opentracing

import (
	"sync"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"sourcegraph.com/sourcegraph/appdash"
)

// Span is the Appdash implemntation of the `opentracing.Span` interface.
type Span struct {
	Recorder      *appdash.Recorder
	tracer        *Tracer
	operationName string
	startTime     time.Time
	sampled       bool // If the trace is sampled or not

	attrLock   sync.Mutex
	attributes map[string]string

	tagLock sync.Mutex
	tags    opentracing.Tags

	logLock sync.Mutex
	logs    []opentracing.LogData
}

func newAppdashSpan(operationName string, tracer *Tracer, r *appdash.Recorder, sampled bool) *Span {
	return &Span{
		Recorder:      r,
		tracer:        tracer,
		operationName: operationName,
		startTime:     time.Now(),
		sampled:       sampled,
		attributes:    make(map[string]string),
		tags:          make(opentracing.Tags),
		logs:          make([]opentracing.LogData, 0),
	}
}

// SetOperationName setss the name of the span, overwriting the previous value.
func (s *Span) SetOperationName(operationName string) opentracing.Span {
	s.operationName = operationName
	return s
}

func (s *Span) Tracer() opentracing.Tracer {
	return s.tracer
}

// Finish ends the span.
//
func (s *Span) Finish() {
	s.FinishWithOptions(opentracing.FinishOptions{FinishTime: time.Now()})
}

// Internally, the `appdash.Reporter` reports the span's name, tags,
// attributes, and log events.
func (s *Span) FinishWithOptions(opts opentracing.FinishOptions) {
	if !s.sampled {
		return
	}

	s.Recorder.Name(s.operationName) // Set the span's name

	// Convert span tags to annotations.
	for key, value := range s.tags {
		// XXX: I'm not sure right now how to represente arbritrary structs,
		// so strings will have to do for now.
		if v, ok := value.(string); ok {
			s.Recorder.Annotation(appdash.Annotation{Key: key, Value: []byte(v)})
		}
	}

	// Record any trace attributes as annotations.
	for key, value := range s.attributes {
		s.Recorder.Annotation(appdash.Annotation{Key: key, Value: []byte(value)})
	}

	// TODO(bg): appdash.timespanEvent should be public and use recorder.Event
	// with the used timestamps.
	for _, log := range s.logs {
		s.Recorder.Log(log.Event)
	}

	// Log all bulk log data
	for _, log := range opts.BulkLogData {
		s.Recorder.Log(log.Event)
	}

	// Send a SpanCompletionEvent, which satisfies the appdash.Timespan interface
	// By doing this, we can actually see how long spans took.
	s.Recorder.Event(spanCompletionEvent{s.startTime, time.Now()})
}

// SetTag sets a key value pair.
//
// The value is an arbritary type, but the system must know how to handle it,
// otherwise the behavior is undefined when reporting the tags.
func (s *Span) SetTag(key string, value interface{}) opentracing.Span {
	if s.sampled {
		s.tagLock.Lock()
		defer s.tagLock.Unlock()

		s.tags[key] = value
	}
	return s
}

// Log does not report the data right away, but instead stores it internally.
// Once (*Span).Finish() is called, all of the data is reported.
// See `opentracing.LogData` for more details on the semantics of the data.
func (s *Span) Log(data opentracing.LogData) {
	if !s.sampled {
		return
	}
	s.logLock.Lock()
	defer s.logLock.Unlock()

	s.logs = append(s.logs, data)
}

// LogEvent is short for Log(opentracing.LogData{Event: event, ...})
func (s *Span) LogEvent(event string) {
	if s.sampled {
		s.Log(opentracing.LogData{Event: event, Timestamp: time.Now()})
	}
}

// LogEventWithPayload is short for
// Log(opentracing.LogData{Event: event, Payload: payload, ...}).
func (s *Span) LogEventWithPayload(event string, payload interface{}) {
	if s.sampled {
		s.Log(opentracing.LogData{Event: event, Timestamp: time.Now(), Payload: payload})
	}
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

	s.attrLock.Lock()
	defer s.attrLock.Unlock()

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

	s.attrLock.Lock()
	value, ok := s.attributes[key]
	s.attrLock.Unlock()
	if !ok {
		return ""
	}
	return value
}
