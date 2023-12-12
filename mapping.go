package polo

import "reflect"

// MapSorter is used by the sort package to sort a slice of reflect.Value objects.
// Assumes that the reflect.Value objects can only be types which are comparable
// i.e, can be used as a map key. (will panic otherwise)
func MapSorter(keys []reflect.Value) func(int, int) bool {
	return func(i int, j int) bool {
		a, b := keys[i], keys[j]
		if a.Kind() == reflect.Interface {
			a, b = a.Elem(), b.Elem()
		}

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
				result := MapCompare(a.Index(i), b.Index(i))
				if result == 0 {
					continue
				}

				return result < 0
			}
		}

		panic("unsupported key compare")
	}
}

// MapCompare returns an integer representing the comparison between two reflect.Value objects.
// Assumes that a and b can only have a type that is comparable. (will panic otherwise).
// Returns 1 (a > b); 0 (a == b); -1 (a < b)
func MapCompare(a, b reflect.Value) int {
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
			result := MapCompare(a.Index(i), b.Index(i))
			if result == 0 {
				continue
			}

			return result
		}

		return 0
	}

	panic("unsupported key compare")
}
