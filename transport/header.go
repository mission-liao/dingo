package transport

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
const maxCountOfRegistries uint64 = uint64(^uint32(0))

/*
 The Common header section of the byte stream marshalled from Marshaller(s),
 external components(broker.Producer, broker.Consumer, backend.Reporter, backend.Store) could rely
 on Header to get some identity info from the byte stream they have, like this:

   h, err := DecodeHeader(b)
   // the id of task
   h.ID()
   // the name of task
   h.Name()

 Registries could be added to Header. For example, if your Marshaller encodes each argument
 in different byte streams, you could record their lengths(in byte) in registries section
 in Header:

   // marshalling
   bs := [][]byte{}
   h := task.H
   for _, v := range args {
     b_, _ := json.Marshal(v)
	 h.Append(uint64(len(b_)))
	 bs = append(bs, b_)
   }

   // compose those byte streams
   b, _ := h.Flush() // header section
   for _, v := range bs {
	   b = append(b, v)
   }

   // unmarshalling
   h, _ := DecodeHeader(b)
   for _, v := range h.Registry() {
     // you could rely on registry to decompose
     // the byte stream here.
   }
*/
type Header struct {
	// header type, "dingo" would raise an error when encountering headers with
	// unknown types.
	T int16

	// dingo-generated id for this task
	I string

	// task name
	N string

	// registries(a serious of uint64), their usage depends on Marshaller(s).
	R []uint64
}

func (me *Header) Type() int16    { return me.T }
func (me *Header) ID() string     { return me.I }
func (me *Header) Name() string   { return me.N }
func (me *Header) Length() uint64 { return uint64(50 + 8*len(me.R) + len(me.N)) }

/*
Flush the header to a byte stream. Note: after flushing, all registries would be reset.
*/
func (me *Header) Flush() ([]byte, error) {
	// TODO: pre allocate size for []byte
	defer me.Reset()

	if len(me.I) != idLen {
		return nil, errors.New(fmt.Sprintf("length of id should be equal to %v, not [%v]", idLen, me.I))
	}

	// type(2) || total-length(8) || id(36) || count of registries(4) || registries(?) || name(?)
	length := me.Length()
	b := make([]byte, length)

	// type -- 4 bytes
	binary.PutVarint(b[:2], int64(me.T))

	// total header length -- 8 bytes
	binary.PutUvarint(b[2:10], uint64(length))

	// id -- 36 bytes
	copy(b[10:46], me.I)

	// count of registries -- 4 bytes
	cntOfRegistries := uint64(len(me.R))
	if cntOfRegistries >= maxCountOfRegistries {
		return nil, errors.New(fmt.Sprintf("count of registries exceeds maximum: %v", cntOfRegistries))
	}
	binary.PutUvarint(b[46:50], cntOfRegistries)

	// registries
	for i, v := range me.R {
		binary.PutUvarint(b[50+i*8:], v)
	}

	// name
	copy(b[50+len(me.R)*8:length], me.N)

	return b, nil
}
func (me *Header) Registry() []uint64 { return me.R }
func (me *Header) Reset()             { me.R = []uint64{} }
func (me *Header) Append(r uint64)    { me.R = append(me.R, r) }

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

	// count of registries
	C, err := binary.ReadUvarint(bytes.NewBuffer(b[46:50]))
	if (50 + C*8) > L {
		err = errors.New(fmt.Sprintf("registries count is %v, when length is %v", C, L))
		return
	}

	// registries
	Rs := []uint64{}
	var R uint64
	for i := uint64(0); i < C; i++ {
		R, err = binary.ReadUvarint(bytes.NewBuffer(b[50+i*8 : 50+(i+1)*8]))
		if err != nil {
			return
		}
		Rs = append(Rs, R)
	}

	h = &Header{
		T: int16(T),
		I: string(b[10:46]),
		N: string(b[50+C*8 : L]),
		R: Rs,
	}
	return
}
