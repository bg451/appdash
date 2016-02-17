package opentracing

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	opentracing "github.com/opentracing/opentracing-go"
)

const (
	fieldNameTraceID = "traceid"
	fieldNameSpanID  = "spanid"
	fieldNameSampled = "sampled"
)

type splitTextPropagator struct {
	tracer *Tracer
}

type splitBinaryPropagator struct {
	tracer *Tracer
}

type goHTTPPropagator struct {
	*splitBinaryPropagator
}

func (p *splitTextPropagator) InjectSpan(
	sp opentracing.Span,
	carrier interface{},
) error {
	sc := sp.(*Span)
	splitTextCarrier, ok := carrier.(*opentracing.SplitTextCarrier)
	if !ok {
		return opentracing.ErrInvalidCarrier
	}
	splitTextCarrier.TracerState = map[string]string{
		fieldNameTraceID: strconv.FormatUint(uint64(sc.Recorder.SpanID.Trace), 10),
		fieldNameSpanID:  strconv.FormatUint(uint64(sc.Recorder.SpanID.Span), 10),
		fieldNameSampled: strconv.FormatBool(sc.sampled),
	}
	sc.Lock()
	splitTextCarrier.TraceAttributes = make(map[string]string, len(sc.attributes))
	for k, v := range sc.attributes {
		splitTextCarrier.TraceAttributes[k] = v
	}
	sc.Unlock()
	return nil
}

func (p *splitTextPropagator) JoinTrace(
	operationName string,
	carrier interface{},
) (opentracing.Span, error) {
	splitTextCarrier, ok := carrier.(*opentracing.SplitTextCarrier)
	if !ok {
		return nil, opentracing.ErrInvalidCarrier
	}
	requiredFieldCount := 0
	var traceID, propagatedSpanID uint64
	var sampled bool
	var err error
	for k, v := range splitTextCarrier.TracerState {
		switch strings.ToLower(k) {
		case fieldNameTraceID:
			traceID, err = strconv.ParseUint(v, 10, 64)
			if err != nil {
				return nil, opentracing.ErrTraceCorrupted
			}
			requiredFieldCount++
		case fieldNameSpanID:
			propagatedSpanID, err = strconv.ParseUint(v, 10, 64)
			if err != nil {
				return nil, opentracing.ErrTraceCorrupted
			}
			requiredFieldCount++
		case fieldNameSampled:
			sampled, err = strconv.ParseBool(v)
			if err != nil {
				return nil, opentracing.ErrTraceCorrupted
			}
			requiredFieldCount++
		default:
			return nil, fmt.Errorf("Unknown TracerState field: %v", k)
		}
	}

	if requiredFieldCount < 3 {
		return nil, fmt.Errorf("Only found %v of 3 required fields", requiredFieldCount)
	}

	sp := newAppdashSpan(operationName, p.tracer)
	sp.Recorder = p.tracer.newChildRecorder(propagatedSpanID, traceID)
	sp.sampled = sampled
	sp.attributes = splitTextCarrier.TraceAttributes
	sp.tags = make(map[string]interface{}, 0)

	return sp, nil
}

func (p *splitBinaryPropagator) InjectSpan(
	sp opentracing.Span,
	carrier interface{},
) error {
	sc := sp.(*Span)
	splitBinaryCarrier, ok := carrier.(*opentracing.SplitBinaryCarrier)
	if !ok {
		return opentracing.ErrInvalidCarrier
	}
	var err error
	var sampledByte byte
	if sc.sampled {
		sampledByte = 1
	}

	// Handle the trace and span ids, and sampled status.
	contextBuf := new(bytes.Buffer)
	err = binary.Write(contextBuf, binary.BigEndian, uint64(sc.Recorder.SpanID.Trace))
	if err != nil {
		return err
	}

	err = binary.Write(contextBuf, binary.BigEndian, uint64(sc.Recorder.SpanID.Span))
	if err != nil {
		return err
	}

	err = binary.Write(contextBuf, binary.BigEndian, sampledByte)
	if err != nil {
		return err
	}

	// Handle the attributes.
	attrsBuf := new(bytes.Buffer)
	err = binary.Write(attrsBuf, binary.BigEndian, int32(len(sc.attributes)))
	if err != nil {
		return err
	}

	sc.Lock()
	for k, v := range sc.attributes {
		keyBytes := []byte(k)
		lenKeyBytes := int32(len(keyBytes))
		if err := binary.Write(attrsBuf, binary.BigEndian, lenKeyBytes); err != nil {
			return err
		}
		if err := binary.Write(attrsBuf, binary.BigEndian, keyBytes); err != nil {
			return err
		}

		valBytes := []byte(v)
		lenValBytes := int32(len(valBytes))
		if err := binary.Write(attrsBuf, binary.BigEndian, lenValBytes); err != nil {
			return err
		}
		if err := binary.Write(attrsBuf, binary.BigEndian, valBytes); err != nil {
			return err
		}
	}
	sc.Unlock()

	splitBinaryCarrier.TracerState = contextBuf.Bytes()
	splitBinaryCarrier.TraceAttributes = attrsBuf.Bytes()
	return nil
}

func (p *splitBinaryPropagator) JoinTrace(
	operationName string,
	carrier interface{},
) (opentracing.Span, error) {
	var err error
	splitBinaryCarrier, ok := carrier.(*opentracing.SplitBinaryCarrier)
	if !ok {
		return nil, opentracing.ErrInvalidCarrier
	}

	// Handle the trace, span ids, and sampled status.
	contextReader := bytes.NewReader(splitBinaryCarrier.TracerState)
	var traceID, propagatedSpanID uint64
	var sampledByte byte

	err = binary.Read(contextReader, binary.BigEndian, &traceID)
	if err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}
	err = binary.Read(contextReader, binary.BigEndian, &propagatedSpanID)
	if err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}
	err = binary.Read(contextReader, binary.BigEndian, &sampledByte)
	if err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}

	// Handle the attributes.
	attrsReader := bytes.NewReader(splitBinaryCarrier.TraceAttributes)
	var numAttrs int32
	err = binary.Read(attrsReader, binary.BigEndian, &numAttrs)
	if err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}
	iNumAttrs := int(numAttrs)
	attrMap := make(map[string]string, iNumAttrs)
	for i := 0; i < iNumAttrs; i++ {
		var keyLen int32
		err = binary.Read(attrsReader, binary.BigEndian, &keyLen)
		if err != nil {
			return nil, opentracing.ErrTraceCorrupted
		}
		keyBytes := make([]byte, keyLen)
		err = binary.Read(attrsReader, binary.BigEndian, &keyBytes)
		if err != nil {
			return nil, opentracing.ErrTraceCorrupted
		}

		var valLen int32
		err = binary.Read(attrsReader, binary.BigEndian, &valLen)
		if err != nil {
			return nil, opentracing.ErrTraceCorrupted
		}

		valBytes := make([]byte, valLen)
		err = binary.Read(attrsReader, binary.BigEndian, &valBytes)
		if err != nil {
			return nil, opentracing.ErrTraceCorrupted
		}

		attrMap[string(keyBytes)] = string(valBytes)
	}

	sp := newAppdashSpan(operationName, p.tracer)
	sp.Recorder = p.tracer.newChildRecorder(propagatedSpanID, traceID)
	sp.attributes = attrMap
	sp.sampled = sampledByte != 0
	sp.tags = make(map[string]interface{}, 0)

	return sp, nil
}

const (
	tracerStateHeaderName = "Tracer-State"
	traceAttrsHeaderName  = "Trace-Attributes"
)

func (p *goHTTPPropagator) InjectSpan(
	sp opentracing.Span,
	carrier interface{},
) error {
	// Defer to SplitBinary for the real work.
	splitBinaryCarrier := opentracing.NewSplitBinaryCarrier()
	if err := p.splitBinaryPropagator.InjectSpan(sp, splitBinaryCarrier); err != nil {
		return err
	}

	// Encode into the HTTP header as two base64 strings.
	header := carrier.(http.Header)
	header.Add(tracerStateHeaderName, base64.StdEncoding.EncodeToString(
		splitBinaryCarrier.TracerState))
	header.Add(traceAttrsHeaderName, base64.StdEncoding.EncodeToString(
		splitBinaryCarrier.TraceAttributes))

	return nil
}

func (p *goHTTPPropagator) JoinTrace(
	operationName string,
	carrier interface{},
) (opentracing.Span, error) {
	// Decode the two base64-encoded data blobs from the HTTP header.
	header := carrier.(http.Header)
	tracerStateBase64, found := header[http.CanonicalHeaderKey(tracerStateHeaderName)]
	if !found || len(tracerStateBase64) == 0 {
		return nil, opentracing.ErrTraceNotFound
	}
	traceAttrsBase64, found := header[http.CanonicalHeaderKey(traceAttrsHeaderName)]
	if !found || len(traceAttrsBase64) == 0 {
		return nil, opentracing.ErrTraceNotFound
	}
	tracerStateBinary, err := base64.StdEncoding.DecodeString(tracerStateBase64[0])
	if err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}
	traceAttrsBinary, err := base64.StdEncoding.DecodeString(traceAttrsBase64[0])
	if err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}

	// Defer to SplitBinary for the real work.
	splitBinaryCarrier := &opentracing.SplitBinaryCarrier{
		TracerState:     tracerStateBinary,
		TraceAttributes: traceAttrsBinary,
	}
	return p.splitBinaryPropagator.JoinTrace(operationName, splitBinaryCarrier)
}
