package opentracing

import (
	"time"

	"github.com/opentracing/opentracing-go"
	"sourcegraph.com/sourcegraph/appdash"
)

// Span is the Appdash implemntation of the `opentracing.Span` interface.
//
// Span is not safe for concurrent usage.
type Span struct {
	Recorder      *appdash.Recorder
	operationName string
	startTime     time.Time
	attributes    map[string]string
	tags          opentracing.Tags
	logs          []opentracing.LogData
}

func newAppdashSpan(recorder *appdash.Recorder, operationName string) opentracing.Span {
	return &Span{
		Recorder:      recorder,
		operationName: operationName,
		startTime:     time.Now(),
		attributes:    make(map[string]string),
		tags:          make(opentracing.Tags),
		logs:          make([]opentracing.LogData, 0),
	}
}

// Sets the name of the span, overwriting the previous value.
func (s *Span) SetOperationName(operationName string) opentracing.Span {
	s.operationName = operationName
	return s
}

// StartChild returns a new Span.
//
// The child is created using the parent span's appdash.Recorder.
func (s *Span) StartChild(operationName string) opentracing.Span {
	recorder := s.Recorder.Child()
	return newAppdashSpan(recorder, operationName)
}

// Finish() ends the span.
//
// Internally, the `appdash.Reporter` reports the span's name, tags,
// attributes, and log events.
func (s *Span) Finish() {
	s.Recorder.Name(s.operationName) // Set the span's name

	for key, value := range s.tags {
		// Set the tags.
		// XXX: I'm not sure right now how to represente arbritrary structs,
		// so strings will have to do for now.
		if v, ok := value.(string); ok {
			s.Recorder.Annotation(appdash.Annotation{Key: key, Value: []byte(v)})
		}
	}

	for key, value := range s.attributes {
		// Record any trace attributes
		s.Recorder.Annotation(appdash.Annotation{Key: key, Value: []byte(value)})
	}

	for _, log := range s.logs {
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
	s.tags[key] = value
	return s
}

// Log does not report the data right away, but instead stores it internally.
// Once (*Span).Finish() is called, all of the data is reported.
// See `opentracing.LogData` for more details on the semantics of the data.
func (s *Span) Log(data opentracing.LogData) {
	s.logs = append(s.logs, data)
}

// LogEvent is short for Log(opentracing.LogData{Event: event, ...})
func (s *Span) LogEvent(event string) {
	s.Log(opentracing.LogData{Event: event, Timestamp: time.Now()})
}

// LogEvent is short for Log(opentracing.LogData{Event: event, Payload: payload, ...})
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
	s.attributes[key] = value
	return s
}

// TraceAttribute retuns the value for a given key. If the key doesn't exist,
// an empty string is returned.
func (s *Span) TraceAttribute(key string) (value string) {
	value, ok := s.attributes[key]
	if !ok {
		return ""
	}
	return value
}
