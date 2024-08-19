package polo

import "reflect"

// ValueSort is used by the sort package to sort a slice of reflect.Value objects.
// Assumes that the reflect.Value objects can only be types which are comparable
// i.e, can be used as a map key. (will panic otherwise)
func ValueSort(keys []reflect.Value) func(int, int) bool {
	return func(i int, j int) bool {
		a, b := keys[i], keys[j]
		if a.Kind() == reflect.Interface {
			a, b = a.Elem(), b.Elem()
		}

		return ValueLt(a, b)
	}
}

// ValueLt is returns a < b, for two reflected values a & b
func ValueLt(a, b reflect.Value) bool {
	switch a.Kind() {
	case reflect.Bool:
		return b.Bool()

	case reflect.String:
		return a.String() < b.String()

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.Int() < b.Int()

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return a.Uint() < b.Uint()

	case reflect.Float32, reflect.Float64:
		return a.Float() < b.Float()

	case reflect.Array:
		if a.Len() != b.Len() {
			panic("array length must equal")
		}

		for i := 0; i < a.Len(); i++ {
			result := ValueCmp(a.Index(i), b.Index(i))
			if result == 0 {
				continue
			}

			return result < 0
		}
	}

	panic("unsupported key compare")
}

// ValueCmp returns an integer representing the comparison between two reflect.Value objects.
// Assumes that a and b can only have a type that is comparable. (will panic otherwise).
// Returns 1 (a > b); 0 (a == b); -1 (a < b)
func ValueCmp(a, b reflect.Value) int {
	if a.Kind() == reflect.Interface {
		a, b = a.Elem(), b.Elem()
	}

	switch a.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		av, bv := a.Int(), b.Int()

		switch {
		case av < bv:
			return -1
		case av == bv:
			return 0
		case av > bv:
			return 1
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		av, bv := a.Uint(), b.Uint()

		switch {
		case av < bv:
			return -1
		case av == bv:
			return 0
		case av > bv:
			return 1
		}

	case reflect.Float32, reflect.Float64:
		av, bv := a.Float(), b.Float()

		switch {
		case av < bv:
			return -1
		case av == bv:
			return 0
		case av > bv:
			return 1
		}

	case reflect.String:
		av, bv := a.String(), b.String()

		switch {
		case av < bv:
			return -1
		case av == bv:
			return 0
		case av > bv:
			return 1
		}

	case reflect.Array:
		if a.Len() != b.Len() {
			panic("array length must equal")
		}

		for i := 0; i < a.Len(); i++ {
			result := ValueCmp(a.Index(i), b.Index(i))
			if result == 0 {
				continue
			}

			return result
		}

		return 0
	}

	panic("unsupported key compare")
}

// Key is an indexed reflect value. Preserves the original index of key.
// Can be used in conjunction with the KeySort function to acquire sorted order of reflected value.
type Key struct {
	idx int // represents the original index before sorting
	val reflect.Value
}

// NewKey creates a new value for the given index and value.
// The given value is reflected to use in Key.
func NewKey(idx int, val any) Key {
	return Key{idx: idx, val: reflect.ValueOf(val)}
}

// Index returns the index of the key
func (key Key) Index() int {
	return key.idx
}

// KeySort accepts a slice of Key values and a channel to return them on.
// They are returned in sorted order and are merge sorted recursively.
func KeySort(keys []Key, ch chan Key) {
	defer close(ch)

	// If there are no elements, return
	if len(keys) == 0 {
		return
	}

	// If there is only 1 element, return it
	if len(keys) == 1 {
		ch <- keys[0]
		return
	}

	// Determine the midpoint
	mid := len(keys) / 2

	// Start sorting the left side
	left := make(chan Key)
	go KeySort(keys[:mid], left)
	// Start sorting the right side
	right := make(chan Key)
	go KeySort(keys[mid:], right)

	// Get initial values from the both channels
	lv, lok := <-left
	rv, rok := <-right

	// Iterate until both sides have returned all values
	for lok || rok {
		switch {
		case lok && rok:
			if ValueLt(lv.val, rv.val) {
				ch <- lv
				lv, lok = <-left
			} else {
				ch <- rv
				rv, rok = <-right
			}

		case lok && !rok:
			ch <- lv
			lv, lok = <-left

		case !lok && rok:
			ch <- rv
			rv, rok = <-right
		}
	}
}
