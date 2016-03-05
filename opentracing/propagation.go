package opentracing

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"sourcegraph.com/sourcegraph/appdash/opentracing/pb"

	"github.com/gogo/protobuf/proto"
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

func (p *splitTextPropagator) Inject(
	sp opentracing.Span,
	carrier interface{},
) error {
	sc, ok := sp.(*Span)
	if !ok {
		return opentracing.ErrInvalidSpan
	}
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

func (p *splitTextPropagator) Join(
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

func (p *splitBinaryPropagator) Inject(
	sp opentracing.Span,
	carrier interface{},
) error {
	sc, ok := sp.(*Span)
	if !ok {
		return opentracing.ErrInvalidSpan
	}
	splitBinaryCarrier, ok := carrier.(*opentracing.SplitBinaryCarrier)
	if !ok {
		return opentracing.ErrInvalidCarrier
	}

	state := &pb.TracerState{
		TraceId: uint64(sc.Recorder.SpanID.Trace),
		SpanId:  uint64(sc.Recorder.SpanID.Span),
		Sampled: sc.sampled,
	}

	contextBytes, err := proto.Marshal(state)
	if err != nil {
		return err
	}
	splitBinaryCarrier.TracerState = contextBytes

	// Only attempt to encode the baggage if it has items.
	if len(sc.baggage) > 0 {
		sc.Lock()
		baggage := &pb.Baggage{
			Baggage: sc.baggage,
		}
		baggageBytes, err := proto.Marshal(baggage)
		sc.Unlock()
		if err != nil {
			return err
		}
		splitBinaryCarrier.Baggage = baggageBytes
	}

	return nil
}

func (p *splitBinaryPropagator) Join(
	operationName string,
	carrier interface{},
) (opentracing.Span, error) {
	splitBinaryCarrier, ok := carrier.(*opentracing.SplitBinaryCarrier)
	if !ok {
		return nil, opentracing.ErrInvalidCarrier
	}

	// Handle the trace, span ids, and sampled status.
	context := pb.TracerState{}
	if err := proto.Unmarshal(splitBinaryCarrier.TracerState, &context); err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}

	var baggageMap map[string]string
	if len(splitBinaryCarrier.Baggage) > 0 {
		// Handle the baggage.
		baggage := pb.Baggage{}
		if err := proto.Unmarshal(splitBinaryCarrier.Baggage, &baggage); err != nil {
			return nil, opentracing.ErrTraceCorrupted
		}
		baggageMap = baggage.Baggage
	} else {
		baggageMap = make(map[string]string, 0)
	}

	sp := newAppdashSpan(operationName, p.tracer)
	sp.Recorder = p.tracer.newChildRecorder(context.SpanId, context.TraceId)
	sp.sampled = context.Sampled
	sp.baggage = baggageMap
	sp.tags = make(map[string]interface{}, 0)

	return sp, nil
}

const (
	tracerStateHeaderName  = "Tracer-State"
	traceBaggageHeaderName = "Trace-Baggage"
)

func (p *goHTTPPropagator) Inject(
	sp opentracing.Span,
	carrier interface{},
) error {
	// Defer to SplitBinary for the real work.
	splitBinaryCarrier := opentracing.NewSplitBinaryCarrier()
	if err := p.splitBinaryPropagator.Inject(sp, splitBinaryCarrier); err != nil {
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

func (p *goHTTPPropagator) Join(
	operationName string,
	carrier interface{},
) (opentracing.Span, error) {
	// Decode the two base64-encoded data blobs from the HTTP header.
	header, ok := carrier.(http.Header)
	if !ok {
		return nil, opentracing.ErrInvalidCarrier
	}
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
	return p.splitBinaryPropagator.Join(operationName, splitBinaryCarrier)
}
