package opentracing

import (
	"net/http"
	"testing"

	opentracing "github.com/opentracing/opentracing-go"

	"sourcegraph.com/sourcegraph/appdash"
)

func TestOpentracingPropagators(t *testing.T) {
	var c noopCollector
	r := appdash.NewRecorder(appdash.SpanID{}, &c)
	tracer := NewTracerWithOptions(r, Options{SampleFunc: alwaysTraceFunc})
	key := "key"
	value := "value"

	sp1 := tracer.StartSpan("test")
	sp1.SetBaggageItem(key, value)

	tests := []struct {
		format, carrier interface{}
	}{
		{opentracing.SplitBinary, opentracing.NewSplitBinaryCarrier()},
		{opentracing.SplitText, opentracing.NewSplitTextCarrier()},
		{opentracing.GoHTTPHeader, http.Header{}},
	}
	for i, test := range tests {
		// Inject the span into the carrier
		err := tracer.Inject(sp1, test.format, test.carrier)
		if err != nil {
			t.Fatalf("%d: %s", i, err)
		}

		// Extract it.
		sp2, err := tracer.Join("child", test.format, test.carrier)
		if err != nil {
			t.Errorf("%d: %s", i, err)
		}

		compareSpans(sp1, sp2, i, t)
	}
}

// Compares a parent and a child span that belongs to the parent.
func compareSpans(otParent, otChild opentracing.Span, i int, t *testing.T) {
	parent := otParent.(*Span)
	child := otChild.(*Span)
	spanID := parent.Recorder.SpanID
	spanID2 := child.Recorder.SpanID

	if spanID.Trace != spanID2.Trace {
		t.Errorf("%d: Expected trace id to be the same, got %d and %d",
			i, spanID.Trace, spanID2.Trace)
	}

	if spanID.Span != spanID2.Parent {
		t.Errorf("%d: Expected new span to have parent span %d, got %d",
			i, spanID.Span, spanID2.Parent)
	}
	if parent.sampled != child.sampled {
		t.Errorf("%d: Expected sampling status to be the same, got sp1 %b and sp2 %b",
			i, parent.sampled, child.sampled)
	}

	if len(parent.baggage) != len(child.baggage) {
		t.Errorf("%d: Expected amount of trace baggage to be the same, got %d, %d",
			i, len(parent.baggage), len(child.baggage))
	}

	for key, parentValue := range parent.baggage {
		childValue, ok := child.baggage[key]
		if !ok {
			t.Errorf("Expected child span to have parent trace attribute %s", key)
		}
		if parentValue != childValue {
			t.Errorf("Expected key values to be the same, got %s, %s",
				parentValue, childValue)
		}
	}
}
