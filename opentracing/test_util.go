package opentracing

import "sourcegraph.com/sourcegraph/appdash"

// noopCollector implements appdash.Collector interface. For use with a recorder.
type noopCollector struct{}

func (c *noopCollector) Collect(s appdash.SpanID, ans ...appdash.Annotation) error {
	return nil
}

// noTraceFunc is a sampling function that always returns false.
func noTraceFunc(int64) bool     { return false }
func alwaysTraceFunc(int64) bool { return true }
