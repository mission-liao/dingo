package transport

import (
	"encoding/gob"
	"reflect"
	"testing"

	"github.com/stretchr/testify/suite"
)

//
// gob
//

type InvokerLazyGobTestSuite struct {
	InvokerTestSuite
}

func TestInvokerLazyGobSuite(t *testing.T) {
	gob.Register(map[string]int{})
	gob.Register(_test_embed{})
	gob.Register(_test_embed_with_collision{})
	gob.Register(TestStruct{})
	gob.Register(map[string]*TestStruct{})
	gob.Register([]*TestStruct{})

	suite.Run(t, &InvokerLazyGobTestSuite{
		InvokerTestSuite{
			ivk:     &LazyInvoker{},
			convert: ioGOB,
		},
	})
}

//
// test cases
//

func (s *InvokerLazyGobTestSuite) TestMap2Struct() {
	s.InvokerTestSuite._testMap2Struct(map[string]*TestStruct{
		"a": &TestStruct{Name: "Mary", Count: 11},
		"b": &TestStruct{Name: "Bob", Count: 10},
		"c": &TestStruct{Name: "Tom", Count: 12},
		"d": &TestStruct{}, // gob didn't allow nil value as struct
	})
}

func (s *InvokerLazyGobTestSuite) TestSliceOfStruct() {
	s.InvokerTestSuite._testSliceOfStruct([]*TestStruct{
		&TestStruct{Name: "Mary", Count: 11},
		&TestStruct{Name: "Bob", Count: 10},
		&TestStruct{Name: "Tom", Count: 12},
		&TestStruct{},
	})
}

func (s *InvokerLazyGobTestSuite) TestReturn() {
	// struct & *struct would be marshalled to the same
	// stuff by god, we need to take care of that.
	fn := func() (a *TestStruct, b *_test_embed) { return }

	v, err := s.convert(fn, &TestStruct{}, &_test_embed{})
	s.Nil(err)
	v, err = s.ivk.Return(fn, v)
	s.Nil(err)

	// check each type
	_, ok := v[0].(*TestStruct)
	s.True(ok)
	_, ok = v[1].(*_test_embed)
	s.True(ok)
}

//
// json-safe
//

type InvokerLazyJsonSafeTestSuite struct {
	InvokerTestSuite
}

func TestInvokerLazyJsonSafeSuite(t *testing.T) {
	m := JsonSafeMarshaller{}
	suite.Run(t, &InvokerLazyJsonSafeTestSuite{
		InvokerTestSuite{
			ivk: &LazyInvoker{},
			convert: func(f interface{}, args ...interface{}) (v []interface{}, err error) {
				// encode
				b, offs, err := m.encode(args)
				if err != nil {
					return
				}

				// decode
				funcT := reflect.TypeOf(f)
				v, _, err = m.decode(b, offs, func(i int) reflect.Type {
					// for invoker, we mainly focus on input
					return funcT.In(i)
				})

				return
			},
		},
	})
}
