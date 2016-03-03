package opentracing

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"sourcegraph.com/sourcegraph/appdash/opentracing/capnproto"
	"zombiezen.com/go/capnproto2"

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

// Serialize all the things
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

	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return err
	}
	ctx, err := capnproto.NewRootTracerState(seg)
	if err != nil {
		return err
	}
	ctx.SetTraceid(uint64(sc.Recorder.SpanID.Trace))
	ctx.SetSpanid(uint64(sc.Recorder.SpanID.Span))
	ctx.SetSampled(sc.sampled)
	contextBuf := new(bytes.Buffer)
	err = capnp.NewEncoder(contextBuf).Encode(msg)
	if err != nil {
		return nil
	}

	// Baggage struct creation
	msg, seg, err = capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return err
	}
	baggage, err := capnproto.NewRootBaggage(seg)
	if err != nil {
		return err
	}

	sc.Lock()
	numItems := len(sc.baggage)
	baggageItems, err := capnproto.NewBaggage_Item_List(seg, int32(numItems))
	if err != nil {
		return err
	}

	i := 0
	for k, v := range sc.baggage {
		item, err := capnproto.NewBaggage_Item(seg)
		if err != nil {
			return err
		}
		item.SetKey(k)
		item.SetVal(v)
		baggageItems.Set(i, item)
		i++
	}
	sc.Unlock()
	baggage.SetItems(baggageItems)

	baggageBuf := new(bytes.Buffer)
	err = capnp.NewEncoder(baggageBuf).Encode(msg)

	splitBinaryCarrier.TracerState = contextBuf.Bytes()
	splitBinaryCarrier.Baggage = baggageBuf.Bytes()
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
	contextReader := bytes.NewReader(splitBinaryCarrier.TracerState)
	ctxMessage, err := capnp.NewDecoder(contextReader).Decode()
	if err != nil {
		return nil, err
	}
	ctx, err := capnproto.ReadRootTracerState(ctxMessage)
	if err != nil {
		return nil, err
	}

	baggageReader := bytes.NewReader(splitBinaryCarrier.Baggage)
	baggageMessage, err := capnp.NewDecoder(baggageReader).Decode()
	if err != nil {
		return nil, err
	}
	baggage, err := capnproto.ReadRootBaggage(baggageMessage)
	if err != nil {
		return nil, err
	}
	items, err := baggage.Items()
	if err != nil {
		return nil, err
	}
	numItems := items.Len()
	baggageMap := make(map[string]string, numItems)
	for i := 0; i < numItems; i++ {
		item := items.At(i)
		key, err := item.Key()
		if err != nil {
			return nil, err
		}
		val, err := item.Val()
		if err != nil {
			return nil, err
		}
		baggageMap[key] = val
	}

	sp := newAppdashSpan(operationName, p.tracer)
	sp.Recorder = p.tracer.newChildRecorder(ctx.Spanid(), ctx.Traceid())
	sp.sampled = ctx.Sampled()
	sp.baggage = baggageMap
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

func (p *goHTTPPropagator) JoinTrace(
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
