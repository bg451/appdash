package opentracing

import (
	"bytes"
	"encoding/binary"
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

// PropagateSpanAsBinary returns a binary representation of an Appdash span
// using encoding/binary.
//
// Encodes in the order of the TraceID, SpanID, Sampled, trace attribute len
// then writes each kv pair in the order of `len_key key len_val val`
func (t *Tracer) PropagateSpanAsBinary(
	sp opentracing.Span,
) (
	contextSnapshot []byte,
	traceAttrs []byte,
) {
	s := sp.(*Span)
	traceId := uint64(s.Recorder.SpanID.Trace)
	spanId := uint64(s.Recorder.SpanID.Span)
	var sampleByte byte = 0
	if s.sampled {
		sampleByte = 1
	}

	contextBuffer := new(bytes.Buffer)

	err := binary.Write(contextBuffer, binary.BigEndian, traceId)
	if err != nil {
		panic("error encoding Trace ID")
	}

	err = binary.Write(contextBuffer, binary.BigEndian, spanId)
	if err != nil {
		panic("error encoding Trace ID")
	}

	err = binary.Write(contextBuffer, binary.BigEndian, sampleByte)
	if err != nil {
		panic("error encoding sampled bool")
	}

	attrBuffer := new(bytes.Buffer)

	numAttrs := uint32(len(s.attributes))
	err = binary.Write(attrBuffer, binary.BigEndian, numAttrs)
	if err != nil {
		panic("error encoding attribute size")
	}

	for k, v := range s.attributes {
		kBytes := []byte(k)
		err = binary.Write(attrBuffer, binary.BigEndian, int32(len(kBytes)))
		err = binary.Write(attrBuffer, binary.BigEndian, kBytes)
		vBytes := []byte(v)
		err = binary.Write(attrBuffer, binary.BigEndian, int32(len(vBytes)))
		err = binary.Write(attrBuffer, binary.BigEndian, vBytes)
	}

	return contextBuffer.Bytes(), attrBuffer.Bytes()
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
	var traceId, spanId uint64
	var sampleByte byte
	contextBuffer := bytes.NewBuffer(contextSnapshot)

	err := binary.Read(contextBuffer, binary.BigEndian, &traceId)
	if err != nil {
		return nil, err
	}
	err = binary.Read(contextBuffer, binary.BigEndian, &spanId)
	if err != nil {
		return nil, err
	}
	err = binary.Read(contextBuffer, binary.BigEndian, &sampleByte)
	if err != nil {
		return nil, err
	}

	span := t.StartTrace(operationName)
	span.(*Span).sampled = sampleByte != 0
	span.(*Span).Recorder.SpanID = appdash.NewSpanID(
		appdash.SpanID{Trace: appdash.ID(traceId), Span: appdash.ID(spanId)})

	attrBuffer := bytes.NewBuffer(traceAttrs)
	var numAttrs int32
	err = binary.Read(attrBuffer, binary.BigEndian, &numAttrs)
	if err != nil {
		return nil, err
	}

	for i := 0; i < int(numAttrs); i++ {
		var kLen, vLen int32
		err = binary.Read(attrBuffer, binary.BigEndian, &kLen)
		if err != nil {
			return nil, err
		}
		kBytes := make([]byte, kLen)
		err = binary.Read(attrBuffer, binary.BigEndian, &kBytes)
		if err != nil {
			return nil, err
		}

		err = binary.Read(attrBuffer, binary.BigEndian, &vLen)
		if err != nil {
			return nil, err
		}
		vBytes := make([]byte, vLen)
		err = binary.Read(attrBuffer, binary.BigEndian, &vBytes)
		if err != nil {
			return nil, err
		}

		span.SetTraceAttribute(string(kBytes), string(vBytes))
	}

	return span, nil
}
