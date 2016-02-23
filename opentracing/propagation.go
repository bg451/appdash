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
	splitTextCarrier.Baggage = make(map[string]string, len(sc.baggage))
	for k, v := range sc.baggage {
		splitTextCarrier.Baggage[k] = v
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
			return nil, fmt.Errorf("unknown TracerState field: %v", k)
		}
	}

	if requiredFieldCount < 3 {
		return nil, fmt.Errorf("only found %v of 3 required fields", requiredFieldCount)
	}

	sp := newAppdashSpan(operationName, p.tracer)
	sp.Recorder = p.tracer.newChildRecorder(propagatedSpanID, traceID)
	sp.sampled = sampled
	sp.baggage = splitTextCarrier.Baggage
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

	// Handle the baggage.
	baggageBuf := new(bytes.Buffer)
	err = binary.Write(baggageBuf, binary.BigEndian, int32(len(sc.baggage)))
	if err != nil {
		return err
	}

	sc.Lock()
	for k, v := range sc.baggage {
		keyBytes := []byte(k)
		lenKeyBytes := int32(len(keyBytes))
		if err := binary.Write(baggageBuf, binary.BigEndian, lenKeyBytes); err != nil {
			return err
		}
		if err := binary.Write(baggageBuf, binary.BigEndian, keyBytes); err != nil {
			return err
		}

		valBytes := []byte(v)
		lenValBytes := int32(len(valBytes))
		if err := binary.Write(baggageBuf, binary.BigEndian, lenValBytes); err != nil {
			return err
		}
		if err := binary.Write(baggageBuf, binary.BigEndian, valBytes); err != nil {
			return err
		}
	}
	sc.Unlock()

	splitBinaryCarrier.TracerState = contextBuf.Bytes()
	splitBinaryCarrier.Baggage = baggageBuf.Bytes()
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

	// Handle the baggage.
	baggageReader := bytes.NewReader(splitBinaryCarrier.Baggage)
	var numItems int32
	err = binary.Read(baggageReader, binary.BigEndian, &numItems)
	if err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}
	iNumItems := int(numItems)
	baggageMap := make(map[string]string, iNumItems)
	for i := 0; i < iNumItems; i++ {
		var keyLen int32
		err = binary.Read(baggageReader, binary.BigEndian, &keyLen)
		if err != nil {
			return nil, opentracing.ErrTraceCorrupted
		}
		keyBytes := make([]byte, keyLen)
		err = binary.Read(baggageReader, binary.BigEndian, &keyBytes)
		if err != nil {
			return nil, opentracing.ErrTraceCorrupted
		}

		var valLen int32
		err = binary.Read(baggageReader, binary.BigEndian, &valLen)
		if err != nil {
			return nil, opentracing.ErrTraceCorrupted
		}

		valBytes := make([]byte, valLen)
		err = binary.Read(baggageReader, binary.BigEndian, &valBytes)
		if err != nil {
			return nil, opentracing.ErrTraceCorrupted
		}

		baggageMap[string(keyBytes)] = string(valBytes)
	}

	sp := newAppdashSpan(operationName, p.tracer)
	sp.Recorder = p.tracer.newChildRecorder(propagatedSpanID, traceID)
	sp.baggage = baggageMap
	sp.sampled = sampledByte != 0
	sp.tags = make(map[string]interface{}, 0)

	return sp, nil
}

const (
	tracerStateHeaderName  = "Tracer-State"
	traceBaggageHeaderName = "Trace-Baggage"
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
	header.Add(traceBaggageHeaderName, base64.StdEncoding.EncodeToString(
		splitBinaryCarrier.Baggage))

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
	baggageBase64, found := header[http.CanonicalHeaderKey(traceBaggageHeaderName)]
	if !found || len(baggageBase64) == 0 {
		return nil, opentracing.ErrTraceNotFound
	}
	tracerStateBinary, err := base64.StdEncoding.DecodeString(tracerStateBase64[0])
	if err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}
	baggageBinary, err := base64.StdEncoding.DecodeString(baggageBase64[0])
	if err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}

	// Defer to SplitBinary for the real work.
	splitBinaryCarrier := &opentracing.SplitBinaryCarrier{
		TracerState: tracerStateBinary,
		Baggage:     baggageBinary,
	}
	return p.splitBinaryPropagator.JoinTrace(operationName, splitBinaryCarrier)
}
