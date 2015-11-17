package transport

// TODO: the first 2 bytes should be the version of header format

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

const VERSION uint16 = 0

type Header struct {
	L  uint16 // length of header
	M  int16  // id of marshaller
	NL uint16 // length of name
	N  string // name of task/report
	I  string // dingo-generated id
	V  uint16 // version of header
}

func (me *Header) Version() uint16 { return me.V }
func (me *Header) Length() uint16  { return me.L }
func (me *Header) ID() string      { return me.I }
func (me *Header) Name() string    { return me.N }
func (me *Header) MashID() int16   { return me.M }

type Meta interface {
	ID() string
	Name() string
}

func DecodeHeader(b []byte) (m *Header, err error) {
	if b == nil {
		err = errors.New("nil buffer")
		return
	}
	if len(b) < 64 {
		err = errors.New(fmt.Sprintf("length is not enough :%v", string(b)))
		return
	}
	V, err := binary.ReadUvarint(bytes.NewBuffer(b[:16]))
	if err != nil {
		return
	}
	if V != uint64(VERSION) {
		err = errors.New(fmt.Sprintf("version mismatch:%v", V))
		return
	}

	L, err := binary.ReadUvarint(bytes.NewBuffer(b[16:32]))
	if err != nil {
		return
	}
	if L < 64 {
		err = errors.New(fmt.Sprintf("invalid header length: %v", string(b)))
		return
	}
	M, err := binary.ReadVarint(bytes.NewBuffer(b[32:48]))
	if err != nil {
		return
	}
	NL, err := binary.ReadUvarint(bytes.NewBuffer(b[48:64]))
	if err != nil {
		return
	}
	return &Header{
		L:  uint16(L),
		I:  string(b[64+NL : L]),
		NL: uint16(NL),
		N:  string(b[64 : 64+NL]),
		M:  int16(M),
	}, nil
}

func EncodeHeader(id string, name string, mashID int16) (b []byte) {
	nl := len(name)
	length := 64 + nl + len(id)
	b = make([]byte, length)
	binary.PutUvarint(b[:16], uint64(VERSION))
	binary.PutUvarint(b[16:32], uint64(length))
	binary.PutVarint(b[32:48], int64(mashID))
	binary.PutUvarint(b[48:64], uint64(nl))
	copy(b[64:64+nl], name)
	copy(b[64+nl:], id)
	return
}
