package opentracing

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strconv"

	"github.com/opentracing/opentracing-go"
	"sourcegraph.com/sourcegraph/appdash"
)

type Tracer struct {
	recorder *appdash.Recorder
	name     string
}

// NewAppdashTracer returns a new Tracer that implements the `opentracing.Tracer`
// interface.
//
// NewAppdashTracer requires an `appdash.Recorder` in order to serialize and
// write events to an Appdash store.
func NewTracer(name string, r *appdash.Recorder) *Tracer {
	return &Tracer{recorder: r}
}

// StartTrace starts a new Trace and returns a new span.
//
// Specifically, StartTrace generates a new root SpanID and creates a child
// off the Tracer's recorder, changing the recorder's SpanID to contain the
// new root SpanID.
func (t *Tracer) StartTrace(operationName string) opentracing.Span {
	spanId := appdash.NewRootSpanID()
	recorder := t.recorder.Child()
	recorder.SpanID = spanId

	return newAppdashSpan(recorder, operationName)
}

func (t *Tracer) PropagateSpanAsText(
	sp opentracing.Span,
) (
	contextIDMap map[string]string,
	attrsMap map[string]string,
) {
	span := sp.(*Span)

	contextIDMap = map[string]string{
		// Calling this SpanID is a bit disingenuous imo
		"Spanid":  strconv.FormatUint(uint64(span.Recorder.SpanID.Span), 10),
		"Traceid": strconv.FormatUint(uint64(span.Recorder.SpanID.Trace), 10),
	}

	attrsMap = make(map[string]string)
	for key, value := range span.attributes {
		attrsMap[key] = value
	}
	return contextIDMap, attrsMap
}

// JoinTraceFromText joins and returns a new child span.
//
// JoinTraceFromText expects a parsable appdash.SpanID string. It will use that
// SpanID to create a new child span.
func (t *Tracer) JoinTraceFromText(
	operationName string,
	contextSnapshot map[string]string,
	traceAttrs map[string]string,
) (
	opentracing.Span,
	error,
) {

	spanID, err := parseUintFromMap(contextSnapshot, "Spanid")
	if err != nil {
		return nil, err
	}

	traceID, err := parseUintFromMap(contextSnapshot, "Traceid")
	if err != nil {
		return nil, err
	}

	span := t.StartTrace(operationName)
	span.(*Span).Recorder.SpanID = appdash.NewSpanID(
		appdash.SpanID{Trace: appdash.ID(traceID), Span: appdash.ID(spanID)})

	for k, v := range traceAttrs {
		span.SetTraceAttribute(k, v)
	}

	return span, nil
}

func parseUintFromMap(attrs map[string]string, key string) (uint64, error) {
	v, ok := attrs[key]
	if !ok {
		return 0, fmt.Errorf("%s does not exist", key)
	}
	return strconv.ParseUint(v, 10, 64)
}

// PropagateSpanAsText returns a binary representation of an Appdash span
// using encoding/gob.
//
// The only thing that gets encoded is the SpanID.
func (t *Tracer) PropagateSpanAsBinary(
	sp opentracing.Span,
) (
	contextSnapshot []byte,
	traceAttrs []byte,
) {
	var contextSnapshotBuffer bytes.Buffer
	s := sp.(*Span)
	err := gob.NewEncoder(&contextSnapshotBuffer).Encode(s.Recorder.SpanID)
	if err != nil {
		panic("error encoding SpanId")
	}

	var traceAttrsBuffer bytes.Buffer
	err = gob.NewEncoder(&traceAttrsBuffer).Encode(s.attributes)
	if err != nil {
		panic("error encoding trace attributes")
	}

	return contextSnapshotBuffer.Bytes(), traceAttrsBuffer.Bytes()
}

func (t *Tracer) JoinTraceFromBinary(
	operationName string,
	contextSnapshot []byte,
	traceAttrs []byte,
) (
	opentracing.Span,
	error,
) {

	spanID := appdash.SpanID{}
	contextSnapshotBuffer := bytes.NewBuffer(contextSnapshot)
	err := gob.NewDecoder(contextSnapshotBuffer).Decode(&spanID)
	if err != nil {
		return nil, err
	}

	traceAttrMap := make(map[string]string)
	traceAttrsBuffer := bytes.NewBuffer(traceAttrs)
	err = gob.NewDecoder(traceAttrsBuffer).Decode(&traceAttrMap)
	if err != nil {
		return nil, err
	}

	span := t.StartTrace(operationName)
	span.(*Span).Recorder.SpanID = appdash.NewSpanID(spanID)

	for k, v := range traceAttrMap {
		span.SetTraceAttribute(k, v)
	}
	return span, nil
}
