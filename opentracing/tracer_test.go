package opentracing

import (
	"testing"

	"github.com/opentracing/opentracing-go"

	"sourcegraph.com/sourcegraph/appdash"
)

var tracer opentracing.Tracer

func TestTextSerialization(t *testing.T) {
	collector := appdash.NewMemoryStore()
	id := appdash.SpanID{0, 0, 0}
	recorder := appdash.NewRecorder(id, collector)
	tracer = NewTracer(recorder)
}
