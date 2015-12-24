package dingo

import (
	"encoding/gob"
	"reflect"
	"testing"

	"github.com/stretchr/testify/suite"
)

//
// gob
//

type invokerLazyGobTestSuite struct {
	InvokerTestSuite
}

func TestInvokerLazyGobSuite(t *testing.T) {
	gob.Register(map[string]int{})
	gob.Register(testEmbed{})
	gob.Register(testEmbedWithCollision{})
	gob.Register(TestStruct{})
	gob.Register(map[string]*TestStruct{})
	gob.Register([]*TestStruct{})

	suite.Run(t, &invokerLazyGobTestSuite{
		InvokerTestSuite{
			ivk:     &LazyInvoker{},
			convert: ioGOB,
		},
	})
}

//
// test cases
//

func (s *invokerLazyGobTestSuite) TestMap2Struct() {
	s.InvokerTestSuite._testMap2Struct(map[string]*TestStruct{
		"a": &TestStruct{Name: "Mary", Count: 11},
		"b": &TestStruct{Name: "Bob", Count: 10},
		"c": &TestStruct{Name: "Tom", Count: 12},
		"d": &TestStruct{}, // gob didn't allow nil value as struct
	})
}

func (s *invokerLazyGobTestSuite) TestSliceOfStruct() {
	s.InvokerTestSuite._testSliceOfStruct([]*TestStruct{
		&TestStruct{Name: "Mary", Count: 11},
		&TestStruct{Name: "Bob", Count: 10},
		&TestStruct{Name: "Tom", Count: 12},
		&TestStruct{},
	})
}

func (s *invokerLazyGobTestSuite) TestReturn() {
	// struct & *struct would be marshalled to the same
	// stuff by god, we need to take care of that.
	fn := func() (a *TestStruct, b *testEmbed) { return }

	v, err := s.convert(fn, &TestStruct{}, &testEmbed{})
	s.Nil(err)
	v, err = s.ivk.Return(fn, v)
	s.Nil(err)

	// check each type
	_, ok := v[0].(*TestStruct)
	s.True(ok)
	_, ok = v[1].(*testEmbed)
	s.True(ok)
}

//
// json-safe
//

func ioJSONSafe() func(f interface{}, args ...interface{}) (v []interface{}, err error) {
	m := &JSONSafeCodec{}
	return func(f interface{}, args ...interface{}) (v []interface{}, err error) {
		// encode
		bs, err := m.encode(args)
		if err != nil {
			return
		}

		// decode
		funcT := reflect.TypeOf(f)
		v, err = m.decode(bs, func(i int) reflect.Type {
			// for invoker, we mainly focus on input
			return funcT.In(i)
		})

		return
	}
}

type invokerLazyJsonSafeTestSuite struct {
	InvokerTestSuite
}

func TestInvokerLazyJsonSafeSuite(t *testing.T) {
	suite.Run(t, &invokerLazyJsonSafeTestSuite{
		InvokerTestSuite{
			ivk:     &LazyInvoker{},
			convert: ioJSONSafe(),
		},
	})
}

//
// test cases
//

func (s *invokerLazyJsonSafeTestSuite) TestMap2Struct() {
	s.InvokerTestSuite._testMap2Struct(map[string]*TestStruct{
		"a": &TestStruct{Name: "Mary", Count: 11},
		"b": &TestStruct{Name: "Bob", Count: 10},
		"c": &TestStruct{Name: "Tom", Count: 12},
		"d": nil,
	})
}

func (s *invokerLazyJsonSafeTestSuite) TestSliceOfStruct() {
	s.InvokerTestSuite._testSliceOfStruct([]*TestStruct{
		&TestStruct{Name: "Mary", Count: 11},
		&TestStruct{Name: "Bob", Count: 10},
		&TestStruct{Name: "Tom", Count: 12},
		nil,
	})
}
