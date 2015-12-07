package transport

// TODO: the first 2 bytes should be the version of header format

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

type Meta interface {
	ID() string
	Name() string
}

const idLen int = 36
const maxCountOfPayloads uint64 = uint64(^uint32(0))

type Header struct {
	T int16    // header type
	I string   // dingo-generated id
	N string   // task name
	P []uint64 // offsets in bytes of each payload
}

//
// Header
//

func (me *Header) Type() int16    { return me.T }
func (me *Header) ID() string     { return me.I }
func (me *Header) Name() string   { return me.N }
func (me *Header) Length() uint64 { return uint64(50 + 8*len(me.P) + len(me.N)) }
func (me *Header) Flush() ([]byte, error) {
	if len(me.I) != idLen {
		return nil, errors.New(fmt.Sprintf("length of id should be equal to %v, not [%v]", idLen, me.I))
	}

	// type(2) || total-length(8) || id(36) || count of payload(4) || payloads(?) || name(?)
	length := me.Length()
	b := make([]byte, length)

	// type -- 4 bytes
	binary.PutVarint(b[:2], int64(me.T))

	// total header length -- 8 bytes
	binary.PutUvarint(b[2:10], uint64(length))

	// id -- 36 bytes
	copy(b[10:46], me.I)

	// count of payload -- 4 bytes
	cntOfPayloads := uint64(len(me.P))
	if cntOfPayloads >= maxCountOfPayloads {
		return nil, errors.New(fmt.Sprintf("count of payloads exceeds maximum: %v", cntOfPayloads))
	}
	binary.PutUvarint(b[46:50], cntOfPayloads)

	// payloads
	for i, v := range me.P {
		binary.PutUvarint(b[50+i*8:], v)
	}

	// name
	copy(b[50+len(me.P)*8:length], me.N)

	return b, nil
}
func (me *Header) Payloads() []uint64       { return me.P }
func (me *Header) ResetPayloads()           { me.P = []uint64{} }
func (me *Header) AddPayload(offset uint64) { me.P = append(me.P, offset) }

func NewHeader(id, name string) *Header {
	return &Header{
		T: 0,
		I: id,
		N: name,
	}
}

func DecodeHeader(b []byte) (h *Header, err error) {
	if b == nil {
		err = errors.New("nil buffer")
		return
	}
	if len(b) < 50 {
		err = errors.New(fmt.Sprintf("length is not enough :%v", string(b)))
		return
	}

	// type
	T, err := binary.ReadUvarint(bytes.NewBuffer(b[:2]))
	if err != nil {
		return
	}
	if T != 0 {
		err = errors.New(fmt.Sprintf("unknown type:%v", T))
		return
	}

	// total length
	L, err := binary.ReadUvarint(bytes.NewBuffer(b[2:10]))
	if err != nil {
		return
	}
	if L < 50 {
		err = errors.New(fmt.Sprintf("invalid header length: %v", string(b)))
		return
	}

	// count of payloads
	C, err := binary.ReadUvarint(bytes.NewBuffer(b[46:50]))
	if (50 + C*8) > L {
		err = errors.New(fmt.Sprintf("payload count is %v, when length is %v", C, L))
		return
	}

	// payloads
	Ps := []uint64{}
	var P uint64
	for i := uint64(0); i < C; i++ {
		P, err = binary.ReadUvarint(bytes.NewBuffer(b[50+i*8 : 50+(i+1)*8]))
		if err != nil {
			return
		}
		Ps = append(Ps, P)
	}

	h = &Header{
		T: int16(T),
		I: string(b[10:46]),
		N: string(b[50+C*8 : L]),
		P: Ps,
	}
	return
}
