package transport

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskEqual(t *testing.T) {
	ass := assert.New(t)
	ivk := NewDefaultInvoker()

	// same
	{
		m1 := map[string]string{
			"t1": "1",
			"t2": "2",
		}

		m2 := map[string]string{
			"t1": "1",
			"t2": "2",
		}

		t, err := ivk.ComposeTask("name#1", []interface{}{1, "test123", m1})
		ass.Nil(err)

		o, err := ivk.ComposeTask("name#1", []interface{}{1, "test123", m2})
		ass.Nil(err)
		ass.True(t.Equal(o))
	}

	// diff map
	{
		t, err := ivk.ComposeTask("name#1", []interface{}{1, "test123", map[string]string{
			"t1": "1",
			"t2": "2",
		}})
		ass.Nil(err)

		o, err := ivk.ComposeTask("name#1", []interface{}{1, "test123", map[string]string{
			"t2": "2",
			"t3": "3",
		}})
		ass.Nil(err)
		ass.False(t.Equal(o))
	}

	// only Name is different
	{
		t, err := ivk.ComposeTask("name#1", []interface{}{1, "test#123"})
		ass.Nil(err)

		o, err := ivk.ComposeTask("name#2", []interface{}{1, "test#123"})
		ass.Nil(err)
		ass.False(t.Equal(o))
	}

	// sequence of args is different
	{
		t, err := ivk.ComposeTask("name#1", []interface{}{1, "test#123"})
		ass.Nil(err)

		o, err := ivk.ComposeTask("name#1", []interface{}{"test#123", 1})
		ass.Nil(err)
		ass.False(t.Equal(o))
	}

	// different args
	{
		t, err := ivk.ComposeTask("name#1", []interface{}{1, "test#123"})
		ass.Nil(err)

		o, err := ivk.ComposeTask("name#1", []interface{}{2, "test#123"})
		ass.Nil(err)
		ass.False(t.Equal(o))
	}
}
