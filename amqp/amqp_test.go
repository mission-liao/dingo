package dgamqp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAmqpInitClose(t *testing.T) {
	ass := assert.New(t)

	conn, err := newConnection(DefaultAmqpConfig())
	ass.Nil(err)

	for i := 0; i < 100; i++ {
		func() {
			ch, err := conn.Channel()
			ass.Nil(err)
			if err == nil {
				defer conn.ReleaseChannel(ch)
			}
		}()
	}

	ass.Nil(conn.Close())
}

func TestAmqp127_0_0_1(t *testing.T) {
	ass := assert.New(t)
	conn, err := newConnection(DefaultAmqpConfig().Host("127.0.0.1"))
	ass.Nil(err)

	ch, err := conn.Channel()
	ass.Nil(err)
	defer conn.ReleaseChannel(ch)

	ass.Nil(conn.Close())
}
