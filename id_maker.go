package dingo

import (
	"errors"
	"fmt"
	"sync/atomic"

	uuid "github.com/satori/go.uuid"
)

var ID = struct {
	// default ID maker
	Default int
	// an ID maker implemented via uuid4
	UUID int
	// an ID maker implemented by atomic.AddInt64
	SEQ int
}{
	0, 1, 2,
}

/*IDMaker is an object that can generate a series of identiy, typed as string.
Each idenity should be unique.
*/
type IDMaker interface {
	// routine(thread) safe is required.
	NewID() (string, error)
}

// default IDMaker
type uuidMaker struct{}

func (*uuidMaker) NewID() (string, error) {
	var u, _ = uuid.NewV4()
	var usuid = u.String()

	return usuid, nil
}

/*SeqIDMaker is an implementation of IDMaker suited for local mode.
A sequence of number would be generated when called. Usage:
 err := app.AddIDMaker(101, &dingo.SeqIDMaker{})
*/
type SeqIDMaker struct {
	i uint64
}

func (seq *SeqIDMaker) NewID() (string, error) {
	r := atomic.AddUint64(&seq.i, 1)
	if r > ^uint64(0)>>1 {
		return "", errors.New("sequential id generate failed: overflow")
	}
	return fmt.Sprintf("%d", r), nil
}
