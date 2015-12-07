package transport

import (
	"github.com/stretchr/testify/suite"
)

//
// struct
//
type TestStruct struct {
	Name  string
	Count int
}

//
// embed struct
//
type _test_embed struct {
	TestStruct
	Age  int
	Addr string
}

type _test_embed_with_collision struct {
	TestStruct
	Age  int
	Addr string

	// collision, named with tag
	Name  string `json:"out.Name"`
	Count int    `json:"out.Count,omitempty"`

	// struct pointer
	S *TestStruct
}

type InvokerTestSuite struct {
	suite.Suite

	ivk     Invoker
	convert func(f interface{}, args ...interface{}) ([]interface{}, error)
}

//
// test cases
//

func (s *InvokerTestSuite) TestFloat64() {
	// float64
	{
		called := float64(0.0)
		chk := func(v float64) (float64, error) {
			called = v
			return v, nil
		}

		param, err := s.convert(chk, float64(1.0))
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Call(chk, param)
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

func (s *InvokerTestSuite) TestInt64() {
	// int64
	{
		called := int64(0)
		chk := func(v int64) (int64, error) {
			called = v
			return v, nil
		}
		param, err := s.convert(chk, int64(1))
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Call(chk, param)
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

func (s *InvokerTestSuite) TestInt() {
	// int
	{
		called := int(0)
		chk := func(v int) (int, error) {
			called = v
			return v, nil
		}
		param, err := s.convert(chk, int(1))
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Call(chk, param)
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

func (s *InvokerTestSuite) TestString() {
	{
		called := ""
		chk := func(v string) (string, error) {
			called = v
			return v, nil
		}
		param, err := s.convert(chk, "test string")
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Call(chk, param)
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

func (s *InvokerTestSuite) TestStruct() {
	// *TestStruct
	{
		called := (*TestStruct)(nil)
		chk := func(v *TestStruct) (*TestStruct, error) {
			called = v
			return v, nil
		}
		v := &TestStruct{Name: "Bob", Count: 10}
		param, err := s.convert(chk, v)
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Call(chk, param)
			s.Nil(err)
			s.Len(ret, 2)
			if ret != nil && len(ret) > 0 {
				f, ok := ret[0].(*TestStruct)
				s.True(ok)
				if ok {
					s.Equal(v, f)
				}
			}

			s.Equal(called, v)
		}
	}

	// **TestStruct
	{
		called := (**TestStruct)(nil)
		name := ""
		count := 0
		chk := func(v **TestStruct) (**TestStruct, error) {
			called = v
			if v != nil && *v != nil {
				name = (**v).Name
				count = (**v).Count
			}
			// access its element
			return v, nil
		}
		{
			v_ := &TestStruct{Name: "Bob", Count: 10}
			// pointer-2-pointer
			v := &v_
			param, err := s.convert(chk, v)
			s.Nil(err)
			s.NotNil(param)
			if param != nil {
				ret, err := s.ivk.Call(chk, param)
				s.Nil(err)
				s.Len(ret, 2)
				if ret != nil && len(ret) > 0 {
					f, ok := ret[0].(**TestStruct)
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
			param, err := s.convert(chk, nil)
			s.Nil(err)
			s.NotNil(param)
			if param != nil {
				ret, err := s.ivk.Call(chk, param)
				s.Nil(err)
				s.Len(ret, 2)
				if ret != nil && len(ret) > 0 {
					f, ok := ret[0].(**TestStruct)
					s.True(ok)
					if ok {
						s.Equal((**TestStruct)(nil), f)
					}
				}
			}
		}
	}
}

func (s *InvokerTestSuite) TestEmbed() {
	// *_test_embed
	{
		called := (*_test_embed)(nil)
		age, count, name, address := 0, 0, "", ""
		chk := func(v *_test_embed) (*_test_embed, error) {
			called = v
			if v != nil {
				name = v.Name
				age = v.Age
				count = v.Count
				address = v.Addr
			}
			return v, nil
		}
		v := &_test_embed{TestStruct: TestStruct{Name: "Bob", Count: 10}, Age: 100, Addr: "heaven"}
		param, err := s.convert(chk, v)
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Call(chk, param)
			s.Nil(err)
			s.Len(ret, 2)
			if ret != nil && len(ret) > 0 {
				f, ok := ret[0].(*_test_embed)
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

func (s *InvokerTestSuite) TestEmbedWithCollision() {
	// *_test_embed_with_collision
	{
		called := (*_test_embed_with_collision)(nil)
		age, count, name, address, count_, name_, count_s, name_s := 0, 0, "", "", 0, "", 0, ""
		chk := func(v *_test_embed_with_collision) (*_test_embed_with_collision, error) {
			called = v
			if v != nil {
				name = v.Name
				age = v.Age
				count = v.Count
				address = v.Addr
				name_ = v.TestStruct.Name
				count_ = v.TestStruct.Count
				name_s = v.S.Name
				count_s = v.S.Count
			}
			return v, nil
		}
		v := &_test_embed_with_collision{
			TestStruct: TestStruct{Name: "Bob", Count: 10},
			Age:        100,
			Addr:       "heaven",
			Name:       "Mary",
			Count:      11,
			S:          &TestStruct{Name: "Tom", Count: 12},
		}
		param, err := s.convert(chk, v)
		s.Nil(err)
		s.NotNil(param)
		if param != nil {
			ret, err := s.ivk.Call(chk, param)
			s.Nil(err)
			s.Len(ret, 2)
			if ret != nil && len(ret) > 0 {
				f, ok := ret[0].(*_test_embed_with_collision)
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

func (s *InvokerTestSuite) TestMap() {
	// map[string]int
	var called map[string]int
	chk := func(v map[string]int) (map[string]int, error) {
		called = v
		return v, nil
	}
	v := map[string]int{"Tom": 12, "Bob": 10, "Mary": 11}
	param, err := s.convert(chk, v)
	s.Nil(err)
	s.NotNil(param)
	if param != nil {
		ret, err := s.ivk.Call(chk, param)
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

func (s *InvokerTestSuite) TestSlice() {
	// []string
	var called []string
	chk := func(v []string) ([]string, error) {
		called = v
		return v, nil
	}
	v := []string{"Tom", "Bob", "Mary"}
	param, err := s.convert(chk, v)
	s.Nil(err)
	s.NotNil(param)
	if param != nil {
		ret, err := s.ivk.Call(chk, param)
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

func (s *InvokerTestSuite) _testMap2Struct(v map[string]*TestStruct) {
	// map[string]*TestStruct
	var called map[string]*TestStruct
	name, count := "", 0
	chk := func(vv map[string]*TestStruct) (map[string]*TestStruct, error) {
		called = vv
		name = vv["a"].Name
		count = vv["a"].Count
		if v["d"] == nil {
			s.Nil(vv["d"])
		}
		return vv, nil
	}

	param, err := s.convert(chk, v)
	s.Nil(err)
	s.NotNil(param)
	if param != nil {
		ret, err := s.ivk.Call(chk, param)
		s.Nil(err)
		s.Len(ret, 2)
		if ret != nil && len(ret) > 0 {
			f, ok := ret[0].(map[string]*TestStruct)
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

func (s *InvokerTestSuite) _testSliceOfStruct(v []*TestStruct) {
	// []*TestStruct
	var called []*TestStruct
	chk := func(vv []*TestStruct) ([]*TestStruct, error) {
		called = vv
		s.Len(vv, 4)
		if v[3] == nil {
			s.Nil(vv[3])
		}
		return v, nil
	}

	param, err := s.convert(chk, v)
	s.Nil(err)
	s.NotNil(param)
	if param != nil {
		ret, err := s.ivk.Call(chk, param)
		s.Nil(err)
		s.Len(ret, 2)
		if ret != nil && len(ret) > 0 {
			f, ok := ret[0].([]*TestStruct)
			s.True(ok)
			if ok {
				s.Equal(v, f)
			}
		}

		s.Equal(v, called)
	}
}
