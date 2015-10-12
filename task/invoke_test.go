package task

import (
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

//
// struct
//
type _struct struct {
	Name  string
	Count int
}

// struct with private field
type _struct_private struct {
	_struct
	private int
}

//
// embed struct
//
type _embed struct {
	_struct
	Age  int
	Addr string
}

type _embed_with_collision struct {
	_struct
	Age  int
	Addr string

	// collision, named with tag
	Name  string `json:"out.Name"`
	Count int    `json:"out.Count,omitempty"`

	// struct pointer
	S *_struct
}

//
// testing suite
//
type InvokeTestSuite struct {
	suite.Suite

	ivk Invoker
}

func (s *InvokeTestSuite) SetupSuite() {
	// prepare invoker
	s.ivk = NewDefaultInvoker()

	// TODO: map
	// TODO: slice
}

func TestInvokeTestSuiteMain(t *testing.T) {
	suite.Run(t, &InvokeTestSuite{})
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

		param, err := ioJSON(float64(1.0))
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
		param, err := ioJSON(int64(1))
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

func (s *InvokeTestSuite) TestString() {
	{
		called := ""
		chk := func(v string) (string, error) {
			called = v
			return v, nil
		}
		param, err := ioJSON("test string")
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
	// *_struct
	{
		called := (*_struct)(nil)
		chk := func(v *_struct) (*_struct, error) {
			called = v
			return v, nil
		}
		v := &_struct{Name: "Bob", Count: 10}
		param, err := ioJSON(v)
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Invoke(chk, param)
			s.Nil(err)
			s.Len(ret, 2)
			if ret != nil && len(ret) > 0 {
				f, ok := ret[0].(*_struct)
				s.True(ok)
				if ok {
					s.Equal(v, f)
				}
			}

			s.Equal(called, v)
		}
	}

	// **_struct
	{
		called := (**_struct)(nil)
		name := ""
		count := 0
		chk := func(v **_struct) (**_struct, error) {
			called = v
			if v != nil && *v != nil {
				name = (**v).Name
				count = (**v).Count
			}
			// access its element
			return v, nil
		}
		{
			v_ := &_struct{Name: "Bob", Count: 10}
			// pointer-2-pointer
			v := &v_
			param, err := ioJSON(v)
			s.Nil(err)
			s.NotNil(param)
			if param != nil {
				ret, err := s.ivk.Invoke(chk, param)
				s.Nil(err)
				s.Len(ret, 2)
				if ret != nil && len(ret) > 0 {
					f, ok := ret[0].(**_struct)
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
			param, err := ioJSON(nil)
			s.Nil(err)
			s.NotNil(param)
			if param != nil {
				ret, err := s.ivk.Invoke(chk, param)
				s.Nil(err)
				s.Len(ret, 2)
				if ret != nil && len(ret) > 0 {
					f, ok := ret[0].(**_struct)
					s.True(ok)
					if ok {
						s.Equal((**_struct)(nil), f)
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
		v := &_embed{_struct: _struct{Name: "Bob", Count: 10}, Age: 100, Addr: "heaven"}
		param, err := ioJSON(v)
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
				name_ = v._struct.Name
				count_ = v._struct.Count
				name_s = v.S.Name
				count_s = v.S.Count
			}
			return v, nil
		}
		v := &_embed_with_collision{
			_struct: _struct{Name: "Bob", Count: 10},
			Age:     100,
			Addr:    "heaven",
			Name:    "Mary",
			Count:   11,
			S:       &_struct{Name: "Tom", Count: 12},
		}
		param, err := ioJSON(v)
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
	{
		var called map[string]int
		chk := func(v map[string]int) (map[string]int, error) {
			called = v
			return v, nil
		}
		v := map[string]int{"Tom": 12, "Bob": 10, "Mary": 11}
		param, err := ioJSON(v)
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

	// map[string]*_struct
	{
		var called map[string]*_struct
		name, count := "", 0
		chk := func(v map[string]*_struct) (map[string]*_struct, error) {
			called = v
			name = v["a"].Name
			count = v["a"].Count
			s.Nil(v["d"])
			return v, nil
		}
		v := map[string]*_struct{
			"a": &_struct{Name: "Mary", Count: 11},
			"b": &_struct{Name: "Bob", Count: 10},
			"c": &_struct{Name: "Tom", Count: 12},
			"d": nil,
		}
		param, err := ioJSON(v)
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Invoke(chk, param)
			s.Nil(err)
			s.Len(ret, 2)
			if ret != nil && len(ret) > 0 {
				f, ok := ret[0].(map[string]*_struct)
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
}

func (s *InvokeTestSuite) TestSlice() {
	// []string
	{
		var called []string
		chk := func(v []string) ([]string, error) {
			called = v
			return v, nil
		}
		v := []string{"Tom", "Bob", "Mary"}
		param, err := ioJSON(v)
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

	// TODO: []*_struct
	{
		var called []*_struct
		chk := func(v []*_struct) ([]*_struct, error) {
			called = v
			s.Len(v, 4)
			s.Nil(v[3])
			return v, nil
		}
		v := []*_struct{
			&_struct{Name: "Mary", Count: 11},
			&_struct{Name: "Bob", Count: 10},
			&_struct{Name: "Tom", Count: 12},
			nil,
		}
		param, err := ioJSON(v)
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Invoke(chk, param)
			s.Nil(err)
			s.Len(ret, 2)
			if ret != nil && len(ret) > 0 {
				f, ok := ret[0].([]*_struct)
				s.True(ok)
				if ok {
					s.Equal(v, f)
				}
			}

			s.Equal(v, called)
		}
	}
}

func (s *InvokeTestSuite) TestPrivateField() {
}
