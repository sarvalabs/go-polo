package polo

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"reflect"
)

// Depolorizer is a decoding buffer that can sequentially depolorize object from it.
// It can check whether there are elements left in the buffer with Done()
// and peek the WireType of the next element with Peek().
type Depolorizer struct {
	done, packed bool

	data readbuffer
	load *loadreader
}

// NewDepolorizer returns a new Depolorizer for some given bytes.
// Returns an error if the given bytes is malformed for a POLO wire.
//
// If the given data is a compound wire, the only element in the Depolorizer will be the compound data, and
// it will need to be unwrapped into another Depolorizer with DepolorizePacked() before decoding its elements
func NewDepolorizer(data []byte) (*Depolorizer, error) {
	// Create a new readbuffer from the wire
	rb, err := newreadbuffer(data)
	if err != nil {
		return nil, IncompatibleWireError{err.Error()}
	}

	// Create a non-pack Depolorizer
	return &Depolorizer{data: rb}, nil
}

// Done returns whether all elements in the Depolorizer have been read.
func (depolorizer *Depolorizer) Done() bool {
	// Check if loadreader is done if in packed mode
	if depolorizer.packed {
		return depolorizer.load.done()
	}

	// Return flag for non-pack data
	return depolorizer.done
}

// Peek returns the WireType of the next element in the Depolorizer.
// Returns a boolean along with it, which is false if the Depolorizer has no elements left.
func (depolorizer *Depolorizer) Peek() (WireType, bool) {
	// Check if depolorizer contents are done
	if depolorizer.Done() {
		return WireNull, false
	}

	// Peek the next wire from the loadreader if in packed mode
	if depolorizer.packed {
		return depolorizer.load.peek()
	}

	// Return wire of non-pack data
	return depolorizer.data.wire, true
}

// Depolorize decodes a value from the Depolorizer.
// Decodes the data in the wire into the given object using Go reflection.
// Returns an error if the object is not a pointer or if a decode error occurs.
func (depolorizer *Depolorizer) Depolorize(object any) error {
	// Reflect the object value
	value := reflect.ValueOf(object)
	if value.Kind() != reflect.Pointer {
		return ErrObjectNotPtr
	}

	// Obtain the type of the underlying type
	target := value.Type().Elem()
	// Depolorize the next element to the target type
	result, err := depolorizer.depolorizeValue(target)
	if err != nil {
		return err
	} else if result == zeroVal {
		return nil
	}

	// Convert and set the decoded value
	value.Elem().Set(result.Convert(target))
	return nil
}

// DepolorizeNull decodes a null value from the Depolorizer.
// Returns an error if there are no elements left or if the next element is not WireNull.
func (depolorizer *Depolorizer) DepolorizeNull() error {
	// Peek the next wire type
	wire, ok := depolorizer.Peek()
	if !ok {
		return ErrInsufficientWire
	}

	// Error if not WireNull
	if wire != WireNull {
		return IncompatibleWireType(wire, WireNull)
	}

	// Read the element to consume it
	_, _ = depolorizer.read()
	return nil
}

// DepolorizeBytes decodes a bytes value from the Depolorizer.
// Returns an error if there are no elements left or if the next element is not WireWord.
// Returns a nil byte slice if the next element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeBytes() ([]byte, error) {
	// Peek the next wire type
	wire, ok := depolorizer.Peek()
	if !ok {
		return nil, ErrInsufficientWire
	}

	switch wire {
	case WireWord:
		data, err := depolorizer.read()
		if err != nil {
			return nil, err
		}

		return data.data, nil

	// Nil Byte Slice (Default)
	case WireNull:
		// Read the element to consume it
		_, _ = depolorizer.read()
		return nil, nil

	default:
		return nil, IncompatibleWireType(wire, WireNull, WireWord)
	}
}

// DepolorizeString decodes a string value from the Depolorizer.
// Returns an error if there are no elements left or if the next element is not WireWord.
// Returns an empty string if the next element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeString() (string, error) {
	// Peek the next wire type
	wire, ok := depolorizer.Peek()
	if !ok {
		return "", ErrInsufficientWire
	}

	switch wire {
	// Convert []byte to string
	case WireWord:
		data, err := depolorizer.read()
		if err != nil {
			return "", err
		}

		return string(data.data), nil

	// Empty String (Default)
	case WireNull:
		// Read the element to consume it
		_, _ = depolorizer.read()
		return "", nil

	default:
		return "", IncompatibleWireType(wire, WireNull, WireWord)
	}
}

// DepolorizeBool decodes a bool value from the Depolorizer.
// Returns an error if there are no elements left or if the next element is not WireTrue or WireFalse.
// Returns a false if the next element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeBool() (bool, error) {
	// Peek the next wire type
	wire, ok := depolorizer.Peek()
	if !ok {
		return false, ErrInsufficientWire
	}

	switch wire {
	// True Value
	case WireTrue:
		// Read the element to consume it
		_, _ = depolorizer.read()
		return true, nil

	// False Value (Default)
	case WireFalse, WireNull:
		// Read the element to consume it
		_, _ = depolorizer.read()
		return false, nil

	default:
		return false, IncompatibleWireType(wire, WireNull, WireTrue, WireFalse)
	}
}

// DepolorizeUint decodes an unsigned integer value from the Depolorizer.
// Returns an error if there are no elements left or if the next element is not WirePosInt.
// Returns 0 if the next element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeUint() (uint64, error) {
	// Depolorize an unsigned 64-bit integer
	number, err := depolorizer.depolorizeInteger(false, 64)
	if err != nil {
		return 0, err
	}

	// Return the integer as an uint64
	return number.(uint64), nil
}

// DepolorizeInt decodes a signed integer value from the Depolorizer.
// Returns an error if there are no elements left or if the next element is not WirePosInt or WireNegInt.
// Returns 0 if the next element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeInt() (int64, error) {
	// Depolorize a signed 64-bit integer
	number, err := depolorizer.depolorizeInteger(true, 64)
	if err != nil {
		return 0, err
	}

	// Return the integer as an int64
	return number.(int64), nil
}

// DepolorizeFloat32 decodes a single point precision float from the Depolorizer.
// Returns an error if there are no elements left or if the next element is not WireFloat.
// Returns 0 if the next element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeFloat32() (float32, error) {
	// Peek the next wire type
	wire, ok := depolorizer.Peek()
	if !ok {
		return 0, ErrInsufficientWire
	}

	switch wire {
	case WireFloat:
		data, err := depolorizer.read()
		if err != nil {
			return 0, err
		}

		//
		if len(data.data) != 4 {
			return 0, IncompatibleWireError{"malformed data for 32-bit float"}
		}

		// Convert float from IEEE754 binary representation (single point)
		float := math.Float32frombits(binary.BigEndian.Uint32(data.data))
		if math.IsNaN(float64(float)) {
			return 0, IncompatibleValueError{"float is not a number"}
		}

		return float, nil

	// 0 (Default)
	case WireNull:
		_, _ = depolorizer.read()
		return 0, nil
	default:
		return 0, IncompatibleWireType(wire, WireNull, WireFloat)
	}
}

// DepolorizeFloat64 decodes a double point precision float from the Depolorizer.
// Returns an error if there are no elements left or if the next element is not WireFloat.
// Returns 0 if the next element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeFloat64() (float64, error) {
	// Peek the next wire type
	wire, ok := depolorizer.Peek()
	if !ok {
		return 0, ErrInsufficientWire
	}

	switch wire {
	case WireFloat:
		data, err := depolorizer.read()
		if err != nil {
			return 0, err
		}

		if len(data.data) != 8 {
			return 0, IncompatibleWireError{"malformed data for 64-bit float"}
		}

		// Convert float from IEEE754 binary representation (double point)
		float := math.Float64frombits(binary.BigEndian.Uint64(data.data))
		if math.IsNaN(float) {
			return 0, IncompatibleValueError{"float is not a number"}
		}

		return float, nil

	// 0 (Default)
	case WireNull:
		_, _ = depolorizer.read()
		return 0, nil
	default:
		return 0, IncompatibleWireType(wire, WireNull, WireFloat)
	}
}

// DepolorizeBigInt decodes a big.Int from the Depolorizer.
// Returns an error if there are no elements left or if the next element is not WireBigInt.
// Returns nil if the next element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeBigInt() (*big.Int, error) {
	wire, ok := depolorizer.Peek()
	if !ok {
		return nil, ErrInsufficientWire
	}

	switch wire {
	case WireBigInt:
		data, err := depolorizer.read()
		if err != nil {
			return nil, err
		}

		return new(big.Int).SetBytes(data.data), nil

		// 0 (Default)
	case WireNull:
		_, _ = depolorizer.read()
		return nil, nil

	default:
		return nil, IncompatibleWireType(wire, WireNull, WireBigInt)
	}
}

// DepolorizePacked decodes another Depolorizer from the Depolorizer.
// Returns an error if there are no elements left or if the next element is not WirePack or WireDoc.
// If the next element is a WireNull, returns a ErrNullPack error.
func (depolorizer *Depolorizer) DepolorizePacked() (*Depolorizer, error) {
	// Peek the next wire type
	wire, ok := depolorizer.Peek()
	if !ok {
		return nil, ErrInsufficientWire
	}

	switch wire {
	case WirePack, WireDoc:
		data, err := depolorizer.read()
		if err != nil {
			return nil, err
		}

		// Convert the element into a loadreader
		load, err := data.load()
		if err != nil {
			return nil, err
		}

		// Create a new Depolorizer in packed mode
		return &Depolorizer{load: load, packed: true}, nil

	case WireNull:
		_, _ = depolorizer.read()
		return nil, ErrNullPack

	default:
		return nil, IncompatibleWireType(wire, WireNull, WirePack, WireDoc)
	}
}

// DepolorizeDocument decodes a Document from the Depolorizer.
// Returns an error if there are no elements left, if the next element is not WireDoc.
// Returns nil value if the next element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeDocument() (Document, error) {
	// Peek the wire type of the next element
	wire, ok := depolorizer.Peek()
	if !ok {
		return nil, ErrInsufficientWire
	}

	switch wire {
	case WireDoc:
		// Get the next element as a pack
		// depolorizer with the slice elements
		pack, err := depolorizer.DepolorizePacked()
		if err != nil {
			return nil, err
		}

		doc := make(Document)

		// Iterate on the pack until done
		for !pack.Done() {
			// Depolorize the next object from the pack into the Document key (string)
			docKey, err := pack.DepolorizeString()
			if err != nil {
				return nil, err
			}

			// Read the next object from the pack
			val, err := pack.read()
			if err != nil {
				return nil, err
			}

			// Set the value bytes into the document for the decoded key
			doc.Set(docKey, val.data)
		}

		return doc, nil

	// Nil Document
	case WireNull:
		_, _ = depolorizer.read()
		return nil, nil
	default:
		return nil, IncompatibleWireType(wire, WireNull, WireDoc)
	}
}

// depolorizeInner decodes another Depolorizer from the Depolorizer.
// Unlike DepolorizePacked which will expect a compound element and convert it into a packed Depolorizer,
// depolorizeInner will return the atomic element as an atomic Depolorizer.
func (depolorizer *Depolorizer) depolorizeInner() (*Depolorizer, error) {
	if depolorizer.Done() {
		return nil, ErrInsufficientWire
	}

	data, err := depolorizer.read()
	if err != nil {
		return nil, err
	}

	// Create a non-pack Depolorizer
	return &Depolorizer{data: data}, nil
}

// depolorizeInteger decodes an integer from the Depolorizer.
// Accepts whether the integer should be signed and its bit-size (8, 16, 32 or 64)
func (depolorizer *Depolorizer) depolorizeInteger(signed bool, size int) (any, error) {
	// Peek the next wire type
	wire, ok := depolorizer.Peek()
	if !ok {
		return nil, ErrInsufficientWire
	}

	// Check that wire is either WirePosInt, WireNegInt or WireNull
	if !(wire == WirePosInt || wire == WireNegInt || wire == WireNull) {
		expects := []WireType{WireNull, WirePosInt}
		if signed {
			expects = append(expects, WireNegInt)
		}

		return 0, IncompatibleWireType(wire, expects...)
	}

	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return 0, err
	}

	// Check that bit-size value is valid
	if !isBitSize(size) {
		panic("invalid bit-size for integer decode")
	}

	// Check that the data does not overflow for bit-size
	if len(data.data) > size/8 {
		return 0, IncompatibleValueError{fmt.Sprintf("excess data for %v-bit integer", size)}
	}

	// Decode the data into a uint64
	number := binary.BigEndian.Uint64(append(make([]byte, 8-len(data.data), 8), data.data...))
	if signed {
		// Check that number is within bounds for signed integer
		if number > math.MaxInt64 {
			return 0, IncompatibleValueError{"overflow for signed integer"}
		}
	}

	switch wire {
	case WirePosInt: // do nothing
	case WireNegInt:
		if !signed {
			return 0, IncompatibleWireType(wire, WireNull, WirePosInt)
		}

		// Flip polarity if negative integer
		return -int64(number), nil

	case WireNull:
		number = 0
	}

	// if number is signed
	if signed {
		return int64(number), nil
	}

	return number, nil
}

// depolorizeByteArrayValue accepts a reflect.Type and decodes a byte array from the Depolorizer.
// The target must be an array of bytes and the next value in the Depolorizer must be a WireWord.
func (depolorizer *Depolorizer) depolorizeByteArrayValue(target reflect.Type) (reflect.Value, error) {
	// Depolorize a bytes value
	bytes, err := depolorizer.DepolorizeBytes()
	if err != nil {
		return zeroVal, err
	}

	// Check array length
	if target.Len() != len(bytes) {
		return zeroVal, IncompatibleWireError{"mismatched data length for byte array"}
	}

	// Create a Byte Array Value
	array := reflect.New(target).Elem()
	// Copy array contents from the slice into bytes
	reflect.Copy(array, reflect.ValueOf(bytes))

	return array, nil
}

// depolorizeSliceValue accepts a reflect.Type and decodes a value from the Depolorizer into it.
// The target type must be a slice and the next wire element must be WirePack.
func (depolorizer *Depolorizer) depolorizeSliceValue(target reflect.Type) (reflect.Value, error) {
	// Peek the wire type of the next element
	wire, ok := depolorizer.Peek()
	if !ok {
		return zeroVal, ErrInsufficientWire
	}

	switch wire {
	case WirePack:
		// Get the next element as a pack
		// depolorizer with the slice elements
		pack, err := depolorizer.DepolorizePacked()
		if err != nil {
			return zeroVal, err
		}

		// Make a new slice
		slice := reflect.MakeSlice(target, 0, 0)
		sliceElem := target.Elem()

		// Iterate on the pack until done
		for !pack.Done() {
			// Depolorize the next object from the pack into the element type
			val, err := pack.depolorizeValue(sliceElem)
			if err != nil {
				return zeroVal, err
			}

			// Create a value based on the nullity of val
			var sliceVal reflect.Value
			if val == zeroVal {
				fmt.Println("here")

				// Create a nil value
				sliceVal = reflect.New(sliceElem).Elem()
			} else {
				// Reflect value of val and convert type
				sliceVal = val.Convert(sliceElem)
			}

			// Append to slice value
			slice = reflect.Append(slice, sliceVal)
		}

		return slice, nil

	// Nil Value Slice
	case WireNull:
		_, _ = depolorizer.read()
		return reflect.New(target).Elem(), nil
	default:
		return zeroVal, IncompatibleWireType(wire, WireNull, WirePack)
	}
}

// depolorizeArrayValue accepts a reflect.Type and decodes a value from the Depolorizer into it.
// The target type must be an array and the next wire element must be WirePack.
func (depolorizer *Depolorizer) depolorizeArrayValue(target reflect.Type) (reflect.Value, error) {
	// Peek the wire type of the next element
	wire, ok := depolorizer.Peek()
	if !ok {
		return zeroVal, ErrInsufficientWire
	}

	switch wire {
	case WirePack:
		// Get the next element as a pack
		// depolorizer with the slice elements
		pack, err := depolorizer.DepolorizePacked()
		if err != nil {
			return zeroVal, err
		}

		// Get the array length and element type
		arrayLen := target.Len()
		arrayElem := target.Elem()

		// Create a new array
		array := reflect.New(target).Elem()

		// Iterate on array indices
		for index := 0; index < arrayLen; index++ {
			// Depolorize the next object from the pack into the element type
			val, err := pack.depolorizeValue(arrayElem)
			if err != nil {
				return zeroVal, err
			}

			// Create a value based on the nullity of val
			var arrayVal reflect.Value
			if val == zeroVal {
				// Create a nil value
				arrayVal = reflect.New(arrayElem).Elem()
			} else {
				// Reflect value of val and convert type
				arrayVal = val.Convert(arrayElem)
			}

			// Set index of array value
			array.Index(index).Set(arrayVal)
		}

		return array, nil

	// Nil Value Slice
	case WireNull:
		_, _ = depolorizer.read()
		return reflect.New(target).Elem(), nil
	default:
		return zeroVal, IncompatibleWireType(wire, WireNull, WirePack)
	}
}

// depolorizeMapValue accepts a reflect.Type and decodes a value from the Depolorizer into it.
// The target type must be a map and the next wire element must be WirePack.
func (depolorizer *Depolorizer) depolorizeMapValue(target reflect.Type) (reflect.Value, error) {
	// Peek the wire type of the next element
	wire, ok := depolorizer.Peek()
	if !ok {
		return zeroVal, ErrInsufficientWire
	}

	switch wire {
	case WirePack:
		// Get the next element as a pack
		// depolorizer with the slice elements
		pack, err := depolorizer.DepolorizePacked()
		if err != nil {
			return zeroVal, err
		}

		mapping := reflect.MakeMap(target)
		keyType, valType := target.Key(), target.Elem()

		// Iterate on the pack until done
		for !pack.Done() {
			// Depolorize the next object from the pack into the map key type
			key, err := pack.depolorizeValue(keyType)
			if err != nil {
				return zeroVal, err
			}

			// Depolorize the next object from the pack into the map value type
			val, err := pack.depolorizeValue(valType)
			if err != nil {
				return zeroVal, err
			}

			// Create a value for key
			mapKey := key.Convert(keyType)

			// Create a value for val based on nullity of v
			var mapVal reflect.Value
			if val == zeroVal {
				// Create a nil value
				mapVal = reflect.New(valType).Elem()
			} else {
				// Reflect value of v and convert type
				mapVal = val.Convert(valType)
			}

			// Set the key-value pair into the map value
			mapping.SetMapIndex(mapKey, mapVal)
		}

		return mapping, nil

	// Zero Value Map
	case WireNull:
		_, _ = depolorizer.read()
		return reflect.New(target).Elem(), nil
	default:
		return zeroVal, IncompatibleWireType(wire, WireNull, WirePack)
	}
}

// depolorizeStructValue accepts a reflect.Type and decodes a value from the Depolorizer into it.
// The target type must be a struct and the next wire element must be WirePack or WireDoc.
func (depolorizer *Depolorizer) depolorizeStructValue(target reflect.Type) (reflect.Value, error) {
	// Peek the wire type of the next element
	wire, ok := depolorizer.Peek()
	if !ok {
		return zeroVal, ErrInsufficientWire
	}

	switch wire {
	case WirePack:
		// Get the next element as a pack
		// depolorizer with the slice elements
		pack, err := depolorizer.DepolorizePacked()
		if err != nil {
			return zeroVal, err
		}

		// Create a new struct instance
		structure := reflect.New(target).Elem()

		// Iterate on struct fields
		for index := 0; index < target.NumField(); index++ {
			// Obtain field data for field index
			field := target.Field(index)

			// Skip the field if it is not exported or if it
			// is manually tagged to be skipped with a '-' tag
			if !field.IsExported() || field.Tag.Get("polo") == "-" {
				continue
			}

			// Depolorize the next object from the pack into the map key type
			val, err := pack.depolorizeValue(field.Type)
			if err != nil {
				return zeroVal, IncompatibleWireError{fmt.Sprintf("struct field [%v.%v <%v>]: %v", target, field.Name, field.Type, err)}
			}

			if val != zeroVal {
				structure.Field(index).Set(val.Convert(field.Type))
			}
		}

		return structure, nil

	case WireDoc:
		doc, err := depolorizer.DepolorizeDocument()
		if err != nil {
			return zeroVal, err
		}

		// Create a new struct instance
		structure := reflect.New(target).Elem()

		// Iterate on struct fields
		for index := 0; index < target.NumField(); index++ {
			// Obtain field data for field index
			field := target.Field(index)

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

			object, err := NewDepolorizer(data)
			if err != nil {
				return zeroVal, err
			}

			fieldVal, err := object.depolorizeValue(field.Type)
			if err != nil {
				return zeroVal, IncompatibleWireError{fmt.Sprintf("struct field [%v.%v <%v>]: %v", target, field.Name, field.Type, err)}
			}

			if fieldVal != zeroVal {
				structure.Field(index).Set(fieldVal.Convert(field.Type))
			}
		}

		return structure, nil

	// Null Struct
	case WireNull:
		_, _ = depolorizer.read()
		return zeroVal, nil

	default:
		return zeroVal, IncompatibleWireType(wire, WireNull, WirePack, WireDoc)
	}
}

// depolorizePointer decodes a value of type target from the Depolorizer
func (depolorizer *Depolorizer) depolorizePointer(target reflect.Type) (reflect.Value, error) {
	// recursively call depolorize with the pointer element
	value, err := depolorizer.depolorizeValue(target.Elem())
	if err != nil {
		return zeroVal, err
	}

	// Handle ZeroVal
	if value == zeroVal {
		return zeroVal, nil
	}

	// Create a new pointer value and set its inner value and return it
	p := reflect.New(target.Elem())
	p.Elem().Set(value)

	return p, nil
}

// depolorizeDepolorizable decodes value of type target from the Depolorizer.
// The target type must implement the Depolorizable interface.
func (depolorizer *Depolorizer) depolorizeDepolorizable(target reflect.Type) (reflect.Value, error) {
	// Create a value for the target type
	value := reflect.New(target)

	// Retrieve the next element as a Depolorizer
	inner, err := depolorizer.depolorizeInner()
	if err != nil {
		return zeroVal, err
	}

	// Call the Depolorize method of Depolorizable (accepts a Depolorizer and returns an error)
	outputs := value.MethodByName("Depolorize").Call([]reflect.Value{reflect.ValueOf(inner)})
	if !outputs[0].IsNil() {
		return zeroVal, outputs[0].Interface().(error)
	}

	return value.Elem(), nil
}

// depolorizeValue accepts a reflect.Type and decodes a value from the Depolorizer into it.
// The target type can be any type apart from interfaces, channels and functions.
func (depolorizer *Depolorizer) depolorizeValue(target reflect.Type) (reflect.Value, error) {
	// Depolorizable Type
	if reflect.PointerTo(target).Implements(reflect.TypeOf((*Depolorizable)(nil)).Elem()) {
		return depolorizer.depolorizeDepolorizable(target)
	}

	switch kind := target.Kind(); kind {

	// Pointer Value
	case reflect.Ptr:
		return depolorizer.depolorizePointer(target)

	// Boolean Value
	case reflect.Bool:
		return reflected(depolorizer.DepolorizeBool())

	// String Value
	case reflect.String:
		return reflected(depolorizer.DepolorizeString())

	// Integer Value
	case reflect.Uint8:
		return reflected(depolorizer.depolorizeInteger(false, 8))
	case reflect.Int8:
		return reflected(depolorizer.depolorizeInteger(true, 8))
	case reflect.Uint16:
		return reflected(depolorizer.depolorizeInteger(false, 16))
	case reflect.Int16:
		return reflected(depolorizer.depolorizeInteger(true, 16))
	case reflect.Uint32:
		return reflected(depolorizer.depolorizeInteger(false, 32))
	case reflect.Int32:
		return reflected(depolorizer.depolorizeInteger(true, 32))
	case reflect.Uint, reflect.Uint64:
		return reflected(depolorizer.depolorizeInteger(false, 64))
	case reflect.Int, reflect.Int64:
		return reflected(depolorizer.depolorizeInteger(true, 64))

	// Single Point Float
	case reflect.Float32:
		return reflected(depolorizer.DepolorizeFloat32())

	// Double Point Float
	case reflect.Float64:
		return reflected(depolorizer.DepolorizeFloat64())

	// Slice Value
	case reflect.Slice:
		// Byte Slice
		if target.Elem().Kind() == reflect.Uint8 {
			return reflected(depolorizer.DepolorizeBytes())
		}

		return depolorizer.depolorizeSliceValue(target)

	// Array Value
	case reflect.Array:
		// Byte Array
		if target.Elem().Kind() == reflect.Uint8 {
			return depolorizer.depolorizeByteArrayValue(target)
		}

		return depolorizer.depolorizeArrayValue(target)

	// Map Value (Pack Encoded. Key-Value. Sorted Keys)
	case reflect.Map:
		// Document
		if target == reflect.TypeOf(Document{}) {
			return reflected(depolorizer.DepolorizeDocument())
		}

		return depolorizer.depolorizeMapValue(target)

	// Struct Value (Field Ordered Pack Encoded)
	case reflect.Struct:
		// BigInt
		if target == reflect.TypeOf(*big.NewInt(0)) {
			bigint, err := depolorizer.DepolorizeBigInt()
			if bigint == nil {
				return zeroVal, err
			}

			return reflected(*bigint, err)
		}

		return depolorizer.depolorizeStructValue(target)

	// Unsupported Type
	default:
		return zeroVal, UnsupportedTypeError(target)
	}
}

// read returns the next element in the Depolorizer as a readbuffer.
// If it is in packed mode, it reads from the loadreader, otherwise
// it returns the readbuffer data and set the done flag.
func (depolorizer *Depolorizer) read() (readbuffer, error) {
	// Check if there is another element to read
	if depolorizer.Done() {
		return readbuffer{}, ErrInsufficientWire
	}

	// Read from the loadreader if in packed mode
	if depolorizer.packed {
		return depolorizer.load.next()
	}

	// Set the atomic read flag to done
	depolorizer.done = true
	// Return the data from the atomic buffer
	return depolorizer.data, nil
}

// reflected is a helper function that accepts an arbitrary object and an error.
// It returns the reflect.ValueOf of the object and the same error
func reflected[T any](value T, err error) (reflect.Value, error) {
	return reflect.ValueOf(value), err
}
