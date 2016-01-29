package opentracing

import (
	"testing"

	"github.com/opentracing/opentracing-go"
)

func TestSpan(t *testing.T) {
	// Placeholder for now, this just checks if it satisfies the interface
	var _ opentracing.Span = &Span{}
}
