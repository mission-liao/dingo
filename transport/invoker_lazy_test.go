package transport

import (
	"encoding/gob"
	"testing"

	"github.com/stretchr/testify/suite"
)

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
