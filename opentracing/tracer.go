package opentracing

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strconv"

	"github.com/opentracing/opentracing-go"
	"sourcegraph.com/sourcegraph/appdash"
)

const (
	// TRACEID_KEY is the trace id key for text propagation
	TRACEID_KEY = "Traceid"

	// SPANID_KEY is the span id key for text propagation
	SPANID_KEY = "Spanid"

	// SAMPLED_KEY is the key for determing whether a trace is is sampled or not.
	SAMPLED_KEY = "Sampled"
)

// Tracer is the appdash implementation of the opentracing-go API.
type Tracer struct {
	recorder *appdash.Recorder
	name     string
}

// NewTracer returns a new Tracer that implements the `opentracing.Tracer`
// interface.
//
// NewAppdashTracer requires an `appdash.Recorder` in order to serialize and
// write events to an Appdash store.
func NewTracer(name string, r *appdash.Recorder) *Tracer {
	return &Tracer{name: name, recorder: r}
}

// StartTrace starts a new Trace and returns a new span.
//
// Specifically, StartTrace generates a new root SpanID and creates a child
// off the Tracer's recorder, changing the recorder's SpanID to contain the
// new root SpanID.
func (t *Tracer) StartTrace(operationName string) opentracing.Span {
	spanID := appdash.NewRootSpanID()
	recorder := t.recorder.Child()

	// We need to overwrite the recorder's SpanID to be a root SpanID.
	recorder.SpanID = spanID

	// XXX: Sample everything for now, move away from this.
	sampled := true

	return newAppdashSpan(operationName, recorder, sampled)
}

// PropagateSpanAsText represents the Span for propagation as string:string text
// maps (see JoinTraceFromText()).
//
// Specific to Appdash, the contextSnapshot contains two pieces of core
// indentifying information, "traceid" and "spanid".
func (t *Tracer) PropagateSpanAsText(
	sp opentracing.Span,
) (
	contextIDMap map[string]string,
	attrsMap map[string]string,
) {
	span := sp.(*Span)

	contextIDMap = map[string]string{
		SPANID_KEY:  strconv.FormatUint(uint64(span.Recorder.SpanID.Span), 10),
		TRACEID_KEY: strconv.FormatUint(uint64(span.Recorder.SpanID.Trace), 10),
		SAMPLED_KEY: strconv.FormatBool(span.sampled),
	}

	attrsMap = make(map[string]string)
	for key, value := range span.attributes {
		attrsMap[key] = value
	}
	return contextIDMap, attrsMap
}

// JoinTraceFromText joins and returns a new child span using the text-encoded
// `contextSnapshot` and `traceAttrs` produced by PropagateSpanAsText.
func (t *Tracer) JoinTraceFromText(
	operationName string,
	contextSnapshot map[string]string,
	traceAttrs map[string]string,
) (
	opentracing.Span,
	error,
) {

	spanID, err := parseUintFromMap(contextSnapshot, SPANID_KEY)
	if err != nil {
		return nil, err
	}

	traceID, err := parseUintFromMap(contextSnapshot, TRACEID_KEY)
	if err != nil {
		return nil, err
	}

	sampled, err := strconv.ParseBool(contextSnapshot[SAMPLED_KEY])
	if err != nil {
		return nil, err
	}

	span := t.StartTrace(operationName)
	// I'm not a fan of doing this, I should probably make a private method
	// for creating this.
	span.(*Span).sampled = sampled
	span.(*Span).Recorder.SpanID = appdash.NewSpanID(
		appdash.SpanID{Trace: appdash.ID(traceID), Span: appdash.ID(spanID)})

	for k, v := range traceAttrs {
		span.SetTraceAttribute(k, v)
	}

	return span, nil
}

// parseUintFromMap tries to extract a uint64 from a map with a given key. If
// the key isn't in the map, it returns an error. Any error parsing the
// uint returns an error as well.
func parseUintFromMap(attrs map[string]string, key string) (uint64, error) {
	v, ok := attrs[key]
	if !ok {
		return 0, fmt.Errorf("%s does not exist", key)
	}
	return strconv.ParseUint(v, 10, 64)
}

// spanContextSnapshot holds the relevant context snapshot information
// that will be encoded and decoded by encoding/gob.
type spanContextSnapshot struct {
	SpanID  appdash.SpanID
	Sampled bool
}

// PropagateSpanAsBinary returns a binary representation of an Appdash span
// using encoding/gob.
//
// The core piece of identifying information is the appdash.SpanID struct.
func (t *Tracer) PropagateSpanAsBinary(
	sp opentracing.Span,
) (
	contextSnapshot []byte,
	traceAttrs []byte,
) {
	var contextSnapshotBuffer bytes.Buffer
	s := sp.(*Span)

	snapshot := spanContextSnapshot{s.Recorder.SpanID, s.sampled}

	err := gob.NewEncoder(&contextSnapshotBuffer).Encode(snapshot)
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

// JoinTraceFromBinary starts a new child Span with an optional operationName.
// It uses the binary-encoded Span information from PropagateSpanAsBinary() as
// the new Span's parent.
func (t *Tracer) JoinTraceFromBinary(
	operationName string,
	contextSnapshot []byte,
	traceAttrs []byte,
) (
	opentracing.Span,
	error,
) {
	snapshot := spanContextSnapshot{}
	contextSnapshotBuffer := bytes.NewBuffer(contextSnapshot)
	err := gob.NewDecoder(contextSnapshotBuffer).Decode(&snapshot)
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
	span.(*Span).Recorder.SpanID = appdash.NewSpanID(snapshot.SpanID)
	span.(*Span).sampled = snapshot.Sampled

	for k, v := range traceAttrMap {
		span.SetTraceAttribute(k, v)
	}
	return span, nil
}
