package polo

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/bits"
	"reflect"
	"sort"
)

// polorize accepts a reflect.Value and writebuffer
// and encodes the value into the writebuffer
func polorize(v reflect.Value, wb *writebuffer) (err error) {
	switch kind := v.Kind(); kind {

	// Pointer Value (unwrap and polorize)
	case reflect.Ptr:
		if v.IsNil() {
			wb.write(WireNull, nil)
			return nil
		}

		return polorize(v.Elem(), wb)

	// Boolean Value (no data)
	case reflect.Bool:
		return polorizeBool(v.Bool(), wb)

	// String Value (UTF-8 Encoded Data, Big Endian)
	case reflect.String:
		return polorizeString(v.String(), wb)

	// Unsigned Integer (Binary Encoded Data, BigEndian)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return polorizeUint(v.Uint(), wb)

	// Signed Integer (Binary Encoded Data of the absolute value, BigEndian)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return polorizeInt(v.Int(), wb)

	// Single Point Float (IEEE-754 Single Point Precision Encoded Data, BigEndian)
	case reflect.Float32:
		return polorizeFloat32(float32(v.Float()), wb)

	// Double Point Float (IEEE-754 Double Point Precision Encoded Data, BigEndian)
	case reflect.Float64:
		return polorizeFloat64(v.Float(), wb)

	// Slice Value (Pack Encoded)
	case reflect.Slice:
		return polorizeSlice(v, wb)

	// Array Value (Pack Encoded)
	case reflect.Array:
		return polorizeArray(v, wb)

	// Map Value (Pack Encoded. Key-Value. Sorted Keys)
	case reflect.Map:
		return polorizeMap(v, wb)

	// Struct Value (Field Ordered Pack Encoded)
	case reflect.Struct:
		return polorizeStruct(v, wb)

	// Unsupported Type
	default:
		// Create a recovery handler to check if v.Type() panicked.
		// This will occur if v is zero Value for an abstract nil.
		defer func() {
			if recover() != nil {
				err = errors.New("unsupported type: cannot encode abstract nil")
			}
		}()

		return fmt.Errorf("unsupported type: %v [%v]", v.Type(), v.Type().Kind())
	}
}

// sizeInteger returns the min number of bytes
// required to represent an unsigned 64-bit integer.
func sizeInteger(v uint64) int {
	return (bits.Len64(v) + 8 - 1) / 8
}

// polorizeBool polorizes a bool value into writebuffer
func polorizeBool(value bool, wb *writebuffer) error {
	var wiretype = WireFalse
	if value {
		wiretype = WireTrue
	}

	wb.write(wiretype, nil)
	return nil
}

// polorizeString polorizes a string value into writebuffer
func polorizeString(value string, wb *writebuffer) error {
	wb.write(WireWord, []byte(value))
	return nil
}

// polorizeInt polorizes an unsigned integer value into writebuffer
func polorizeUint(value uint64, wb *writebuffer) error {
	if value == 0 {
		wb.write(WirePosInt, nil)
		return nil
	}

	var buffer [8]byte

	binary.BigEndian.PutUint64(buffer[:], value)
	wb.write(WirePosInt, buffer[8-sizeInteger(value):])

	return nil
}

// polorizeInt polorizes a signed integer value into writebuffer
func polorizeInt(value int64, wb *writebuffer) error {
	if value == 0 {
		wb.write(WirePosInt, nil)
		return nil
	}

	var (
		buffer   [8]byte
		unsigned uint64
		wiretype WireType
	)

	if value < 0 {
		unsigned = uint64(-value)
		wiretype = WireNegInt
	} else {
		unsigned = uint64(value)
		wiretype = WirePosInt
	}

	binary.BigEndian.PutUint64(buffer[:], unsigned)
	wb.write(wiretype, buffer[8-sizeInteger(unsigned):])

	return nil
}

// polorizeFloat32 polorizes a float32 value into writebuffer
func polorizeFloat32(value float32, wb *writebuffer) error {
	var buffer [4]byte
	// Convert float into IEEE754 binary representation (single point)
	binary.BigEndian.PutUint32(buffer[:], math.Float32bits(value))
	wb.write(WireFloat, buffer[:])

	return nil
}

// polorizeFloat64 polorizes a float64 value into writebuffer
func polorizeFloat64(value float64, wb *writebuffer) error {
	var buffer [8]byte
	// Convert float into IEEE754 binary representation (double point)
	binary.BigEndian.PutUint64(buffer[:], math.Float64bits(value))
	wb.write(WireFloat, buffer[:])

	return nil
}

// polorizeSlice polorizes a slice value into writebuffer
func polorizeSlice(value reflect.Value, wb *writebuffer) error {
	// Nil Slice
	if value.IsNil() {
		wb.write(WireNull, nil)
		return nil
	}

	// Byte Slice
	if value.Type().Elem().Kind() == reflect.Uint8 {
		wb.write(WireWord, value.Bytes())
		return nil
	}

	var swb writebuffer
	// Serialize each element into the writebuffer
	for i := 0; i < value.Len(); i++ {
		if err := polorize(value.Index(i), &swb); err != nil {
			return err
		}
	}

	wb.write(WirePack, swb.load())
	return nil
}

// polorizeArray polorizes an array value into writebuffer
func polorizeArray(value reflect.Value, wb *writebuffer) error {
	// Byte Array
	if element := value.Type().Elem(); element.Kind() == reflect.Uint8 {
		// Create a Byte Slice Value
		s := reflect.MakeSlice(reflect.SliceOf(element), 0, value.Len())
		// Append each value in the array into the slice
		for i := 0; i < value.Len(); i++ {
			s = reflect.Append(s, value.Index(i))
		}

		wb.write(WireWord, s.Bytes())
		return nil
	}

	var awb writebuffer
	// Serialize each element into the writebuffer
	for i := 0; i < value.Len(); i++ {
		if err := polorize(value.Index(i), &awb); err != nil {
			return err
		}
	}

	wb.write(WirePack, awb.load())
	return nil
}

// polorizeMap polorizes a map value into writebuffer
func polorizeMap(value reflect.Value, wb *writebuffer) error {
	// Nil Map
	if value.IsNil() {
		wb.write(WireNull, nil)
		return nil
	}

	// Sort the map keys
	keys := value.MapKeys()
	sort.Slice(keys, sorter(keys))

	// Check if value is a polo.Document, if it is, we must write the data directly
	// into the buffer (with appropriate conversions) and tag the load as a WireDoc
	if value.Type() == reflect.TypeOf(Document{}) {
		var dwb writebuffer
		// Serialize each key (string) and value (bytes)
		for _, k := range keys {
			// Write the key into the buffer
			dwb.write(WireWord, []byte(k.String()))
			// Write the value into the buffer
			dwb.write(WireWord, value.MapIndex(k).Bytes())
		}

		wb.write(WireDoc, dwb.load())
		return nil
	}

	var mwb writebuffer
	// Serialize each key and its value into the writebuffer
	for _, k := range keys {
		// Polorize the key into the buffer
		if err := polorize(k, &mwb); err != nil {
			return err
		}
		// Polorize the value into the buffer
		if err := polorize(value.MapIndex(k), &mwb); err != nil {
			return err
		}
	}

	wb.write(WirePack, mwb.load())
	return nil
}

// polorizeStruct polorizes a struct value into writebuffer
func polorizeStruct(value reflect.Value, wb *writebuffer) error {
	// Get the Type of the value
	t := value.Type()

	// If v is of type big.Int, encode as such
	if t == reflect.TypeOf(*big.NewInt(0)) {
		u, _ := value.Interface().(big.Int)
		wb.write(WireBigInt, u.Bytes())
		return nil
	}

	var swb writebuffer
	// Serialize each field into the writebuffer
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip the field if it is not exported or if it
		// is manually tagged to be skipped with a '-' tag
		if !field.IsExported() || field.Tag.Get("polo") == "-" {
			continue
		}

		if err := polorize(value.Field(i), &swb); err != nil {
			return err
		}
	}

	wb.write(WirePack, swb.load())
	return nil
}

// sorter is used by the sort package to sort a slice of reflect.Value objects.
// Assumes that the reflect.Value objects can only be types which are comparable
// i.e, can be used as a map key. (will panic otherwise)
func sorter(keys []reflect.Value) func(int, int) bool {
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
				result := compare(a.Index(i), b.Index(i))
				if result == 0 {
					continue
				}

				return result < 0
			}

			return false
		}

		panic("unsupported key compare")
	}
}

// compare returns an integer representing the comparison between two reflect.Value objects.
// Assumes that a and b can only have a type that is comparable. (will panic otherwise).
// Returns 1 (a > b); 0 (a == b); -1 (a < b)
func compare(a, b reflect.Value) int {
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
			result := compare(a.Index(i), b.Index(i))
			if result == 0 {
				continue
			}

			return result
		}

		return 0
	}

	panic("unsupported key compare")
}
