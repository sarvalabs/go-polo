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

func TestKey(t *testing.T) {
	tests := []struct {
		name string
		idx  int
		val  any
	}{
		{"Int Key", 0, 42},
		{"String Key", 1, "test"},
		{"Float Key", 2, 3.14},
		{"Bool Key", 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := NewKey(tt.idx, tt.val)
			assert.Equal(t, tt.idx, key.Index())
			assert.Equal(t, reflect.ValueOf(tt.val), key.val)
		})
	}
}

func TestKeySort(t *testing.T) {
	tests := []struct {
		name   string
		keys   []Key
		sorted []Key
	}{
		{
			"Int Keys",
			[]Key{NewKey(0, 3), NewKey(1, 1), NewKey(2, 2)},
			[]Key{NewKey(1, 1), NewKey(2, 2), NewKey(0, 3)},
		},
		{
			"String Keys",
			[]Key{NewKey(0, "c"), NewKey(1, "a"), NewKey(2, "b")},
			[]Key{NewKey(1, "a"), NewKey(2, "b"), NewKey(0, "c")},
		},
		{
			"Float Keys",
			[]Key{NewKey(0, 3.1), NewKey(1, 1.1), NewKey(2, 2.1)},
			[]Key{NewKey(1, 1.1), NewKey(2, 2.1), NewKey(0, 3.1)},
		},
		{
			"Bool Keys",
			[]Key{NewKey(0, true), NewKey(1, false)},
			[]Key{NewKey(1, false), NewKey(0, true)},
		},
		{
			"Single Key",
			[]Key{NewKey(0, 42)},
			[]Key{NewKey(0, 42)},
		},
		{
			"No Keys",
			[]Key{},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := make(chan Key)
			go KeySort(tt.keys, ch)

			var sortedKeys []Key
			for key := range ch {
				sortedKeys = append(sortedKeys, key)
			}

			assert.Equal(t, tt.sorted, sortedKeys)
		})
	}
}
