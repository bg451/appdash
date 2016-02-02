package opentracing

import (
	"testing"

	"sourcegraph.com/sourcegraph/appdash"

	"github.com/opentracing/opentracing-go"
)

func TestSpanStartChild(t *testing.T) {
	collector := appdash.NewMemoryStore()
	spanID := appdash.SpanID{100, 200, 300}
	recorder := appdash.NewRecorder(spanID, collector)
	var span opentracing.Span = newAppdashSpan("parent", recorder, true)

	otChild := span.StartChild("child") // returns opentracing.Span

	// Convert it back to a *Span to access it's internal structures
	child := otChild.(*Span)

	traceid := child.Recorder.SpanID.Trace
	if traceid != spanID.Trace {
		t.Error("Expected child's TraceID to be", spanID.Trace, "got", traceid)
	}

	parent := child.Recorder.SpanID.Parent
	if parent != spanID.Span {
		t.Error("Expected the child's ParentID to be", spanID.Span, "got", parent)
	}

	id := child.Recorder.SpanID.Span
	if id == spanID.Span {
		t.Error("Child's SpanID is the same as it's parent")
	}
}
