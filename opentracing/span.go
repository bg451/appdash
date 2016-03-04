package opentracing

import (
	"fmt"
	"sync"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"sourcegraph.com/sourcegraph/appdash"
)

// Span is the Appdash implementation of the opentracing.Span interface.
type Span struct {
	sync.Mutex
	Recorder      *appdash.Recorder
	tracer        *Tracer
	operationName string
	startTime     time.Time
	sampled       bool
	baggage       map[string]string
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

// Tracer returns the internal opentracing.Tracer
func (s *Span) Tracer() opentracing.Tracer {
	return s.tracer
}

// Finish ends the span. It is shorthand for:
//
// s.FinishWithOptions(opentracing.FinishOptions{FinishTime: time.Now()})
func (s *Span) Finish() {
	s.FinishWithOptions(opentracing.FinishOptions{FinishTime: time.Now()})
}

// FinishWithOptions finishes the span with opentracing.FinishOptions.
//
// Internally, the appdash.Reporter reports the span's name, tags,
// baggage, and log events.
func (s *Span) FinishWithOptions(opts opentracing.FinishOptions) {
	if !s.sampled {
		return
	}

	s.Lock()
	defer s.Unlock()

	s.Recorder.Name(s.operationName) // Record the span's name

	// Convert span tags to annotations.
	for key, value := range s.tags {
		val := []byte(fmt.Sprintf("%v", value))
		s.Recorder.Annotation(appdash.Annotation{Key: key, Value: val})
	}

	// Record any baggage as annotations.
	for key, value := range s.baggage {
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

	// Send a Timespan event. By doing this, we can actually see how long spans
	// took in the Appdash UI.
	s.Recorder.Event(appdash.Timespan{S: s.startTime, E: endTime})
}

// SetTag sets a key value tag.
//
// The value is an arbitrary type, but the system must know how to handle it,
// otherwise the behavior is undefined when reporting the tags.
func (s *Span) SetTag(key string, value interface{}) opentracing.Span {
	s.Lock()
	defer s.Unlock()

	s.tags[key] = value
	return s
}

// Log does not report the data right away, but instead stores it internally.
// Once (*Span).Finish() is called, all of the data is reported.
// See opentracing.LogData for more details on the semantics of the data.
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

// SetBaggageItem adds a baggage item to the trace.
//
// If the supplied key doesn't match opentracing.CanonicalizeBaggageKey,
// the key will still be used, however the behavior is undefined.
func (s *Span) SetBaggageItem(restrictedKey, value string) opentracing.Span {
	key, valid := opentracing.CanonicalizeBaggageKey(restrictedKey)
	if !valid {
		key = restrictedKey
	}

	s.Lock()
	defer s.Unlock()

	s.baggage[key] = value
	return s
}

// BaggageItem returns the value for a given key. If the key doesn't exist,
// an empty string is returned. It will attempt to canonicalize the key,
// however if it doesn't match the match the expected pattern it will use the
// provided key.
func (s *Span) BaggageItem(restrictedKey string) (value string) {
	key, valid := opentracing.CanonicalizeBaggageKey(restrictedKey)
	if !valid {
		key = restrictedKey
	}

	s.Lock()
	value, ok := s.baggage[key]
	s.Unlock()
	if !ok {
		return ""
	}
	return value
}
