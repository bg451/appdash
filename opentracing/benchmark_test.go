package opentracing

import (
	"fmt"
	"testing"

	"sourcegraph.com/sourcegraph/appdash"

	"github.com/opentracing/opentracing-go"
)

var tags []string

func init() {
	tags = make([]string, 1000)
	for j := 0; j < len(tags); j++ {
		tags[j] = fmt.Sprintf("%d", j)
	}
}

// Credit to github.com/opentracing/opentracing-go/blob/b95bb770247870c2cf2b194a52f77d2077349f75/standardtracer/bench_test.go
func benchmarkWithOps(b *testing.B, numEvent, numTag, numItems int) {
	var r noopCollector
	recorder := appdash.NewRecorder(appdash.SpanID{}, &r)
	t := NewTracerWithOptions(recorder, Options{SampleFunc: noTraceFunc})
	benchmarkWithOpsAndCB(b, func() opentracing.Span {
		return t.StartSpan("test")
	}, numEvent, numTag, numItems)
}

func benchmarkWithOpsAndCB(b *testing.B, create func() opentracing.Span,
	numEvent, numTag, numItems int) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sp := create()
		for j := 0; j < numEvent; j++ {
			sp.LogEvent("event")
		}
		for j := 0; j < numTag; j++ {
			sp.SetTag(tags[j], nil)
		}
		for j := 0; j < numItems; j++ {
			sp.SetBaggageItem(tags[j], tags[j])
		}
		sp.Finish()
	}
	b.StopTimer()
}

func BenchmarkSpan_Empty(b *testing.B) {
	benchmarkWithOps(b, 0, 0, 0)
}

func BenchmarkSpan_100Events(b *testing.B) {
	benchmarkWithOps(b, 100, 0, 0)
}

func BenchmarkSpan_1000Events(b *testing.B) {
	benchmarkWithOps(b, 1000, 0, 0)
}

func BenchmarkSpan_100Tags(b *testing.B) {
	benchmarkWithOps(b, 0, 100, 0)
}

func BenchmarkSpan_1000Tags(b *testing.B) {
	benchmarkWithOps(b, 0, 1000, 0)
}

func BenchmarkSpan_100BaggageItems(b *testing.B) {
	benchmarkWithOps(b, 0, 0, 100)
}

func BenchmarkSpan_1000BaggageItems(b *testing.B) {
	benchmarkWithOps(b, 0, 0, 100)
}
