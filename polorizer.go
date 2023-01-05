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

// Polorizer is an encoding buffer
type Polorizer struct {
	wb *writebuffer
}

// NewPolorizer creates a new Polorizer
func NewPolorizer() *Polorizer {
	return &Polorizer{wb: &writebuffer{}}
}

func (polorizer *Polorizer) Bytes() []byte {
	switch polorizer.wb.counter {
	case 0:
		return []byte{0}
	case 1:
		return polorizer.wb.bytes()
	default:
		return polorizer.Packed()
	}
}

func (polorizer *Polorizer) Packed() []byte {
	// Declare a new writebuffer
	var wb writebuffer
	// Write the contents of the polorized buffer
	// into the writebuffer and tag with WirePack
	wb.write(WirePack, polorizer.wb.load())

	return wb.bytes()
}

// Polorize encodes a value into the Polorizer.
// Encodes the object based on its type using the Go reflection.
func (polorizer *Polorizer) Polorize(value any) error {
	return polorizer.polorizeValue(reflect.ValueOf(value))
}

// PolorizeNull encodes a null value into the Polorizer.
// Encodes a WireNull into the head, consuming a position on the wire.
func (polorizer *Polorizer) PolorizeNull() {
	polorizer.wb.write(WireNull, nil)
}

// PolorizeBool encodes a bool value into the Polorizer.
// Encodes the boolean as either WireTrue or WireFalse, depending on its value.
func (polorizer *Polorizer) PolorizeBool(value bool) {
	var wiretype = WireFalse
	if value {
		wiretype = WireTrue
	}

	polorizer.wb.write(wiretype, nil)
}

// PolorizeString encodes a string value into the Polorizer.
// Encodes the string as its UTF-8 encoded bytes with the wire type being WireWord.
func (polorizer *Polorizer) PolorizeString(value string) {
	polorizer.wb.write(WireWord, []byte(value))
}

// PolorizeBytes encodes a bytes value into the Polorizer.
// Encodes the bytes as is with the wire type being WireWord.
func (polorizer *Polorizer) PolorizeBytes(value []byte) {
	polorizer.wb.write(WireWord, value)
}

// PolorizeUint encodes a signed integer value into the Polorizer.
// Encodes the integer as it's the binary form (big-endian) with the wire type being WirePosInt.
func (polorizer *Polorizer) PolorizeUint(value uint64) {
	if value == 0 {
		polorizer.wb.write(WirePosInt, nil)
		return
	}

	var buffer [8]byte

	binary.BigEndian.PutUint64(buffer[:], value)
	polorizer.wb.write(WirePosInt, buffer[8-intsize(value):])
}

// PolorizeInt encodes a signed integer value into the Polorizer.
// Encodes the integer as the binary form of its absolute value with the wire type
// being WirePosInt or WireBigInt based on polarity, with zero considered as positive.
func (polorizer *Polorizer) PolorizeInt(value int64) {
	if value == 0 {
		polorizer.wb.write(WirePosInt, nil)
		return
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
	polorizer.wb.write(wiretype, buffer[8-intsize(unsigned):])
}

// PolorizeFloat32 encodes a single point precision float into the Polorizer.
// Encodes the float as its IEEE754 binary form (big-endian) with the wire type being WireFloat.
func (polorizer *Polorizer) PolorizeFloat32(value float32) {
	var buffer [4]byte

	// Convert float into IEEE754 binary representation (single point)
	binary.BigEndian.PutUint32(buffer[:], math.Float32bits(value))
	polorizer.wb.write(WireFloat, buffer[:])
}

// PolorizeFloat64 encodes a double point precision float into the Polorizer.
// Encodes the float as its IEEE754 binary form (big-endian) with the wire type being WireFloat.
func (polorizer *Polorizer) PolorizeFloat64(value float64) {
	var buffer [8]byte

	// Convert float into IEEE754 binary representation (double point)
	binary.BigEndian.PutUint64(buffer[:], math.Float64bits(value))
	polorizer.wb.write(WireFloat, buffer[:])
}

// PolorizeBigInt encodes a big.Int into the Polorizer.
// Encodes the big.Int as its binary form with the wire type being WirePosInt
// or WireBigInt based on polarity, with zero considered as positive.
func (polorizer *Polorizer) PolorizeBigInt(value *big.Int) {
	polorizer.wb.write(WireBigInt, value.Bytes())
}

// PolorizePacked encodes the contents of another Polorizer as pack-encoded data.
// The contents are packed into a WireLoad message and tagged with the WirePack wire type.
func (polorizer *Polorizer) PolorizePacked(value *Polorizer) {
	polorizer.wb.write(WirePack, value.wb.load())
}

// PolorizeArray encodes an array or slice into the Polorizer.
// Encodes the array/slice elements as POLO pack-encoded data with the wire type being WirePack.
// Returns an error if value is not an array/slice or if the array elements cannot be encoded.
// If the value is a nil slice, it is encoded as WireNull.
func (polorizer *Polorizer) PolorizeArray(value any) error {
	// Reflect on the given value
	v := reflect.ValueOf(value)
	// Check value kind
	switch v.Kind() {
	case reflect.Array:
	case reflect.Slice:
		// Nil Slice
		if v.IsNil() {
			polorizer.PolorizeNull()
			return nil
		}

	default:
		// Not an array or slice
		return errors.New("PolorizeArray: value is not an array or slice")
	}

	return polorizer.polorizeArrayValue(v)
}

// PolorizeMap encodes a map into the Polorizer.
// Encodes the map keys and values as POLO pack-encoded data with the wire type being WirePack.
// Returns an error if the value is not a map or if the map keys or values cannot be encoded.
// If the value is a nil map, it is encoded as a WireNull.
func (polorizer *Polorizer) PolorizeMap(value any) error {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Map {
		return errors.New("PolorizeMap: value is not a map")
	}

	// Nil Map
	if v.IsNil() {
		polorizer.PolorizeNull()
		return nil
	}

	return polorizer.polorizeMapValue(v)
}

// PolorizeDocument encodes a Document into the Polorizer.
// Encodes the Document keys and raw values as POLO doc-encoded data with the wire type being WireDoc.
// If the Document is nil, it is encoded as a WireNull.
func (polorizer *Polorizer) PolorizeDocument(document Document) {
	// Nil Document
	if document == nil {
		polorizer.PolorizeNull()
		return
	}

	// Collect all the document keys
	keys := make([]string, 0, document.Size())
	for key := range document {
		keys = append(keys, key)
	}

	// Sort the document keys
	sort.Strings(keys)
	// Create a new polorizer for the document elements
	documentWire := NewPolorizer()

	// Serialize each key (string) and value (bytes)
	for _, key := range keys {
		// Write the document key
		documentWire.PolorizeString(key)
		// Write the document value
		documentWire.PolorizeBytes(document[key])
	}

	// Wrap the document polorizer contents as a WireLoad and
	// write to the Polorizer with the WireDoc tag
	polorizer.wb.write(WireDoc, documentWire.wb.load())
}

// PolorizeStruct encodes a struct into the Polorizer.
// Encodes the struct fields as POLO pack-encoded data with the wire type being WirePack.
// Returns an error if value is not a struct or if its fields cannot be encoded.
func (polorizer *Polorizer) PolorizeStruct(value any) error {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Map {
		return errors.New("PolorizeStruct: value is not a struct")
	}

	return polorizer.polorizeStructValue(v)
}

// polorizeByteArrayValue accepts a reflect.Value and encodes it into the Polorizer.
// The value must be an array of bytes and is encoded as WireWord.
func (polorizer *Polorizer) polorizeByteArrayValue(value reflect.Value) {
	// Determine array element type (uint8) and size
	arrsize, arrelem := value.Len(), value.Type().Elem()
	// Create a Byte Slice Value
	slice := reflect.MakeSlice(reflect.SliceOf(arrelem), arrsize, arrsize)
	// Copy array contents into slice
	reflect.Copy(slice, value)

	polorizer.PolorizeBytes(slice.Bytes())
}

// polorizeArrayValue accepts a reflect.Value and encodes it into the Polorizer.
// The value must be an array or slice and is encoded as element pack encoded data.
func (polorizer *Polorizer) polorizeArrayValue(value reflect.Value) error {
	array := NewPolorizer()

	// Serialize each element into the writebuffer
	for i := 0; i < value.Len(); i++ {
		if err := array.polorizeValue(value.Index(i)); err != nil {
			return err
		}
	}

	polorizer.PolorizePacked(array)
	return nil
}

// polorizeMapValue accepts a reflect.Value and encodes it into the Polorizer.
// The value must be a map and is encoded as key-value pack encoded data.
// Map keys are sorted before being sequentially encoded.
func (polorizer *Polorizer) polorizeMapValue(value reflect.Value) error {
	// Sort the map keys
	keys := value.MapKeys()
	sort.Slice(keys, sorter(keys))

	// Create a new polorizer for the map elements
	mapping := NewPolorizer()
	// Serialize each key and its value into the polorizer
	for _, k := range keys {
		// Polorize the key into the buffer
		if err := mapping.polorizeValue(k); err != nil {
			return err
		}
		// Polorize the value into the buffer
		if err := mapping.polorizeValue(value.MapIndex(k)); err != nil {
			return err
		}
	}

	polorizer.PolorizePacked(mapping)
	return nil
}

// polorizeStructValue accepts a reflect.Value and encodes it into the Polorizer.
// The value must be a struct and is encoded as field ordered pack encoded data.
func (polorizer *Polorizer) polorizeStructValue(value reflect.Value) error {
	// Get the Type of the value
	t := value.Type()

	structure := NewPolorizer()
	// Serialize each field into the writebuffer
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip the field if it is not exported or if it
		// is manually tagged to be skipped with a '-' tag
		if !field.IsExported() || field.Tag.Get("polo") == "-" {
			continue
		}

		if err := structure.polorizeValue(value.Field(i)); err != nil {
			return err
		}
	}

	polorizer.PolorizePacked(structure)
	return nil
}

// polorizeValue accepts a reflect.Value and encodes it into the Polorizer.
// The underlying value can be any type apart from interfaces, channels and functions.
func (polorizer *Polorizer) polorizeValue(value reflect.Value) (err error) {
	// Check the kind of value
	switch kind := value.Kind(); kind {

	// Pointer
	case reflect.Ptr:
		if value.IsNil() {
			polorizer.PolorizeNull()
			return nil
		}

		return polorizer.polorizeValue(value.Elem())

	// Boolean
	case reflect.Bool:
		polorizer.PolorizeBool(value.Bool())

	// String
	case reflect.String:
		polorizer.PolorizeString(value.String())

	// Unsigned Integer
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		polorizer.PolorizeUint(value.Uint())

	// Signed Integer
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		polorizer.PolorizeInt(value.Int())

	// Single Point Float
	case reflect.Float32:
		polorizer.PolorizeFloat32(float32(value.Float()))

	// Double Point Float
	case reflect.Float64:
		polorizer.PolorizeFloat64(value.Float())

	// Slice Value (Pack Encoded)
	case reflect.Slice:
		// Nil Slice
		if value.IsNil() {
			polorizer.PolorizeNull()
			return nil
		}

		// Byte Slice
		if value.Type().Elem().Kind() == reflect.Uint8 {
			polorizer.PolorizeBytes(value.Bytes())
			return nil
		}

		return polorizer.polorizeArrayValue(value)

	// Array Value (Pack Encoded)
	case reflect.Array:
		// Byte Array
		if value.Type().Elem().Kind() == reflect.Uint8 {
			polorizer.polorizeByteArrayValue(value)
			return nil
		}

		return polorizer.polorizeArrayValue(value)

	// Map Value (Pack Encoded. Key-Value. Sorted Keys)
	case reflect.Map:
		// Nil Map
		if value.IsNil() {
			polorizer.PolorizeNull()
			return nil
		}

		// Check if value is a polo.Document and encode as such
		if value.Type() == reflect.TypeOf(Document{}) {
			document := value.Interface().(Document)
			polorizer.PolorizeDocument(document)
			return nil
		}

		return polorizer.polorizeMapValue(value)

	// Struct Value (Field Ordered Pack Encoded)
	case reflect.Struct:
		// Check if value is a big.Int and encode as such
		if value.Type() == reflect.TypeOf(*big.NewInt(0)) {
			bignumber, _ := value.Interface().(big.Int)
			polorizer.PolorizeBigInt(&bignumber)
			return nil
		}

		return polorizer.polorizeStructValue(value)

	// Unsupported Type
	default:
		// Create a recovery handler to check if v.Type() panicked.
		// This will occur if v is zero Value for an abstract nil.
		defer func() {
			if recover() != nil {
				err = errors.New("unsupported type: cannot encode abstract nil")
			}
		}()

		return fmt.Errorf("unsupported type: %v [%v]", value.Type(), value.Type().Kind())
	}

	return nil
}

// intsize returns the min number of bytes
// required to represent an unsigned 64-bit integer.
func intsize(v uint64) int {
	return (bits.Len64(v) + 8 - 1) / 8
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