package dingo

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

const maxUint32 = uint64(^uint32(0))
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
   b, _ := h.Flush(0) // header section
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

func (hd *Header) Type() int16    { return hd.T }
func (hd *Header) ID() string     { return hd.I }
func (hd *Header) Name() string   { return hd.N }
func (hd *Header) Length() uint64 { return uint64(18 + 8*len(hd.R) + len(hd.N) + len(hd.I)) }

/*
Flush the header to a byte stream. Note: after flushing, all registries would be reset.
*/
func (hd *Header) Flush(prealloc uint64) ([]byte, error) {
	defer hd.Reset()

	// type(2) || total-length(8) || length Of ID(4) || count of registries(4) || registries(?) || ID(?) || name(?)
	length := hd.Length()
	b := make([]byte, length, length+prealloc)

	// type -- 2 bytes
	binary.PutVarint(b[:2], int64(hd.T))

	// total header length -- 8 bytes
	binary.PutUvarint(b[2:10], uint64(length))

	// length of ID -- 4 byte
	L := uint64(len(hd.I))
	if L >= maxUint32 {
		return nil, fmt.Errorf("length of ID exceeding max: %v", L)
	}
	binary.PutUvarint(b[10:14], L)

	// count of registries -- 4 bytes
	cntOfRegistries := uint64(len(hd.R))
	if cntOfRegistries >= maxCountOfRegistries {
		return nil, fmt.Errorf("count of registries exceeds maximum: %v", cntOfRegistries)
	}
	binary.PutUvarint(b[14:18], cntOfRegistries)

	// registries
	for i, v := range hd.R {
		binary.PutUvarint(b[18+i*8:], v)
	}

	// id
	var cur uint64 = uint64(18 + len(hd.R)*8)
	copy(b[cur:cur+L], hd.I)

	// name
	copy(b[cur+L:length], hd.N)

	return b, nil
}
func (hd *Header) Registry() []uint64 { return hd.R }
func (hd *Header) Reset()             { hd.R = []uint64{} }
func (hd *Header) Append(r uint64)    { hd.R = append(hd.R, r) }

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
	if len(b) < 18 {
		err = fmt.Errorf("length is not enough :%v", string(b))
		return
	}

	// type
	T, err := binary.ReadUvarint(bytes.NewBuffer(b[:2]))
	if err != nil {
		return
	}
	if T != 0 {
		err = fmt.Errorf("unknown type:%v", T)
		return
	}

	// total length
	L, err := binary.ReadUvarint(bytes.NewBuffer(b[2:10]))
	if err != nil {
		return
	}
	if L < 18 {
		err = fmt.Errorf("invalid header length: %v", string(b))
		return
	}

	// length of ID
	IL, err := binary.ReadUvarint(bytes.NewBuffer(b[10:14]))
	if err != nil {
		return
	}

	// count of registries
	C, err := binary.ReadUvarint(bytes.NewBuffer(b[14:18]))
	if (18 + C*8) > L {
		err = fmt.Errorf("registries count is %v, when length is %v", C, L)
		return
	}

	// registries
	Rs := []uint64{}
	var R uint64
	for i := uint64(0); i < C; i++ {
		R, err = binary.ReadUvarint(bytes.NewBuffer(b[18+i*8 : 18+(i+1)*8]))
		if err != nil {
			return
		}
		Rs = append(Rs, R)
	}

	var cur = 18 + C*8

	h = &Header{
		T: int16(T),
		I: string(b[cur : cur+IL]),
		N: string(b[cur+IL : L]),
		R: Rs,
	}
	return
}
