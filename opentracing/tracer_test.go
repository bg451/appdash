package opentracing

import (
	"testing"

	"sourcegraph.com/sourcegraph/appdash"
)

func TestTextSerialization(t *testing.T) {
	collector := appdash.NewMemoryStore()
	id := appdash.SpanID{0, 0, 0}
	recorder := appdash.NewRecorder(id, collector)
	tracer := NewTracer("my-tracer", recorder)

	parentSpan := tracer.StartTrace("test.parent")
	attrKey := "test123"
	attrVal := "1isthisworking-"
	parentSpan.SetTraceAttribute(attrKey, attrVal)
	parentSpanId := parentSpan.(*Span).Recorder.SpanID
	contextMap, attrMap := tracer.PropagateSpanAsText(parentSpan)
	childSpan, err := tracer.JoinTraceFromText("", contextMap, attrMap)
	if err != nil {
		t.Error(err)
	}

	// Make sure the trace attributes were propagated.
	if childSpan.TraceAttribute(attrKey) != attrVal {
		t.Error("Expected trace attribute to be propagated")
	}

	if childSpan.(*Span).Recorder.Trace != parentSpanId.Trace {
		t.Error("Expected child to have same trace id as the parent")
	}

	if childSpan.(*Span).Recorder.Parent != parentSpanId.Span {
		t.Error("Expected child's parent id to be", parentSpanId.Span,
			"got", childSpan.(*Span).Recorder.Parent)
	}
}

func TestBinarySerialization(t *testing.T) {
	collector := appdash.NewMemoryStore()
	id := appdash.SpanID{0, 0, 0}
	recorder := appdash.NewRecorder(id, collector)
	tracer := NewTracer("my-tracer", recorder)

	parentSpan := tracer.StartTrace("test.parent")
	attrKey := "test123"
	attrVal := "1isthisworking-"
	parentSpan.SetTraceAttribute(attrKey, attrVal)
	parentSpanId := parentSpan.(*Span).Recorder.SpanID
	contextMap, attrMap := tracer.PropagateSpanAsBinary(parentSpan)
	childSpan, err := tracer.JoinTraceFromBinary("", contextMap, attrMap)

	if err != nil {
		t.Error(err)
	}

	// Make sure the trace attributes were propagated.
	if childSpan.TraceAttribute(attrKey) != attrVal {
		t.Error("Expected trace attribute to be propagated")
	}

	if childSpan.(*Span).Recorder.Trace != parentSpanId.Trace {
		t.Error("Expected child to have same trace id as the parent")
	}

	if childSpan.(*Span).Recorder.Parent != parentSpanId.Span {
		t.Error("Expected child's parent id to be", parentSpanId.Span,
			"got", childSpan.(*Span).Recorder.Parent)
	}
}
