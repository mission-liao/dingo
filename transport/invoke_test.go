package transport

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

// simulate users passing a list of parameters,
// marshaling/unmarshaling by JSON, finally becoming
// a list of interface{}
func ioJSON(v ...interface{}) ([]interface{}, error) {
	var ret []interface{}

	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// simulate Gob encoding over the wire
func ioGOB(v ...interface{}) ([]interface{}, error) {
	var (
		wire bytes.Buffer
		ret  []interface{}
	)
	enc := gob.NewEncoder(&wire)
	dec := gob.NewDecoder(&wire)

	err := enc.Encode(v)
	if err != nil {
		return nil, err
	}

	err = dec.Decode(&ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

//
// struct
//
type Struct struct {
	Name  string
	Count int
}

// struct with private field
type _struct_private struct {
	Struct
	private int
}

//
// embed struct
//
type _embed struct {
	Struct
	Age  int
	Addr string
}

type _embed_with_collision struct {
	Struct
	Age  int
	Addr string

	// collision, named with tag
	Name  string `json:"out.Name"`
	Count int    `json:"out.Count,omitempty"`

	// struct pointer
	S *Struct
}

//
// testing suite
//
type InvokeTestSuite struct {
	suite.Suite

	ivk     Invoker
	convert func(...interface{}) ([]interface{}, error)
}

//
// json
//

type InvokeJsonTestSuite struct {
	InvokeTestSuite
}

func TestInvokeJsonSuite(t *testing.T) {
	suite.Run(t, &InvokeJsonTestSuite{
		InvokeTestSuite{
			ivk:     NewDefaultInvoker(),
			convert: ioJSON,
		},
	})
}

func (s *InvokeJsonTestSuite) TestMap2Struct() {
	s.InvokeTestSuite._testMap2Struct(map[string]*Struct{
		"a": &Struct{Name: "Mary", Count: 11},
		"b": &Struct{Name: "Bob", Count: 10},
		"c": &Struct{Name: "Tom", Count: 12},
		"d": nil,
	})
}

func (s *InvokeJsonTestSuite) TestSliceOfStruct() {
	s.InvokeTestSuite._testSliceOfStruct([]*Struct{
		&Struct{Name: "Mary", Count: 11},
		&Struct{Name: "Bob", Count: 10},
		&Struct{Name: "Tom", Count: 12},
		nil,
	})
}

//
// gob
//

type InvokeGobTestSuite struct {
	InvokeTestSuite
}

func TestInvokeGobSuite(t *testing.T) {
	gob.Register(map[string]int{})
	gob.Register(_embed{})
	gob.Register(_embed_with_collision{})
	gob.Register(Struct{})
	gob.Register(map[string]*Struct{})
	gob.Register([]*Struct{})

	suite.Run(t, &InvokeGobTestSuite{
		InvokeTestSuite{
			ivk:     NewDefaultInvoker(),
			convert: ioGOB,
		},
	})
}

func (s *InvokeGobTestSuite) TestMap2Struct() {
	s.InvokeTestSuite._testMap2Struct(map[string]*Struct{
		"a": &Struct{Name: "Mary", Count: 11},
		"b": &Struct{Name: "Bob", Count: 10},
		"c": &Struct{Name: "Tom", Count: 12},
		"d": &Struct{}, // gob didn't allow nil value as struct
	})
}

func (s *InvokeGobTestSuite) TestSliceOfStruct() {
	s.InvokeTestSuite._testSliceOfStruct([]*Struct{
		&Struct{Name: "Mary", Count: 11},
		&Struct{Name: "Bob", Count: 10},
		&Struct{Name: "Tom", Count: 12},
		&Struct{},
	})
}

//
// test cases
//

func (s *InvokeTestSuite) TestFloat64() {
	// float64
	{
		called := float64(0.0)
		chk := func(v float64) (float64, error) {
			called = v
			return v, nil
		}

		param, err := s.convert(float64(1.0))
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Invoke(chk, param)
			s.Nil(err)
			s.Len(ret, 2)

			if ret != nil && len(ret) > 0 {
				f, ok := ret[0].(float64)
				s.True(ok)
				if ok {
					s.Equal(float64(1.0), f)
				}
			}

			s.Equal(float64(1.0), called)
		}
	}

	// TODO: *float64
	{
	}
}

func (s *InvokeTestSuite) TestInt64() {
	// int64
	{
		called := int64(0)
		chk := func(v int64) (int64, error) {
			called = v
			return v, nil
		}
		param, err := s.convert(int64(1))
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Invoke(chk, param)
			s.Nil(err)
			s.Len(ret, 2)
			if ret != nil && len(ret) > 0 {
				f, ok := ret[0].(int64)
				s.True(ok)
				if ok {
					s.Equal(int64(1), f)
				}
			}

			s.Equal(int64(1), called)
		}
	}
}

func (s *InvokeTestSuite) TestInt() {
	// int
	{
		called := int(0)
		chk := func(v int) (int, error) {
			called = v
			return v, nil
		}
		param, err := s.convert(int(1))
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Invoke(chk, param)
			s.Nil(err)
			s.Len(ret, 2)
			if ret != nil && len(ret) > 0 {
				f, ok := ret[0].(int)
				s.True(ok)
				if ok {
					s.Equal(int(1), f)
				}
			}

			s.Equal(int(1), called)
		}
	}
}

func (s *InvokeTestSuite) TestString() {
	{
		called := ""
		chk := func(v string) (string, error) {
			called = v
			return v, nil
		}
		param, err := s.convert("test string")
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Invoke(chk, param)
			s.Nil(err)
			s.Len(ret, 2)
			if ret != nil && len(ret) > 0 {
				f, ok := ret[0].(string)
				s.True(ok)
				if ok {
					s.Equal("test string", f)
				}
			}

			s.Equal("test string", called)
		}
	}
}

func (s *InvokeTestSuite) TestStruct() {
	// *Struct
	{
		called := (*Struct)(nil)
		chk := func(v *Struct) (*Struct, error) {
			called = v
			return v, nil
		}
		v := &Struct{Name: "Bob", Count: 10}
		param, err := s.convert(v)
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Invoke(chk, param)
			s.Nil(err)
			s.Len(ret, 2)
			if ret != nil && len(ret) > 0 {
				f, ok := ret[0].(*Struct)
				s.True(ok)
				if ok {
					s.Equal(v, f)
				}
			}

			s.Equal(called, v)
		}
	}

	// **Struct
	{
		called := (**Struct)(nil)
		name := ""
		count := 0
		chk := func(v **Struct) (**Struct, error) {
			called = v
			if v != nil && *v != nil {
				name = (**v).Name
				count = (**v).Count
			}
			// access its element
			return v, nil
		}
		{
			v_ := &Struct{Name: "Bob", Count: 10}
			// pointer-2-pointer
			v := &v_
			param, err := s.convert(v)
			s.Nil(err)
			s.NotNil(param)
			if param != nil {
				ret, err := s.ivk.Invoke(chk, param)
				s.Nil(err)
				s.Len(ret, 2)
				if ret != nil && len(ret) > 0 {
					f, ok := ret[0].(**Struct)
					s.True(ok)
					if ok {
						s.Equal(v, f)
					}
				}

				s.Equal(v, called)
				s.Equal("Bob", name)
				s.Equal(10, count)
			}
		}

		// calling with nil
		{
			param, err := s.convert(nil)
			s.Nil(err)
			s.NotNil(param)
			if param != nil {
				ret, err := s.ivk.Invoke(chk, param)
				s.Nil(err)
				s.Len(ret, 2)
				if ret != nil && len(ret) > 0 {
					f, ok := ret[0].(**Struct)
					s.True(ok)
					if ok {
						s.Equal((**Struct)(nil), f)
					}
				}
			}
		}
	}
}

func (s *InvokeTestSuite) TestEmbed() {
	// *_embed
	{
		called := (*_embed)(nil)
		age, count, name, address := 0, 0, "", ""
		chk := func(v *_embed) (*_embed, error) {
			called = v
			if v != nil {
				name = v.Name
				age = v.Age
				count = v.Count
				address = v.Addr
			}
			return v, nil
		}
		v := &_embed{Struct: Struct{Name: "Bob", Count: 10}, Age: 100, Addr: "heaven"}
		param, err := s.convert(v)
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Invoke(chk, param)
			s.Nil(err)
			s.Len(ret, 2)
			if ret != nil && len(ret) > 0 {
				f, ok := ret[0].(*_embed)
				s.True(ok)
				if ok {
					s.Equal(v, f)
				}
			}

			s.Equal(v, called)
			s.Equal(100, age)
			s.Equal(10, count)
			s.Equal("Bob", name)
			s.Equal("heaven", address)
		}
	}
}

func (s *InvokeTestSuite) TestEmbedWithCollision() {
	// *_embed_with_collision
	{
		called := (*_embed_with_collision)(nil)
		age, count, name, address, count_, name_, count_s, name_s := 0, 0, "", "", 0, "", 0, ""
		chk := func(v *_embed_with_collision) (*_embed_with_collision, error) {
			called = v
			if v != nil {
				name = v.Name
				age = v.Age
				count = v.Count
				address = v.Addr
				name_ = v.Struct.Name
				count_ = v.Struct.Count
				name_s = v.S.Name
				count_s = v.S.Count
			}
			return v, nil
		}
		v := &_embed_with_collision{
			Struct: Struct{Name: "Bob", Count: 10},
			Age:    100,
			Addr:   "heaven",
			Name:   "Mary",
			Count:  11,
			S:      &Struct{Name: "Tom", Count: 12},
		}
		param, err := s.convert(v)
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Invoke(chk, param)
			s.Nil(err)
			s.Len(ret, 2)
			if ret != nil && len(ret) > 0 {
				f, ok := ret[0].(*_embed_with_collision)
				s.True(ok)
				if ok {
					s.Equal(v, f)
				}
			}

			s.Equal(v, called)
			s.Equal(100, age)
			s.Equal(10, count_)
			s.Equal("Bob", name_)
			s.Equal("heaven", address)
			s.Equal("Mary", name)
			s.Equal(11, count)
			s.Equal("Tom", name_s)
			s.Equal(12, count_s)
		}
	}
}

func (s *InvokeTestSuite) TestMap() {
	// map[string]int
	var called map[string]int
	chk := func(v map[string]int) (map[string]int, error) {
		called = v
		return v, nil
	}
	v := map[string]int{"Tom": 12, "Bob": 10, "Mary": 11}
	param, err := s.convert(v)
	s.Nil(err)
	s.NotNil(param)
	if param != nil {
		ret, err := s.ivk.Invoke(chk, param)
		s.Nil(err)
		s.Len(ret, 2)
		if ret != nil && len(ret) > 0 {
			f, ok := ret[0].(map[string]int)
			s.True(ok)
			if ok {
				s.Equal(v, f)
			}
		}

		s.Equal(v, called)
	}
}

func (s *InvokeTestSuite) _testMap2Struct(v map[string]*Struct) {
	// map[string]*Struct
	var called map[string]*Struct
	name, count := "", 0
	chk := func(vv map[string]*Struct) (map[string]*Struct, error) {
		called = vv
		name = vv["a"].Name
		count = vv["a"].Count
		if v["d"] == nil {
			s.Nil(vv["d"])
		}
		return vv, nil
	}

	param, err := s.convert(v)
	s.Nil(err)
	s.NotNil(param)
	if param != nil {
		ret, err := s.ivk.Invoke(chk, param)
		s.Nil(err)
		s.Len(ret, 2)
		if ret != nil && len(ret) > 0 {
			f, ok := ret[0].(map[string]*Struct)
			s.True(ok)
			if ok {
				s.Equal(v, f)
			}
		}

		s.Equal(v, called)
		s.Equal(v["a"].Name, name)
		s.Equal(v["a"].Count, count)
	}

}

func (s *InvokeTestSuite) TestSlice() {
	// []string
	var called []string
	chk := func(v []string) ([]string, error) {
		called = v
		return v, nil
	}
	v := []string{"Tom", "Bob", "Mary"}
	param, err := s.convert(v)
	s.Nil(err)
	s.NotNil(param)
	if param != nil {
		ret, err := s.ivk.Invoke(chk, param)
		s.Nil(err)
		s.Len(ret, 2)
		if ret != nil && len(ret) > 0 {
			f, ok := ret[0].([]string)
			s.True(ok)
			if ok {
				s.Equal(v, f)
			}
		}

		s.Equal(v, called)
	}
}

func (s *InvokeTestSuite) _testSliceOfStruct(v []*Struct) {
	// []*Struct
	var called []*Struct
	chk := func(vv []*Struct) ([]*Struct, error) {
		called = vv
		s.Len(vv, 4)
		if v[3] == nil {
			s.Nil(vv[3])
		}
		return v, nil
	}

	param, err := s.convert(v)
	s.Nil(err)
	s.NotNil(param)
	if param != nil {
		ret, err := s.ivk.Invoke(chk, param)
		s.Nil(err)
		s.Len(ret, 2)
		if ret != nil && len(ret) > 0 {
			f, ok := ret[0].([]*Struct)
			s.True(ok)
			if ok {
				s.Equal(v, f)
			}
		}

		s.Equal(v, called)
	}
}

// TODO:
func (s *InvokeTestSuite) TestPrivateField() {
}

func (s *InvokeTestSuite) TestReturn() {
	chk := func() (int, float32, string) {
		return 0, 0, ""
	}
	ret := []interface{}{int64(11), float64(12.5), "test string"}
	refined, err := s.ivk.Return(chk, ret)
	s.Nil(err)
	s.Len(refined, 3)
	s.Equal(int(11), refined[0])
	s.Equal(float32(12.5), refined[1])
	s.Equal("test string", refined[2])
}
