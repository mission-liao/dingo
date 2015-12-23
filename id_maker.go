package dingo

import (
	"github.com/satori/go.uuid"
)

var ID = struct {
	// default ID maker
	Default int
	// an ID maker implemented via uuid4
	UUID int
}{
	0, 1,
}

/*
 An object that can generate a series of identiy, typed as string.
 Each idenity should be unique.
*/
type IDMaker interface {
	// routine(thread) safe is required.
	NewID() (string, error)
}

// default IDMaker
type uuidMaker struct{}

func (*uuidMaker) NewID() (string, error) {
	return uuid.NewV4().String(), nil
}
