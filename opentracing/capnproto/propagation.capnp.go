package capnproto

// AUTO GENERATED - DO NOT EDIT

import (
	capnp "zombiezen.com/go/capnproto2"
)

type TracerState struct{ capnp.Struct }

func NewTracerState(s *capnp.Segment) (TracerState, error) {
	st, err := capnp.NewStruct(s, capnp.ObjectSize{DataSize: 24, PointerCount: 0})
	if err != nil {
		return TracerState{}, err
	}
	return TracerState{st}, nil
}

func NewRootTracerState(s *capnp.Segment) (TracerState, error) {
	st, err := capnp.NewRootStruct(s, capnp.ObjectSize{DataSize: 24, PointerCount: 0})
	if err != nil {
		return TracerState{}, err
	}
	return TracerState{st}, nil
}

func ReadRootTracerState(msg *capnp.Message) (TracerState, error) {
	root, err := msg.Root()
	if err != nil {
		return TracerState{}, err
	}
	st := capnp.ToStruct(root)
	return TracerState{st}, nil
}

func (s TracerState) Traceid() uint64 {
	return s.Struct.Uint64(0)
}

func (s TracerState) SetTraceid(v uint64) {

	s.Struct.SetUint64(0, v)
}

func (s TracerState) Spanid() uint64 {
	return s.Struct.Uint64(8)
}

func (s TracerState) SetSpanid(v uint64) {

	s.Struct.SetUint64(8, v)
}

func (s TracerState) Sampled() bool {
	return s.Struct.Bit(128)
}

func (s TracerState) SetSampled(v bool) {

	s.Struct.SetBit(128, v)
}

// TracerState_List is a list of TracerState.
type TracerState_List struct{ capnp.List }

// NewTracerState creates a new list of TracerState.
func NewTracerState_List(s *capnp.Segment, sz int32) (TracerState_List, error) {
	l, err := capnp.NewCompositeList(s, capnp.ObjectSize{DataSize: 24, PointerCount: 0}, sz)
	if err != nil {
		return TracerState_List{}, err
	}
	return TracerState_List{l}, nil
}

func (s TracerState_List) At(i int) TracerState           { return TracerState{s.List.Struct(i)} }
func (s TracerState_List) Set(i int, v TracerState) error { return s.List.SetStruct(i, v.Struct) }

// TracerState_Promise is a wrapper for a TracerState promised by a client call.
type TracerState_Promise struct{ *capnp.Pipeline }

func (p TracerState_Promise) Struct() (TracerState, error) {
	s, err := p.Pipeline.Struct()
	return TracerState{s}, err
}

type Baggage struct{ capnp.Struct }

func NewBaggage(s *capnp.Segment) (Baggage, error) {
	st, err := capnp.NewStruct(s, capnp.ObjectSize{DataSize: 0, PointerCount: 1})
	if err != nil {
		return Baggage{}, err
	}
	return Baggage{st}, nil
}

func NewRootBaggage(s *capnp.Segment) (Baggage, error) {
	st, err := capnp.NewRootStruct(s, capnp.ObjectSize{DataSize: 0, PointerCount: 1})
	if err != nil {
		return Baggage{}, err
	}
	return Baggage{st}, nil
}

func ReadRootBaggage(msg *capnp.Message) (Baggage, error) {
	root, err := msg.Root()
	if err != nil {
		return Baggage{}, err
	}
	st := capnp.ToStruct(root)
	return Baggage{st}, nil
}

func (s Baggage) Items() (Baggage_Item_List, error) {
	p, err := s.Struct.Pointer(0)
	if err != nil {
		return Baggage_Item_List{}, err
	}

	l := capnp.ToList(p)

	return Baggage_Item_List{List: l}, nil
}

func (s Baggage) SetItems(v Baggage_Item_List) error {

	return s.Struct.SetPointer(0, v.List)
}

// Baggage_List is a list of Baggage.
type Baggage_List struct{ capnp.List }

// NewBaggage creates a new list of Baggage.
func NewBaggage_List(s *capnp.Segment, sz int32) (Baggage_List, error) {
	l, err := capnp.NewCompositeList(s, capnp.ObjectSize{DataSize: 0, PointerCount: 1}, sz)
	if err != nil {
		return Baggage_List{}, err
	}
	return Baggage_List{l}, nil
}

func (s Baggage_List) At(i int) Baggage           { return Baggage{s.List.Struct(i)} }
func (s Baggage_List) Set(i int, v Baggage) error { return s.List.SetStruct(i, v.Struct) }

// Baggage_Promise is a wrapper for a Baggage promised by a client call.
type Baggage_Promise struct{ *capnp.Pipeline }

func (p Baggage_Promise) Struct() (Baggage, error) {
	s, err := p.Pipeline.Struct()
	return Baggage{s}, err
}

type Baggage_Item struct{ capnp.Struct }

func NewBaggage_Item(s *capnp.Segment) (Baggage_Item, error) {
	st, err := capnp.NewStruct(s, capnp.ObjectSize{DataSize: 0, PointerCount: 2})
	if err != nil {
		return Baggage_Item{}, err
	}
	return Baggage_Item{st}, nil
}

func NewRootBaggage_Item(s *capnp.Segment) (Baggage_Item, error) {
	st, err := capnp.NewRootStruct(s, capnp.ObjectSize{DataSize: 0, PointerCount: 2})
	if err != nil {
		return Baggage_Item{}, err
	}
	return Baggage_Item{st}, nil
}

func ReadRootBaggage_Item(msg *capnp.Message) (Baggage_Item, error) {
	root, err := msg.Root()
	if err != nil {
		return Baggage_Item{}, err
	}
	st := capnp.ToStruct(root)
	return Baggage_Item{st}, nil
}

func (s Baggage_Item) Key() (string, error) {
	p, err := s.Struct.Pointer(0)
	if err != nil {
		return "", err
	}

	return capnp.ToText(p), nil

}

func (s Baggage_Item) KeyBytes() ([]byte, error) {
	p, err := s.Struct.Pointer(0)
	if err != nil {
		return nil, err
	}
	return capnp.ToData(p), nil
}

func (s Baggage_Item) SetKey(v string) error {

	t, err := capnp.NewText(s.Struct.Segment(), v)
	if err != nil {
		return err
	}
	return s.Struct.SetPointer(0, t)
}

func (s Baggage_Item) Val() (string, error) {
	p, err := s.Struct.Pointer(1)
	if err != nil {
		return "", err
	}

	return capnp.ToText(p), nil

}

func (s Baggage_Item) ValBytes() ([]byte, error) {
	p, err := s.Struct.Pointer(1)
	if err != nil {
		return nil, err
	}
	return capnp.ToData(p), nil
}

func (s Baggage_Item) SetVal(v string) error {

	t, err := capnp.NewText(s.Struct.Segment(), v)
	if err != nil {
		return err
	}
	return s.Struct.SetPointer(1, t)
}

// Baggage_Item_List is a list of Baggage_Item.
type Baggage_Item_List struct{ capnp.List }

// NewBaggage_Item creates a new list of Baggage_Item.
func NewBaggage_Item_List(s *capnp.Segment, sz int32) (Baggage_Item_List, error) {
	l, err := capnp.NewCompositeList(s, capnp.ObjectSize{DataSize: 0, PointerCount: 2}, sz)
	if err != nil {
		return Baggage_Item_List{}, err
	}
	return Baggage_Item_List{l}, nil
}

func (s Baggage_Item_List) At(i int) Baggage_Item           { return Baggage_Item{s.List.Struct(i)} }
func (s Baggage_Item_List) Set(i int, v Baggage_Item) error { return s.List.SetStruct(i, v.Struct) }

// Baggage_Item_Promise is a wrapper for a Baggage_Item promised by a client call.
type Baggage_Item_Promise struct{ *capnp.Pipeline }

func (p Baggage_Item_Promise) Struct() (Baggage_Item, error) {
	s, err := p.Pipeline.Struct()
	return Baggage_Item{s}, err
}
