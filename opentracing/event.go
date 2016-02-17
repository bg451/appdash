package opentracing

import (
	"time"

	"sourcegraph.com/sourcegraph/appdash"
)

func init() {
	appdash.RegisterEvent(spanCompletionEvent{})
}

// SpanCompletionEvent is an event that satisfies the appdash.TimespanEvent
// interaface. This is used to show its beginning and end times, and total time.
type spanCompletionEvent struct {
	S time.Time `trace:"Span.Start"`
	E time.Time `trace:"Span.End"`
}

func (s spanCompletionEvent) Schema() string   { return "spancompletion" }
func (s spanCompletionEvent) Start() time.Time { return s.S }
func (s spanCompletionEvent) End() time.Time   { return s.E }
