package polo

import (
	"bytes"
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"
)

func TestSizeVarint(t *testing.T) {
	var (
		v        uint64
		expected int
	)

	f := fuzz.New().Funcs(func(v *uint64, c fuzz.Continue) {
		x := c.Int63n(64)
		y := c.Int63n(128)
		*v = uint64(y << (x))
	})

	for i := 0; i < 10000; i++ {
		f.Fuzz(&v)

		switch {
		case v < 1<<7:
			expected = 1
		case v < 1<<14:
			expected = 2
		case v < 1<<21:
			expected = 3
		case v < 1<<28:
			expected = 4
		case v < 1<<35:
			expected = 5
		case v < 1<<42:
			expected = 6
		case v < 1<<49:
			expected = 7
		case v < 1<<56:
			expected = 8
		case v < 1<<63:
			expected = 9
		default:
			expected = 10
		}

		result := sizeVarint(v)
		assert.Equal(t, expected, result, "Input: %v", v)
	}
}

func TestAppendVarint(t *testing.T) {
	var v uint64

	f := fuzz.New().Funcs(func(v *uint64, c fuzz.Continue) {
		x := c.Int63n(64)
		y := c.Int63n(128)
		*v = uint64(y << (x))
	})

	for i := 0; i < 10000; i++ {
		f.Fuzz(&v)

		b := make([]byte, 0, 10)
		b = appendVarint(b, v)

		assert.Equal(t, encodeVarint(v), b, "Input: %v", v)
	}
}

func TestConsumeVarint(t *testing.T) {
	tests := []struct {
		input    []byte
		value    uint64
		consumed int
		err      string
	}{
		{[]byte{0}, 0, 1, ""},
		{[]byte{1}, 1, 1, ""},
		{[]byte{127}, 127, 1, ""},
		{[]byte{128, 1}, 128, 2, ""},
		{[]byte{128, 2}, 256, 2, ""},
		{[]byte{140, 204, 239, 5}, 12314124, 4, ""},

		{[]byte{129}, 1, 2, errVarintTerminated.Error()},
		{[]byte{255, 128, 128, 128, 128, 128, 128, 128, 128, 127}, 127, 10, errVarintOverflow.Error()},
		{[]byte{128, 128, 128, 128, 128, 128, 128, 128, 128, 128, 127}, 0, 11, errVarintOverflow.Error()},
	}

	for _, test := range tests {
		r := bytes.NewReader(test.input)
		val, con, err := consumeVarint(r)

		assert.Equal(t, test.value, val, "Input: %v", test.input)
		assert.Equal(t, test.consumed, con, "Input: %v. Reader: %v", test.input, r)

		if test.err == "" {
			assert.Nil(t, err)
		} else {
			assert.Equal(t, test.err, err.Error(), "Input: %v", test.input)
		}
	}
}
