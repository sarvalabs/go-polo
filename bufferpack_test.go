package polo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadbuffer(t *testing.T) {
	tests := []struct {
		data []byte
		tag  int
		wire WireType
		load bool
	}{
		{[]byte{0}, 1, WireNull, false},
		{[]byte{14, 47, 6, 54, 112, 98, 98, 112, 98, 98}, 1, WirePack, true},
	}

	for _, test := range tests {
		rb, err := newreadbuffer(test.data)
		assert.Nil(t, err)
		assert.Equal(t, test.data[test.tag:], rb.data)
		assert.Equal(t, test.wire, rb.wire)

		if _, err = rb.unpack(); test.load {
			assert.Nil(t, err)
		} else {
			assert.NotNil(t, err)
		}
	}
}

func TestLoadReader(t *testing.T) {
	load := newpackbuffer(
		[]byte{6, 51, 86},
		[]byte{112, 98, 98, 1, 44, 112, 98, 98},
	)

	assert.Equal(t, 0, load.coff)
	assert.Equal(t, 0, load.noff)

	assert.Equal(t, WireNull, load.cw)
	assert.Equal(t, WireWord, load.nw)

	data, err := load.next()
	assert.Nil(t, err)
	assert.Equal(t, readbuffer{WireWord, []byte{112, 98, 98}}, data)

	peek, ok := load.peek()
	assert.True(t, ok)
	assert.False(t, load.done())
	assert.Equal(t, WirePosInt, peek)

	data, err = load.next()
	assert.Nil(t, err)
	assert.Equal(t, readbuffer{WirePosInt, []byte{1, 44}}, data)

	peek, ok = load.peek()
	assert.True(t, ok)
	assert.False(t, load.done())
	assert.Equal(t, WireWord, peek)

	data, err = load.next()
	assert.Nil(t, err)
	assert.Equal(t, readbuffer{WireWord, []byte{112, 98, 98}}, data)

	peek, ok = load.peek()
	assert.False(t, ok)
	assert.True(t, load.done())
	assert.Equal(t, WireNull, peek)

	assert.Equal(t, 5, load.coff)
	assert.Equal(t, -1, load.noff)

	assert.Equal(t, WireWord, load.cw)
	assert.Equal(t, WireNull, load.nw)

	data, err = load.next()
	assert.EqualError(t, err, ErrInsufficientWire.Error())
	assert.Equal(t, readbuffer{}, data)
}
