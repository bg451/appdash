package opentracing

import (
	"testing"

	"sourcegraph.com/sourcegraph/appdash"
)

func newSpan() *Span {
	collector := appdash.NewMemoryStore()
	spanID := appdash.SpanID{100, 200, 300}
	recorder := appdash.NewRecorder(spanID, collector)
	return newAppdashSpan("parent", recorder, true)
}

func TestSpanStartChild(t *testing.T) {
	span := newSpan()
	spanID := span.Recorder.SpanID

	otChild := span.StartChild("child") // returns opentracing.Span

	// Convert it back to a *Span to access it's internal structures
	child := otChild.(*Span)

	traceid := child.Recorder.SpanID.Trace
	if traceid != spanID.Trace {
		t.Error("Expected child's TraceID to be", spanID.Trace, "got", traceid)
	}

	parentID := child.Recorder.SpanID.Parent
	if parentID != spanID.Span {
		t.Error("Expected the child's ParentID to be", spanID.Span, "got", parentID)
	}

	id := child.Recorder.SpanID.Span
	if id == spanID.Span {
		t.Error("Child's SpanID is the same as it's parent")
	}
}

func TestSpanAttrPropagation(t *testing.T) {
	parent := newSpan()
	key := "some-attr"
	expected := "val"
	parent.SetTraceAttribute(key, expected)

	if actual := parent.TraceAttribute(key); actual != expected {
		t.Error("Expected trace attribute to be", expected, "got", actual)
	}

	if parent.TraceAttribute("doesnt_exist") != "" {
		t.Error("Expected TraceAttribute to return an empty string for a nonexistent key")
	}

	child := parent.StartChild("child")
	if actual := child.TraceAttribute(key); actual != expected {
		t.Error("Expected child span to have trace attr", expected, "got", actual)
	}
}
