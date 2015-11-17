package transport

// TODO: the first 2 bytes should be the version of header format

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

type Header struct {
	L  uint16 // length of header
	M  int16  // id of marshaller
	NL uint16 // length of name
	N  string // name of task/report
	I  string // dingo-generated id
}

func (me *Header) Length() uint16 { return me.L }
func (me *Header) ID() string     { return me.I }
func (me *Header) Name() string   { return me.N }
func (me *Header) MashID() int16  { return me.M }

type Meta interface {
	ID() string
	Name() string
}

func DecodeHeader(b []byte) (m *Header, err error) {
	if b == nil {
		err = errors.New("nil buffer")
		return
	}
	if len(b) < 48 {
		err = errors.New(fmt.Sprintf("length is not enough :%v", string(b)))
		return
	}
	L, err := binary.ReadUvarint(bytes.NewBuffer(b[:16]))
	if err != nil {
		return
	}
	if L < 48 {
		err = errors.New(fmt.Sprintf("invalid header length: %v", string(b)))
		return
	}
	M, err := binary.ReadVarint(bytes.NewBuffer(b[16:32]))
	if err != nil {
		return
	}
	NL, err := binary.ReadUvarint(bytes.NewBuffer(b[32:48]))
	if err != nil {
		return
	}
	return &Header{
		L:  uint16(L),
		I:  string(b[48+NL : L]),
		NL: uint16(NL),
		N:  string(b[48 : 48+NL]),
		M:  int16(M),
	}, nil
}

func EncodeHeader(id string, name string, mashID int16) (b []byte) {
	nl := len(name)
	length := 48 + nl + len(id)
	b = make([]byte, length)
	binary.PutUvarint(b[:16], uint64(length))
	binary.PutVarint(b[16:32], int64(mashID))
	binary.PutUvarint(b[32:48], uint64(nl))
	copy(b[48:48+nl], name)
	copy(b[48+nl:], id)
	return
}
