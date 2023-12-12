package polo

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ExamplePolorizer is an example for using the Polorizer to encode the fields of a Fruit object
// using a Polorizer which allows sequential encoding of data into a write-only buffer
//
//nolint:lll
func ExamplePolorizer() {
	// Create a Fruit object
	orange := &Fruit{"orange", 300, []string{"tangerine", "mandarin"}}

	// Create a new Polorizer
	polorizer := NewPolorizer()

	// Encode the Name field as a string
	polorizer.PolorizeString(orange.Name)
	// Encode the Cost field as an integer
	polorizer.PolorizeInt(int64(orange.Cost))

	// Create a new Polorizer to serialize the Alias field (slice)
	aliases := NewPolorizer()
	// Encode each element in the Alias slice as a string
	for _, alias := range orange.Alias {
		aliases.PolorizeString(alias)
	}
	// Encode the Polorizer containing the alias field contents as packed data
	polorizer.PolorizePacked(aliases)

	// Print the serialized bytes in the Polorizer buffer
	fmt.Println(polorizer.Bytes())

	// Output:
	// [14 79 6 99 142 1 111 114 97 110 103 101 1 44 63 6 150 1 116 97 110 103 101 114 105 110 101 109 97 110 100 97 114 105 110]
}

func TestNewPolorizer(t *testing.T) {
	polorizer := NewPolorizer()

	assert.Nil(t, polorizer.wb.head)
	assert.Nil(t, polorizer.wb.body)
	assert.Zero(t, polorizer.wb.offset)
	assert.Zero(t, polorizer.wb.counter)
}

func TestPolorizer_PolorizeNull(t *testing.T) {
	polorizer := NewPolorizer()
	assert.Equal(t, []byte{0}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 15}, polorizer.Packed())

	polorizer.PolorizeNull()
	assert.Equal(t, []byte{0}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 0}, polorizer.Packed())

	polorizer.PolorizeNull()
	assert.Equal(t, []byte{14, 47, 0, 0}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 47, 0, 0}, polorizer.Packed())

	polorizer.PolorizeNull()
	assert.Equal(t, []byte{14, 63, 0, 0, 0}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 63, 0, 0, 0}, polorizer.Packed())
}

func TestPolorizer_PolorizeBool(t *testing.T) {
	polorizer := NewPolorizer()

	polorizer.PolorizeBool(true)
	assert.Equal(t, []byte{2}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 2}, polorizer.Packed())

	polorizer.PolorizeBool(false)
	assert.Equal(t, []byte{14, 47, 2, 1}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 47, 2, 1}, polorizer.Packed())
}

func TestPolorizer_PolorizeString(t *testing.T) {
	polorizer := NewPolorizer()

	polorizer.PolorizeString("foo")
	assert.Equal(t, []byte{6, 102, 111, 111}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 6, 102, 111, 111}, polorizer.Packed())

	polorizer.PolorizeString("bar")
	assert.Equal(t, []byte{14, 47, 6, 54, 102, 111, 111, 98, 97, 114}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 47, 6, 54, 102, 111, 111, 98, 97, 114}, polorizer.Packed())
}

func TestPolorizer_PolorizeBytes(t *testing.T) {
	polorizer := NewPolorizer()

	polorizer.PolorizeBytes([]byte{1, 1, 1, 1})
	assert.Equal(t, []byte{6, 1, 1, 1, 1}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 6, 1, 1, 1, 1}, polorizer.Packed())

	polorizer.PolorizeBytes([]byte{2, 2, 2, 2})
	assert.Equal(t, []byte{14, 47, 6, 70, 1, 1, 1, 1, 2, 2, 2, 2}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 47, 6, 70, 1, 1, 1, 1, 2, 2, 2, 2}, polorizer.Packed())
}

func TestPolorizer_PolorizeUint(t *testing.T) {
	polorizer := NewPolorizer()

	polorizer.PolorizeUint(300)
	assert.Equal(t, []byte{3, 1, 44}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 3, 1, 44}, polorizer.Packed())

	polorizer.PolorizeUint(250)
	assert.Equal(t, []byte{14, 47, 3, 35, 1, 44, 250}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 47, 3, 35, 1, 44, 250}, polorizer.Packed())
}

func TestPolorizer_PolorizeInt(t *testing.T) {
	polorizer := NewPolorizer()

	polorizer.PolorizeInt(300)
	assert.Equal(t, []byte{3, 1, 44}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 3, 1, 44}, polorizer.Packed())

	polorizer.PolorizeInt(-250)
	assert.Equal(t, []byte{14, 47, 3, 36, 1, 44, 250}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 47, 3, 36, 1, 44, 250}, polorizer.Packed())
}

func TestPolorizer_PolorizeFloat32(t *testing.T) {
	polorizer := NewPolorizer()

	polorizer.PolorizeFloat32(123.456)
	assert.Equal(t, []byte{7, 66, 246, 233, 121}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 7, 66, 246, 233, 121}, polorizer.Packed())

	polorizer.PolorizeFloat32(-99.99)
	assert.Equal(t, []byte{14, 47, 7, 71, 66, 246, 233, 121, 194, 199, 250, 225}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 47, 7, 71, 66, 246, 233, 121, 194, 199, 250, 225}, polorizer.Packed())
}

//nolint:lll
func TestPolorizer_PolorizeFloat64(t *testing.T) {
	polorizer := NewPolorizer()

	polorizer.PolorizeFloat64(123.456)
	assert.Equal(t, []byte{7, 64, 94, 221, 47, 26, 159, 190, 119}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 7, 64, 94, 221, 47, 26, 159, 190, 119}, polorizer.Packed())

	polorizer.PolorizeFloat64(-99.99)
	assert.Equal(t, []byte{14, 63, 7, 135, 1, 64, 94, 221, 47, 26, 159, 190, 119, 192, 88, 255, 92, 40, 245, 194, 143}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 63, 7, 135, 1, 64, 94, 221, 47, 26, 159, 190, 119, 192, 88, 255, 92, 40, 245, 194, 143}, polorizer.Packed())
}

func TestPolorizer_PolorizeBigInt(t *testing.T) {
	polorizer := NewPolorizer()

	polorizer.PolorizeBigInt(big.NewInt(300))
	assert.Equal(t, []byte{3, 1, 44}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 3, 1, 44}, polorizer.Packed())

	polorizer.PolorizeBigInt(big.NewInt(-250))
	assert.Equal(t, []byte{14, 47, 3, 36, 1, 44, 250}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 47, 3, 36, 1, 44, 250}, polorizer.Packed())

	polorizer.PolorizeBigInt(nil)
	assert.Equal(t, []byte{14, 63, 3, 36, 48, 1, 44, 250}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 63, 3, 36, 48, 1, 44, 250}, polorizer.Packed())
}

func TestPolorizer_PolorizeRaw(t *testing.T) {
	polorizer := NewPolorizer()

	polorizer.PolorizeRaw(Raw{6, 98, 111, 111})
	assert.Equal(t, []byte{5, 6, 98, 111, 111}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 5, 6, 98, 111, 111}, polorizer.Packed())

	polorizer.PolorizeRaw(Raw{0})
	assert.Equal(t, []byte{14, 47, 5, 69, 6, 98, 111, 111, 0}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 47, 5, 69, 6, 98, 111, 111, 0}, polorizer.Packed())

	polorizer.PolorizeRaw(nil)
	assert.Equal(t, []byte{14, 63, 5, 69, 85, 6, 98, 111, 111, 0, 0}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 63, 5, 69, 85, 6, 98, 111, 111, 0, 0}, polorizer.Packed())
}

func TestPolorizer_PolorizeAny(t *testing.T) {
	polorizer := NewPolorizer()
	require.EqualError(t, polorizer.PolorizeAny(Any{200}), "malformed tag: varint terminated prematurely")

	require.NoError(t, polorizer.PolorizeAny(Any{6, 98, 111, 111}))
	assert.Equal(t, []byte{6, 98, 111, 111}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 6, 98, 111, 111}, polorizer.Packed())

	require.NoError(t, polorizer.PolorizeAny(Any{0}))
	assert.Equal(t, []byte{14, 47, 6, 48, 98, 111, 111}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 47, 6, 48, 98, 111, 111}, polorizer.Packed())

	require.NoError(t, polorizer.PolorizeAny(nil))
	assert.Equal(t, []byte{14, 63, 6, 48, 48, 98, 111, 111}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 63, 6, 48, 48, 98, 111, 111}, polorizer.Packed())
}

//nolint:lll
func TestPolorizer_PolorizeDocument(t *testing.T) {
	document := make(Document)
	_ = document.Set("far", 123)
	_ = document.Set("foo", "bar")

	polorizer := NewPolorizer()
	polorizer.PolorizeDocument(document)
	assert.Equal(t, []byte{13, 95, 6, 53, 86, 133, 1, 102, 97, 114, 3, 123, 102, 111, 111, 6, 98, 97, 114}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 13, 95, 6, 53, 86, 133, 1, 102, 97, 114, 3, 123, 102, 111, 111, 6, 98, 97, 114}, polorizer.Packed())

	polorizer.PolorizeDocument(nil)
	assert.Equal(t, []byte{14, 63, 13, 160, 2, 95, 6, 53, 86, 133, 1, 102, 97, 114, 3, 123, 102, 111, 111, 6, 98, 97, 114}, polorizer.Packed())
	assert.Equal(t, []byte{14, 63, 13, 160, 2, 95, 6, 53, 86, 133, 1, 102, 97, 114, 3, 123, 102, 111, 111, 6, 98, 97, 114}, polorizer.Packed())
}

func TestPolorizer_PolorizePacked(t *testing.T) {
	polorizer := NewPolorizer()
	polorizer.PolorizePacked(nil)
	assert.Equal(t, []byte{0}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 0}, polorizer.Packed())

	another := NewPolorizer()
	another.PolorizePacked(polorizer)
	assert.Equal(t, []byte{14, 31, 0}, another.Bytes())
	assert.Equal(t, []byte{14, 31, 14, 31, 0}, another.Packed())
}

func TestPolorizer_polorizerInner(t *testing.T) {
	polorizer := NewPolorizer()
	polorizer.polorizeInner(nil)
	assert.Equal(t, []byte{0}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 0}, polorizer.Packed())

	assert.Nil(t, polorizer.Polorize(5))
	assert.Equal(t, []byte{14, 47, 0, 3, 5}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 47, 0, 3, 5}, polorizer.Packed())

	another := NewPolorizer()
	another.PolorizeInt(300)

	polorizer.polorizeInner(another)
	assert.Equal(t, []byte{14, 63, 0, 3, 19, 5, 1, 44}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 63, 0, 3, 19, 5, 1, 44}, polorizer.Packed())

	another.PolorizeInt(250)
	polorizer.polorizeInner(another)
	assert.Equal(t, []byte{14, 79, 0, 3, 19, 62, 5, 1, 44, 47, 3, 35, 1, 44, 250}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 79, 0, 3, 19, 62, 5, 1, 44, 47, 3, 35, 1, 44, 250}, polorizer.Packed())
}
