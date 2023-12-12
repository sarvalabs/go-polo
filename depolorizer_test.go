package polo

import (
	"fmt"
	"log"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ExampleDepolorizer is an example for using the Depolorizer to decode the fields of a Fruit object
// using a Depolorizer which allows sequential decoding of data from a read-only buffer
func ExampleDepolorizer() {
	wire := []byte{
		14, 79, 6, 99, 142, 1, 111, 114, 97, 110, 103, 101, 1, 44, 63, 6, 150, 1, 116,
		97, 110, 103, 101, 114, 105, 110, 101, 109, 97, 110, 100, 97, 114, 105, 110,
	}

	// Create a new instance of Fruit
	object := new(Fruit)

	// Create a new Depolorizer from the data
	depolorizer, err := NewDepolorizer(wire)
	if err != nil {
		log.Fatalln("invalid wire:", err)
	}

	depolorizer, err = depolorizer.DepolorizePacked()
	if err != nil {
		log.Fatalln("invalid wire:", err)
	}

	// Decode the Name field as a string
	object.Name, err = depolorizer.DepolorizeString()
	if err != nil {
		log.Fatalln("invalid field 'Name':", err)
	}

	// Decode the Cost field as a string
	Cost, err := depolorizer.DepolorizeInt()
	if err != nil {
		log.Fatalln("invalid field 'Cost':", err)
	}
	object.Cost = int(Cost)

	// Decode a new Depolorizer to deserialize the Alias field (slice)
	aliases, err := depolorizer.DepolorizePacked()
	if err != nil {
		log.Fatalln("invalid field 'Alias':", err)
	}

	// Decode each element from the Alias decoder as a string
	for !aliases.Done() {
		alias, err := aliases.DepolorizeString()
		if err != nil {
			log.Fatalln("invalid field element 'Alias':", err)
		}

		object.Alias = append(object.Alias, alias)
	}

	// Print the deserialized object
	fmt.Println(object)

	// Output:
	// &{orange 300 [tangerine mandarin]}
}

func TestNewDepolorizer(t *testing.T) {
	tests := []struct {
		data []byte
		err  string
		buf  *Depolorizer
	}{
		{
			[]byte{0}, "",
			&Depolorizer{data: readbuffer{WireNull, []byte{}}},
		},
		{
			[]byte{3, 1, 44}, "",
			&Depolorizer{data: readbuffer{WirePosInt, []byte{1, 44}}},
		},
		{
			[]byte{14, 47, 3, 35, 1, 44, 250}, "",
			&Depolorizer{data: readbuffer{WirePack, []byte{47, 3, 35, 1, 44, 250}}},
		},
		{[]byte{175}, "malformed tag: varint terminated prematurely", nil},
	}

	for _, test := range tests {
		depolorizer, err := NewDepolorizer(test.data)
		assert.Equal(t, test.buf, depolorizer)

		if test.err != "" {
			assert.EqualError(t, err, test.err)
			continue
		}

		assert.Nil(t, err)
	}
}

func TestDepolorizer_DepolorizeNull(t *testing.T) {
	depolorizer, err := NewDepolorizer([]byte{14, 63, 0, 2, 0})
	require.Nil(t, err)

	depolorizer, err = depolorizer.DepolorizePacked()
	require.Nil(t, err)

	err = depolorizer.DepolorizeNull()
	assert.Nil(t, err)
	assert.False(t, depolorizer.Done())

	err = depolorizer.DepolorizeNull()
	assert.EqualError(t, err, "incompatible wire: unexpected wiretype 'true'. expected one of: {null}")
	assert.False(t, depolorizer.Done())

	err = depolorizer.DepolorizeNull()
	assert.Nil(t, err)
	assert.True(t, depolorizer.Done())

	err = depolorizer.DepolorizeNull()
	assert.EqualError(t, err, "insufficient data in wire for decode")
}

func TestDepolorizer_DepolorizeAny(t *testing.T) {
	depolorizer, err := NewDepolorizer([]byte{14, 79, 6, 54, 96, 101, 102, 111, 111, 98, 97, 114, 6, 102, 111, 111})
	require.NoError(t, err)

	depolorizer, err = depolorizer.DepolorizePacked()
	require.NoError(t, err)

	wire, err := depolorizer.DepolorizeAny()
	require.NoError(t, err)
	require.Equal(t, wire, Any{6, 102, 111, 111})
	require.False(t, depolorizer.Done())

	wire, err = depolorizer.DepolorizeAny()
	require.NoError(t, err)
	require.Equal(t, wire, Any{6, 98, 97, 114})
	require.False(t, depolorizer.Done())

	wire, err = depolorizer.DepolorizeAny()
	require.NoError(t, err)
	require.Equal(t, wire, Any{0})
	require.False(t, depolorizer.Done())

	wire, err = depolorizer.DepolorizeAny()
	require.NoError(t, err)
	require.Equal(t, wire, Any{5, 6, 102, 111, 111})
	require.True(t, depolorizer.Done())

	_, err = depolorizer.DepolorizeAny()
	require.EqualError(t, err, "insufficient data in wire for decode")
}

func TestDepolorizer_DepolorizeRaw(t *testing.T) {
	depolorizer, err := NewDepolorizer([]byte{14, 63, 5, 64, 65, 6, 102, 111, 111})
	require.NoError(t, err)

	depolorizer, err = depolorizer.DepolorizePacked()
	require.NoError(t, err)

	wire, err := depolorizer.DepolorizeRaw()
	require.NoError(t, err)
	require.Equal(t, wire, Raw{6, 102, 111, 111})
	require.False(t, depolorizer.Done())

	_, err = depolorizer.DepolorizeRaw()
	require.EqualError(t, err, "incompatible wire: unexpected wiretype 'null'. expected one of: {raw}")
	require.False(t, depolorizer.Done())

	_, err = depolorizer.DepolorizeRaw()
	require.EqualError(t, err, "incompatible wire: unexpected wiretype 'false'. expected one of: {raw}")
	require.True(t, depolorizer.Done())

	_, err = depolorizer.DepolorizeRaw()
	require.EqualError(t, err, "insufficient data in wire for decode")
}

func TestDepolorizer_DepolorizeBool(t *testing.T) {
	depolorizer, err := NewDepolorizer([]byte{14, 47, 2, 1})
	require.Nil(t, err)

	depolorizer, err = depolorizer.DepolorizePacked()
	require.Nil(t, err)

	var value bool

	value, err = depolorizer.DepolorizeBool()
	assert.Nil(t, err)
	assert.Equal(t, true, value)
	assert.False(t, depolorizer.Done())

	value, err = depolorizer.DepolorizeBool()
	assert.Nil(t, err)
	assert.Equal(t, false, value)
	assert.True(t, depolorizer.Done())

	_, err = depolorizer.DepolorizeBool()
	assert.EqualError(t, err, "insufficient data in wire for decode")
}

func TestDepolorizer_DepolorizeString(t *testing.T) {
	depolorizer, err := NewDepolorizer([]byte{14, 47, 6, 54, 102, 111, 111, 98, 97, 114})
	require.Nil(t, err)

	depolorizer, err = depolorizer.DepolorizePacked()
	require.Nil(t, err)

	_, err = depolorizer.DepolorizeString()
	assert.Nil(t, err)
	assert.False(t, depolorizer.Done())

	_, err = depolorizer.DepolorizeString()
	assert.Nil(t, err)
	assert.True(t, depolorizer.Done())
}

func TestDepolorizer_DepolorizeBytes(t *testing.T) {
	depolorizer, err := NewDepolorizer([]byte{14, 47, 6, 70, 1, 1, 1, 1, 2, 2, 2, 2})
	require.Nil(t, err)

	depolorizer, err = depolorizer.DepolorizePacked()
	require.Nil(t, err)

	_, err = depolorizer.DepolorizeBytes()
	assert.Nil(t, err)
	assert.False(t, depolorizer.Done())

	_, err = depolorizer.DepolorizeBytes()
	assert.Nil(t, err)
	assert.True(t, depolorizer.Done())
}

func TestDepolorizer_DepolorizeUint(t *testing.T) {
	depolorizer, err := NewDepolorizer([]byte{14, 47, 3, 35, 1, 44, 250})
	require.Nil(t, err)

	depolorizer, err = depolorizer.DepolorizePacked()
	require.Nil(t, err)

	var value uint64

	value, err = depolorizer.DepolorizeUint()
	assert.Nil(t, err)
	assert.Equal(t, uint64(300), value)
	assert.False(t, depolorizer.Done())

	value, err = depolorizer.DepolorizeUint()
	assert.Nil(t, err)
	assert.Equal(t, uint64(250), value)
	assert.True(t, depolorizer.Done())

	value, err = depolorizer.DepolorizeUint()
	assert.EqualError(t, err, "insufficient data in wire for decode")
}

func TestDepolorizer_DepolorizeInt(t *testing.T) {
	depolorizer, err := NewDepolorizer([]byte{14, 47, 3, 36, 1, 44, 250})
	require.Nil(t, err)

	depolorizer, err = depolorizer.DepolorizePacked()
	require.Nil(t, err)

	var value int64

	value, err = depolorizer.DepolorizeInt()
	assert.Nil(t, err)
	assert.Equal(t, int64(300), value)
	assert.False(t, depolorizer.Done())

	value, err = depolorizer.DepolorizeInt()
	assert.Nil(t, err)
	assert.Equal(t, int64(-250), value)
	assert.True(t, depolorizer.Done())

	value, err = depolorizer.DepolorizeInt()
	assert.EqualError(t, err, "insufficient data in wire for decode")
}

func TestDepolorizer_DepolorizeFloat32(t *testing.T) {
	depolorizer, err := NewDepolorizer([]byte{14, 47, 7, 71, 66, 246, 233, 121, 194, 199, 250, 225})
	require.Nil(t, err)

	depolorizer, err = depolorizer.DepolorizePacked()
	require.Nil(t, err)

	_, err = depolorizer.DepolorizeFloat32()
	assert.Nil(t, err)
	assert.False(t, depolorizer.Done())

	_, err = depolorizer.DepolorizeFloat32()
	assert.Nil(t, err)
	assert.True(t, depolorizer.Done())

	_, err = depolorizer.DepolorizeFloat32()
	assert.EqualError(t, err, "insufficient data in wire for decode")
}

func TestDepolorizer_DepolorizeFloat64(t *testing.T) {
	depolorizer, err := NewDepolorizer([]byte{14, 63, 7, 135, 1, 64, 94, 221, 47, 26, 159, 190, 119, 192, 88, 255, 92, 40, 245, 194, 143})
	require.Nil(t, err)

	depolorizer, err = depolorizer.DepolorizePacked()
	require.Nil(t, err)

	_, err = depolorizer.DepolorizeFloat64()
	assert.Nil(t, err)
	assert.False(t, depolorizer.Done())

	_, err = depolorizer.DepolorizeFloat64()
	assert.Nil(t, err)
	assert.True(t, depolorizer.Done())

	_, err = depolorizer.DepolorizeFloat64()
	assert.EqualError(t, err, "insufficient data in wire for decode")
}

func TestDepolorizer_DepolorizeBigInt(t *testing.T) {
	depolorizer, err := NewDepolorizer([]byte{14, 47, 3, 36, 1, 44, 250})
	require.Nil(t, err)

	depolorizer, err = depolorizer.DepolorizePacked()
	require.Nil(t, err)

	var value *big.Int

	value, err = depolorizer.DepolorizeBigInt()
	assert.Nil(t, err)
	assert.Equal(t, value, big.NewInt(300))
	assert.False(t, depolorizer.Done())

	value, err = depolorizer.DepolorizeBigInt()
	assert.Nil(t, err)
	assert.Equal(t, value, big.NewInt(-250))
	assert.True(t, depolorizer.Done())
}

func TestDepolorizer_DepolorizePacked(t *testing.T) {
	t.Run("Insufficient", func(t *testing.T) {
		depolorizer, err := NewDepolorizer([]byte{3, 1, 44})
		require.Nil(t, err)

		_, err = depolorizer.DepolorizeInt()
		assert.Nil(t, err)
		assert.True(t, depolorizer.Done())

		_, err = depolorizer.DepolorizePacked()
		assert.EqualError(t, err, "insufficient data in wire for decode")
	})

	t.Run("Malformed", func(t *testing.T) {
		depolorizer, err := NewDepolorizer([]byte{14, 47, 14, 142, 31, 3, 1, 44})
		require.Nil(t, err)

		depolorizer, err = depolorizer.DepolorizePacked()
		require.Nil(t, err)

		_, err = depolorizer.DepolorizePacked()
		assert.EqualError(t, err, "malformed tag: varint terminated prematurely")
	})

	t.Run("Malformed Load", func(t *testing.T) {
		depolorizer, err := NewDepolorizer([]byte{14, 47})
		require.Nil(t, err)

		depolorizer, err = depolorizer.DepolorizePacked()
		assert.EqualError(t, err, "load convert fail: missing head: insufficient data in reader")
	})

	t.Run("NullPack", func(t *testing.T) {
		depolorizer, err := NewDepolorizer([]byte{0})
		require.Nil(t, err)

		_, err = depolorizer.DepolorizePacked()
		assert.EqualError(t, err, ErrNullPack.Error())
	})

	t.Run("Incompatible", func(t *testing.T) {
		depolorizer, err := NewDepolorizer([]byte{3, 1, 44})
		require.Nil(t, err)

		_, err = depolorizer.DepolorizePacked()
		assert.EqualError(t, err, "incompatible wire: unexpected wiretype 'posint'. expected one of: {null, pack, document}")
	})
}

func TestDepolorizer_depolorizeInner(t *testing.T) {
	t.Run("Insufficient", func(t *testing.T) {
		depolorizer, err := NewDepolorizer([]byte{14, 47, 0, 3, 5})
		require.Nil(t, err)

		depolorizer, err = depolorizer.DepolorizePacked()
		require.Nil(t, err)

		inner, err := depolorizer.depolorizeInner()
		assert.Nil(t, err)
		assert.Equal(t, &Depolorizer{data: readbuffer{WireNull, []byte{}}}, inner)

		inner, err = depolorizer.depolorizeInner()
		assert.Nil(t, err)
		assert.Equal(t, &Depolorizer{data: readbuffer{WirePosInt, []byte{5}}}, inner)

		inner, err = depolorizer.depolorizeInner()
		assert.EqualError(t, err, "insufficient data in wire for decode")
	})

	t.Run("Malformed", func(t *testing.T) {
		depolorizer, err := NewDepolorizer([]byte{14, 47, 6, 134})
		require.Nil(t, err)

		depolorizer, err = depolorizer.DepolorizePacked()
		require.Nil(t, err)

		_, err = depolorizer.depolorizeInner()
		assert.EqualError(t, err, "malformed tag: varint terminated prematurely")
	})
}

func TestDepolorizer_ZeroValue(t *testing.T) {
	type Object struct {
		A string
		B string
	}

	t.Run("Slice", func(t *testing.T) {
		depolorizer, err := NewDepolorizer([]byte{14, 63, 14, 144, 1, 47, 6, 54, 112, 98, 98, 112, 98, 98})
		require.Nil(t, err)

		err = depolorizer.Depolorize(new([]Object))
		assert.Nil(t, err)
	})

	t.Run("Array", func(t *testing.T) {
		depolorizer, err := NewDepolorizer([]byte{14, 63, 14, 144, 1, 47, 6, 54, 112, 98, 98, 112, 98, 98})
		require.Nil(t, err)

		err = depolorizer.Depolorize(new([2]Object))
		assert.Nil(t, err)
	})

	t.Run("Map", func(t *testing.T) {
		depolorizer, err := NewDepolorizer([]byte{14, 47, 6, 0, 98, 112, 112})
		require.Nil(t, err)

		err = depolorizer.Depolorize(new(map[string]Object))
		assert.Nil(t, err)
	})

	t.Run("Struct", func(t *testing.T) {
		depolorizer, err := NewDepolorizer([]byte{0})
		require.Nil(t, err)

		err = depolorizer.Depolorize(new(Object))
		assert.Nil(t, err)
	})
}

func TestInsufficientWire(t *testing.T) {
	tests := []struct {
		buffer *Depolorizer
		object any
	}{
		{&Depolorizer{data: readbuffer{}, done: true}, new(bool)},
		{&Depolorizer{data: readbuffer{}, done: true}, new([]byte)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(string)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(uint64)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(uint32)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(uint16)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(uint8)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(int64)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(int32)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(int16)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(int8)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(float32)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(float64)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(big.Int)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(Raw)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(Document)},
		{&Depolorizer{data: readbuffer{}, done: true}, new([]float64)},
		{&Depolorizer{data: readbuffer{}, done: true}, new([2]string)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(map[string]string)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(IntegerObject)},
		{&Depolorizer{data: readbuffer{}, done: true}, new(CustomEncodeObject)},
	}

	for tno, test := range tests {
		err := test.buffer.Depolorize(test.object)
		assert.EqualError(t, err, ErrInsufficientWire.Error(), "Test No: %v", tno)
	}
}
