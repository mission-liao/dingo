package dingo

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
func ioJSON(f interface{}, v ...interface{}) ([]interface{}, error) {
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
func ioGOB(f interface{}, v ...interface{}) ([]interface{}, error) {
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
// base suite of all tests for GenericInvoker
//

type InvokerGenericTestSuite struct {
	InvokerTestSuite
}

func (s *InvokerGenericTestSuite) TestReturn() {
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

//
// json
//

type invokerGenericJsonTestSuite struct {
	InvokerGenericTestSuite
}

func TestInvokerGenericJsonSuite(t *testing.T) {
	suite.Run(t, &invokerGenericJsonTestSuite{
		InvokerGenericTestSuite{
			InvokerTestSuite{
				ivk:     &GenericInvoker{},
				convert: ioJSON,
			},
		},
	})
}

func (s *invokerGenericJsonTestSuite) TestMap2Struct() {
	s.InvokerTestSuite._testMap2Struct(map[string]*TestStruct{
		"a": &TestStruct{Name: "Mary", Count: 11},
		"b": &TestStruct{Name: "Bob", Count: 10},
		"c": &TestStruct{Name: "Tom", Count: 12},
		"d": nil,
	})
}

func (s *invokerGenericJsonTestSuite) TestSliceOfStruct() {
	s.InvokerTestSuite._testSliceOfStruct([]*TestStruct{
		&TestStruct{Name: "Mary", Count: 11},
		&TestStruct{Name: "Bob", Count: 10},
		&TestStruct{Name: "Tom", Count: 12},
		nil,
	})
}

//
// gob
//

type invokerGenericGobTestSuite struct {
	InvokerGenericTestSuite
}

func TestInvokerGenericGobSuite(t *testing.T) {
	gob.Register(map[string]int{})
	gob.Register(_test_embed{})
	gob.Register(_test_embed_with_collision{})
	gob.Register(TestStruct{})
	gob.Register(map[string]*TestStruct{})
	gob.Register([]*TestStruct{})

	suite.Run(t, &invokerGenericGobTestSuite{
		InvokerGenericTestSuite{
			InvokerTestSuite{
				ivk:     &GenericInvoker{},
				convert: ioGOB,
			},
		},
	})
}

func (s *invokerGenericGobTestSuite) TestMap2Struct() {
	s.InvokerTestSuite._testMap2Struct(map[string]*TestStruct{
		"a": &TestStruct{Name: "Mary", Count: 11},
		"b": &TestStruct{Name: "Bob", Count: 10},
		"c": &TestStruct{Name: "Tom", Count: 12},
		"d": &TestStruct{}, // gob didn't allow nil value as struct
	})
}

func (s *invokerGenericGobTestSuite) TestSliceOfStruct() {
	s.InvokerTestSuite._testSliceOfStruct([]*TestStruct{
		&TestStruct{Name: "Mary", Count: 11},
		&TestStruct{Name: "Bob", Count: 10},
		&TestStruct{Name: "Tom", Count: 12},
		&TestStruct{},
	})
}

//
// json safe
//

type invokerGenericJsonSafeTestSuite struct {
	InvokerGenericTestSuite
}

func TestInvokerGenericJsonSafeSuite(t *testing.T) {
	suite.Run(t, &invokerGenericJsonSafeTestSuite{
		InvokerGenericTestSuite{
			InvokerTestSuite{
				ivk:     &GenericInvoker{},
				convert: ioJsonSafe(),
			},
		},
	})
}

func (s *invokerGenericJsonSafeTestSuite) TestMap2Struct() {
	s.InvokerTestSuite._testMap2Struct(map[string]*TestStruct{
		"a": &TestStruct{Name: "Mary", Count: 11},
		"b": &TestStruct{Name: "Bob", Count: 10},
		"c": &TestStruct{Name: "Tom", Count: 12},
		"d": nil,
	})
}

func (s *invokerGenericJsonSafeTestSuite) TestSliceOfStruct() {
	s.InvokerTestSuite._testSliceOfStruct([]*TestStruct{
		&TestStruct{Name: "Mary", Count: 11},
		&TestStruct{Name: "Bob", Count: 10},
		&TestStruct{Name: "Tom", Count: 12},
		nil,
	})
}
