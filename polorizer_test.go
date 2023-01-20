package polo

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestPolorizer_PolorizeFloat64(t *testing.T) {
	polorizer := NewPolorizer()

	polorizer.PolorizeFloat64(123.456)
	assert.Equal(t, []byte{7, 64, 94, 221, 47, 26, 159, 190, 119}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 7, 64, 94, 221, 47, 26, 159, 190, 119}, polorizer.Packed())

	polorizer.PolorizeFloat64(-99.99)
	assert.Equal(t, []byte{14, 63, 7, 135, 1, 64, 94, 221, 47, 26, 159, 190, 119, 192, 88, 255, 92, 40, 245, 194, 143}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 63, 7, 135, 1, 64, 94, 221, 47, 26, 159, 190, 119, 192, 88, 255, 92, 40, 245, 194, 143}, polorizer.Packed())
}

func TestPolorizer_PolorizeDocument(t *testing.T) {
	document := make(Document)
	_ = document.SetObject("far", 123)
	_ = document.SetObject("foo", "bar")

	polorizer := NewPolorizer()
	polorizer.PolorizeDocument(document)
	assert.Equal(t, []byte{13, 95, 6, 54, 86, 134, 1, 102, 97, 114, 3, 123, 102, 111, 111, 6, 98, 97, 114}, polorizer.Bytes())
	assert.Equal(t, []byte{14, 31, 13, 95, 6, 54, 86, 134, 1, 102, 97, 114, 3, 123, 102, 111, 111, 6, 98, 97, 114}, polorizer.Packed())

	polorizer.PolorizeDocument(nil)
	assert.Equal(t, []byte{14, 63, 13, 160, 2, 95, 6, 54, 86, 134, 1, 102, 97, 114, 3, 123, 102, 111, 111, 6, 98, 97, 114}, polorizer.Packed())
	assert.Equal(t, []byte{14, 63, 13, 160, 2, 95, 6, 54, 86, 134, 1, 102, 97, 114, 3, 123, 102, 111, 111, 6, 98, 97, 114}, polorizer.Packed())
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

func TestSorter_Panics(t *testing.T) {
	t.Run("Array Length", func(t *testing.T) {
		a := [4]string{}
		b := [3]string{}

		assert.PanicsWithValue(t, "array length must equal", func() {
			sorter([]reflect.Value{reflect.ValueOf(a), reflect.ValueOf(b)})(0, 1)
		})
	})

	t.Run("Invalid Type", func(t *testing.T) {
		a := make([]string, 2)
		b := make([]string, 4)

		assert.PanicsWithValue(t, "unsupported key compare", func() {
			sorter([]reflect.Value{reflect.ValueOf(a), reflect.ValueOf(b)})(0, 1)
		})
	})
}

func TestCompare_Panics(t *testing.T) {
	t.Run("Array Length", func(t *testing.T) {
		a := [4]string{}
		b := [3]string{}

		assert.PanicsWithValue(t, "array length must equal", func() {
			compare(reflect.ValueOf(a), reflect.ValueOf(b))
		})
	})

	t.Run("Invalid Type", func(t *testing.T) {
		a := make([]string, 2)
		b := make([]string, 4)

		assert.PanicsWithValue(t, "unsupported key compare", func() {
			compare(reflect.ValueOf(a), reflect.ValueOf(b))
		})
	})
}
