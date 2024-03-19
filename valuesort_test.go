package polo

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValueSort_Panics(t *testing.T) {
	t.Run("Array Length", func(t *testing.T) {
		a := [4]string{}
		b := [3]string{}

		assert.PanicsWithValue(t, "array length must equal", func() {
			ValueSort([]reflect.Value{reflect.ValueOf(a), reflect.ValueOf(b)})(0, 1)
		})
	})

	t.Run("Invalid Type", func(t *testing.T) {
		a := make([]string, 2)
		b := make([]string, 4)

		assert.PanicsWithValue(t, "unsupported key compare", func() {
			ValueSort([]reflect.Value{reflect.ValueOf(a), reflect.ValueOf(b)})(0, 1)
		})
	})
}

func TestValueCmp_Panics(t *testing.T) {
	t.Run("Array Length", func(t *testing.T) {
		a := [4]string{}
		b := [3]string{}

		assert.PanicsWithValue(t, "array length must equal", func() {
			ValueCmp(reflect.ValueOf(a), reflect.ValueOf(b))
		})
	})

	t.Run("Invalid Type", func(t *testing.T) {
		a := make([]string, 2)
		b := make([]string, 4)

		assert.PanicsWithValue(t, "unsupported key compare", func() {
			ValueCmp(reflect.ValueOf(a), reflect.ValueOf(b))
		})
	})
}
