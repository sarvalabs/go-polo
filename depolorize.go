package polo

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
)

// depolorize accepts a reflect.Type and a readbuffer attempts to decode
// an object of the given type from the readbuffer and returns it.
func depolorize(t reflect.Type, rb readbuffer) (any, error) {
	switch kind := t.Kind(); kind {

	// Pointer Value
	case reflect.Ptr:
		return depolorizePointer(rb, t)

	// Boolean Value
	case reflect.Bool:
		return depolorizeBool(rb)

	// String Value (UTF-8 Encoded Data, BigEndian)
	case reflect.String:
		return depolorizeString(rb)

	// Unsigned Integer (Binary Encoded Data, BigEndian)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return depolorizeUint(rb, kind)

	// Signed Integer (Binary Encoded Data of the absolute value, BigEndian)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return depolorizeInt(rb, kind)

	// Single Point Float (IEEE-754 Single Point Precision Encoded Data, BigEndian)
	case reflect.Float32:
		return depolorizeFloat32(rb)

	// Double Point Float (IEEE-754 Double Point Precision Encoded Data, BigEndian)
	case reflect.Float64:
		return depolorizeFloat64(rb)

	// Slice Value (Pack Encoded)
	case reflect.Slice:
		return depolorizeSlice(rb, t)

	// Array Value (Pack Encoded)
	case reflect.Array:
		return depolorizeArray(rb, t)

	// Map Value (Pack Encoded. Key-Value. Sorted Keys)
	case reflect.Map:
		return depolorizeMap(rb, t)

	// Struct Value (Field Ordered Pack Encoded)
	case reflect.Struct:
		return depolorizeStruct(rb, t)

	// Unsupported Type
	default:
		return nil, fmt.Errorf("unsupported type: %v [%v]", t, t.Kind())
	}
}

// depolorizeBool decodes a bool value from a readbuffer
func depolorizeBool(rb readbuffer) (bool, error) {
	switch rb.wire {
	// True Value
	case WireTrue:
		return true, nil
	// False Value (Default)
	case WireFalse, WireNull:
		return false, nil
	default:
		return false, IncompatibleWireError{WireTrue, rb.wire}
	}
}

// depolorizeString decodes a string value from a readbuffer
func depolorizeString(rb readbuffer) (string, error) {
	switch rb.wire {
	// Convert []byte to string
	case WireWord:
		return string(rb.data), nil
	// Empty String (Default)
	case WireNull:
		return "", nil
	default:
		return "", IncompatibleWireError{WireWord, rb.wire}
	}
}

// depolorizeUint decodes an unsigned integer value from a readbuffer
func depolorizeUint(rb readbuffer, kind reflect.Kind) (uint64, error) {
	switch rb.wire {
	case WirePosInt:
		switch kind {
		case reflect.Uint8:
			if len(rb.data) > 1 {
				return 0, errors.New("excess data for 8-bit integer")
			}

		case reflect.Uint16:
			if len(rb.data) > 2 {
				return 0, errors.New("excess data for 16-bit integer")
			}

		case reflect.Uint32:
			if len(rb.data) > 4 {
				return 0, errors.New("excess data for 32-bit integer")
			}

		default:
			if len(rb.data) > 8 {
				return 0, errors.New("excess data for 64-bit integer")
			}
		}

		return binary.BigEndian.Uint64(append(make([]byte, 8-len(rb.data), 8), rb.data...)), nil

	// 0 (Default)
	case WireNull:
		return 0, nil
	default:
		return 0, IncompatibleWireError{WirePosInt, rb.wire}
	}
}

// depolorizeInt decodes a signed integer value from a readbuffer
func depolorizeInt(rb readbuffer, kind reflect.Kind) (int64, error) {
	switch rb.wire {
	case WirePosInt, WireNegInt:
		switch kind {
		case reflect.Int8:
			if len(rb.data) > 1 {
				return 0, errors.New("excess data for 8-bit signed integer")
			}

		case reflect.Int16:
			if len(rb.data) > 2 {
				return 0, errors.New("excess data for 16-bit signed integer")
			}

		case reflect.Int32:
			if len(rb.data) > 4 {
				return 0, errors.New("excess data for 32-bit signed integer")
			}

		default:
			if len(rb.data) > 8 {
				return 0, errors.New("excess data for 64-bit signed integer")
			}
		}

		unsigned := binary.BigEndian.Uint64(append(make([]byte, 8-len(rb.data), 8), rb.data...))
		if unsigned > math.MaxInt64 {
			return 0, errors.New("overflow for signed integer")
		}

		// Flip polarity if negative integer
		if rb.wire == WireNegInt {
			return -int64(unsigned), nil
		} else {
			return int64(unsigned), nil
		}

	// 0 (Default)
	case WireNull:
		return 0, nil
	default:
		return 0, IncompatibleWireError{WirePosInt, rb.wire}
	}
}

// depolorizeFloat32 decodes a float32 value from a readbuffer
func depolorizeFloat32(rb readbuffer) (float32, error) {
	switch rb.wire {
	case WireFloat:
		if len(rb.data) != 4 {
			return 0, errors.New("malformed data for 32-bit float")
		}

		// Convert float from IEEE754 binary representation (single point)
		float := math.Float32frombits(binary.BigEndian.Uint32(rb.data))
		if math.IsNaN(float64(float)) {
			return 0, errors.New("float is not a number")
		}

		return float, nil

	// 0 (Default)
	case WireNull:
		return 0, nil
	default:
		return 0, IncompatibleWireError{WireFloat, rb.wire}
	}
}

// depolorizeFloat64 decodes a float64 value from a readbuffer
func depolorizeFloat64(rb readbuffer) (float64, error) {
	switch rb.wire {
	case WireFloat:
		if len(rb.data) != 8 {
			return 0, errors.New("malformed data for 64-bit float")
		}

		// Convert float from IEEE754 binary representation (double point)
		float := math.Float64frombits(binary.BigEndian.Uint64(rb.data))
		if math.IsNaN(float) {
			return 0, errors.New("float is not a number")
		}

		return float, nil

	// 0 (Default)
	case WireNull:
		return 0, nil
	default:
		return 0, IncompatibleWireError{WireFloat, rb.wire}
	}
}

// depolorizeSlice decodes a slice value of type t from a readbuffer
func depolorizeSlice(rb readbuffer, t reflect.Type) (any, error) {
	// Byte Slice
	if t.Elem().Kind() == reflect.Uint8 {
		// Nil Byte Slice
		if rb.wire == WireNull {
			return nil, nil
		}

		return rb.data, nil
	}

	switch rb.wire {
	case WirePack:
		// Convert readbuffer into a loadreader
		load, err := rb.load()
		if err != nil {
			return nil, err
		}

		// Make a new slice
		s := reflect.MakeSlice(t, 0, 0)
		et := t.Elem()

		// Iterate until loadreader is done
		for !load.done() {
			// Get the next element from the load
			element, err := load.next()
			if err != nil {
				return nil, err
			}

			// Depolorize the element into the element type
			sv, err := depolorize(et, element)
			if err != nil {
				return nil, err
			}

			// Create a value based on the nullity of sv
			var val reflect.Value
			if sv == nil {
				// Create a nil value
				val = reflect.New(et).Elem()
			} else {
				// Reflect value of sv and convert type
				val = reflect.ValueOf(sv).Convert(et)
			}

			// Append to slice value
			s = reflect.Append(s, val)
		}

		return s.Interface(), err

	// Nil Value Slice
	case WireNull:
		return reflect.New(t).Elem().Interface(), nil
	default:
		return nil, IncompatibleWireError{WirePack, rb.wire}
	}
}

// depolorizeArray decodes an array value of type t from a readbuffer
func depolorizeArray(rb readbuffer, t reflect.Type) (any, error) {
	// Get the length of the array
	l := t.Len()
	// Create a new array value
	a := reflect.New(t).Elem()

	// Byte Array
	if t.Elem().Kind() == reflect.Uint8 {
		// Unequal Length
		if l != len(rb.data) {
			return nil, errors.New("mismatched data length for byte array")
		}

		// Set each byte in the data into the array
		for i := 0; i < l; i++ {
			a.Index(i).Set(reflect.ValueOf(rb.data[i]))
		}

		return a.Interface(), nil
	}

	switch rb.wire {
	case WirePack:
		// Convert readbuffer into a loadreader
		load, err := rb.load()
		if err != nil {
			return nil, err
		}

		et := t.Elem()

		// Deserialize each element from the readbuffer
		for i := 0; i < l; i++ {
			// Get the next element from the load
			element, err := load.next()
			if err != nil {
				return nil, err
			}

			// Depolorize the element into the element type
			av, err := depolorize(et, element)
			if err != nil {
				return nil, err
			}

			// Create a value based on the nullity of av
			var val reflect.Value
			if av == nil {
				// Create a nil value
				val = reflect.New(et).Elem()
			} else {
				// Reflect value of av and convert type
				val = reflect.ValueOf(av).Convert(et)
			}

			// Set index of array value
			a.Index(i).Set(val)
		}

		return a.Interface(), err

	// Zero Value Array
	case WireNull:
		return reflect.New(t).Elem().Interface(), nil
	default:
		return nil, IncompatibleWireError{WirePack, rb.wire}
	}
}

// depolorizeMap decodes a map value of type t from a readbuffer
func depolorizeMap(rb readbuffer, t reflect.Type) (any, error) {
	// Document Object
	if t == reflect.TypeOf(Document{}) {
		doc := make(Document)

		switch rb.wire {
		case WireDoc:
			// Get the next element from the load
			load, err := rb.load()
			if err != nil {
				return nil, err
			}

			kt := t.Key()
			// Iterate until loadreader is done
			for !load.done() {
				// Get the next element from the load (key)
				keyElement, err := load.next()
				if err != nil {
					return nil, err
				}

				// Get the next element from the load (val)
				valElement, err := load.next()
				if err != nil {
					return nil, err
				}

				// Depolorize the key element into the key type
				k, err := depolorize(kt, keyElement)
				if err != nil {
					return nil, err
				}

				// Create a value for key
				key := reflect.ValueOf(k).Convert(kt)
				// Set the value data into the document for the decoded key
				doc.Set(key.String(), valElement.data)
			}

			return doc, nil

		case WireNull:
			return reflect.New(t).Elem().Interface(), nil
		default:
			return nil, IncompatibleWireError{WireDoc, rb.wire}
		}
	}

	switch rb.wire {
	case WirePack:
		// Convert readbuffer into a loadreader
		load, err := rb.load()
		if err != nil {
			return nil, err
		}

		m := reflect.MakeMap(t)
		kt, vt := t.Key(), t.Elem()

		// Iterate until loadreader is done
		for !load.done() {
			// Get the next element from the load (key)
			keyElement, err := load.next()
			if err != nil {
				return nil, err
			}

			// Get the next element from the load (val)
			valElement, err := load.next()
			if err != nil {
				return nil, err
			}

			// Depolorize the element into the element type (key)
			k, err := depolorize(kt, keyElement)
			if err != nil {
				return nil, err
			}

			// Depolorize the element into the element type (value)
			v, err := depolorize(vt, valElement)
			if err != nil {
				return nil, err
			}

			// Create a value for key
			key := reflect.ValueOf(k).Convert(kt)

			// Create a value for val based on nullity of v
			var val reflect.Value
			if v == nil {
				// Create a nil value
				val = reflect.New(vt).Elem()
			} else {
				// Reflect value of v and convert type
				val = reflect.ValueOf(v).Convert(vt)
			}

			// Set the key-value pair into the map value
			m.SetMapIndex(key, val)
		}

		return m.Interface(), nil

	// Zero Value Map
	case WireNull:
		return reflect.New(t).Elem().Interface(), nil
	default:
		return nil, IncompatibleWireError{WirePack, rb.wire}
	}
}

// depolorizeStruct decodes a struct value of type t from a readbuffer
func depolorizeStruct(rb readbuffer, t reflect.Type) (any, error) {
	// Binary Integer (Big Int)
	if t == reflect.TypeOf(*big.NewInt(0)) {
		var u big.Int

		switch rb.wire {
		case WireBigInt:
			u.SetBytes(rb.data)
			return u, nil //nlreturn:wsl, nlreturn
		case WireNull:
			return nil, ErrNullStruct //nlreturn:wsl, nlreturn
		default:
			return nil, IncompatibleWireError{WireBigInt, rb.wire}
		}
	}

	switch rb.wire {
	case WirePack:
		// Get the next element from the load
		load, err := rb.load()
		if err != nil {
			return nil, err
		}

		v := reflect.New(t).Elem()

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)

			// Skip the field if it is not exported or if it
			// is manually tagged to be skipped with a '-' tag
			if !field.IsExported() || field.Tag.Get("polo") == "-" {
				continue
			}

			element, err := load.next()
			if err != nil {
				return nil, err
			}

			fv, err := depolorize(field.Type, element)
			if err != nil {
				return nil, fmt.Errorf("struct field [%v.%v <%v>]: %w", t, field.Name, field.Type, err)
			}

			if fv != nil {
				v.Field(i).Set(reflect.ValueOf(fv).Convert(field.Type))
			}
		}

		return v.Interface(), nil

	case WireDoc:
		// Decode the wire into a Document
		doc := make(Document)
		if err := Depolorize(&doc, rb.bytes()); err != nil {
			return nil, err
		}

		v := reflect.New(t).Elem()

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)

			// Skip the field if it is not exported or if it
			// is manually tagged to be skipped with a '-' tag
			tag := field.Tag.Get("polo")
			if !field.IsExported() || tag == "-" {
				continue
			}

			// Determine doc key for struct field. Field name is used
			// directly if there is no provided in the polo tag.
			fieldName := field.Name
			if tag != "" {
				fieldName = tag
			}

			// Retrieve the data for the field from the document,
			// if there is no data for the key, skip the field
			data := doc.Get(fieldName)
			if data == nil {
				continue
			}

			frb, err := newreadbuffer(data)
			if err != nil {
				return nil, err
			}

			fv, err := depolorize(field.Type, frb)
			if err != nil {
				return nil, fmt.Errorf("struct field [%v.%v <%v>]: %w", t, field.Name, field.Type, err)
			}

			if fv != nil {
				v.Field(i).Set(reflect.ValueOf(fv).Convert(field.Type))
			}
		}

		return v.Interface(), nil

	// Null Struct
	case WireNull:
		return nil, ErrNullStruct
	default:
		return nil, IncompatibleWireError{WirePack, rb.wire}
	}
}

// depolorizePointer decodes a value of type t from a readbuffer
func depolorizePointer(rb readbuffer, t reflect.Type) (any, error) {
	// recursively call depolorize with the pointer element
	v, err := depolorize(t.Elem(), rb)
	if err != nil {
		// If the returned struct was decoded from null, return a nil
		if errors.Is(err, ErrNullStruct) {
			return nil, nil
		}

		return nil, err
	}

	// Create a new pointer value and set its inner value and return it
	p := reflect.New(t.Elem())
	p.Elem().Set(reflect.ValueOf(v))

	return p.Interface(), nil
}
