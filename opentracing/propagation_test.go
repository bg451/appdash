package opentracing

import (
	"testing"

	opentracing "github.com/opentracing/opentracing-go"

	"sourcegraph.com/sourcegraph/appdash"
)

func TestSplitTextPropagator(t *testing.T) {
	var c noopCollector
	r := appdash.NewRecorder(appdash.SpanID{}, &c)
	tracer := NewTracerWithOptions(r, Options{SampleFunc: alwaysTraceFunc})
	key := "key"
	value := "value"
	sp1 := tracer.StartSpan("span_a")
	sp1.SetTraceAttribute(key, value)

	// Inject the span into the carrier
	carrier := opentracing.NewSplitTextCarrier()
	tracer.Injector(opentracing.SplitText).InjectSpan(sp1, carrier)

	// Extract it.
	sp2, err := tracer.Extractor(opentracing.SplitText).JoinTrace("", carrier)
	if err != nil {
		t.Errorf("Error extracting span %s", err)
	}

	compareSpans(sp1, sp2, t)
}

func TestSplitBinaryPropagator(t *testing.T) {
	var c noopCollector
	r := appdash.NewRecorder(appdash.SpanID{}, &c)
	tracer := NewTracerWithOptions(r, Options{SampleFunc: alwaysTraceFunc})
	key := "key"
	value := "value"
	sp1 := tracer.StartSpan("span_a")
	sp1.SetTraceAttribute(key, value)

	// Inject the span into the carrier
	carrier := opentracing.NewSplitBinaryCarrier()
	tracer.Injector(opentracing.SplitBinary).InjectSpan(sp1, carrier)

	// Extract it.
	sp2, err := tracer.Extractor(opentracing.SplitBinary).JoinTrace("", carrier)
	if err != nil {
		t.Errorf("Error extracting span %s", err)
	}
	compareSpans(sp1, sp2, t)
}

// Compares a parent and a child span that belongs to the parent.
func compareSpans(otParent, otChild opentracing.Span, t *testing.T) {
	parent := otParent.(*Span)
	child := otChild.(*Span)
	spanID := parent.Recorder.SpanID
	spanID2 := child.Recorder.SpanID

	if spanID.Trace != spanID2.Trace {
		t.Errorf("Expected trace id to be the same, got %d and %d",
			spanID.Trace, spanID2.Trace)
	}

	if spanID.Span != spanID2.Parent {
		t.Errorf("Expected new span to have parent span %d, got %d",
			spanID.Span, spanID2.Parent)
	}
	if parent.sampled != child.sampled {
		t.Errorf("Expected sampling status to be the same, got sp1 %b and sp2 %b",
			parent.sampled, child.sampled)
	}

	if len(parent.attributes) != len(child.attributes) {
		t.Errorf("Expected amount of trace attributes to be the same, got %d, %d",
			len(parent.attributes), len(child.attributes))
	}

	for key, parentValue := range parent.attributes {
		childValue, ok := child.attributes[key]
		if !ok {
			t.Errorf("Expected child span to have parent trace attribute %s", key)
		}
		if parentValue != childValue {
			t.Errorf("Expected key values to be the same, got %s, %s",
				parentValue, childValue)
		}
	}
}
