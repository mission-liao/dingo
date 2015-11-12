package transport

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

type Header struct {
	L uint16 // length of header
	M int16  // id of marshaller
	I string // dingo-generated id
}

func (me *Header) Length() uint16 { return me.L }
func (me *Header) ID() string     { return me.I }
func (me *Header) MashID() int16  { return me.M }

type Meta interface {
	ID() string
}

func DecodeHeader(b []byte) (m *Header, err error) {
	if b == nil {
		err = errors.New("nil buffer")
		return
	}
	if len(b) < 32 {
		err = errors.New(fmt.Sprintf("length is not enough :%v", string(b)))
		return
	}
	L, err := binary.ReadUvarint(bytes.NewBuffer(b[:16]))
	if err != nil {
		return
	}
	if L < 32 {
		err = errors.New(fmt.Sprintf("invalid header length: %v", string(b)))
		return
	}
	M, err := binary.ReadVarint(bytes.NewBuffer(b[16:32]))
	if err != nil {
		return
	}
	return &Header{
		L: uint16(L),
		I: string(b[32:L]),
		M: int16(M),
	}, nil
}

func EncodeHeader(id string, mashID int16) (b []byte) {
	length := 32 + len(id)
	b = make([]byte, length)
	binary.PutUvarint(b[:16], uint64(length))
	binary.PutVarint(b[16:32], int64(mashID))
	copy(b[32:], id)
	return
}
