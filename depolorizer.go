package polo

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
)

// Depolorizer is a decoding buffer that can sequentially depolorize object from it.
// It can check whether there are elements left in the buffer with Done()
// and peek the WireType of the next element with Peek().
type Depolorizer struct {
	done, packed bool

	data readbuffer
	pack *packbuffer

	cfg wireConfig
}

// NewDepolorizer returns a new Depolorizer for some given bytes.
// Returns an error if the given bytes is malformed for a POLO wire.
//
// If the given data is a compound wire, the only element in the Depolorizer will be the compound data, and
// it will need to be unwrapped into another Depolorizer with DepolorizePacked() before decoding its elements
func NewDepolorizer(data []byte, options ...EncodingOptions) (*Depolorizer, error) {
	// Generate a default wire config
	config := defaultWireConfig()
	// Apply any given options to the config
	for _, opt := range options {
		opt(config)
	}

	// Create a new readbuffer from the wire
	rb, err := newreadbuffer(data)
	if err != nil {
		return nil, err
	}

	// Create a non-pack Depolorizer
	return &Depolorizer{data: rb, cfg: *config}, nil
}

// newLoadDepolorizer returns a new Depolorizer from a given readbuffer.
// The readbuffer is converted into a packbuffer and the returned Depolorizer is created in packed mode.
func newLoadDepolorizer(data readbuffer) (*Depolorizer, error) {
	// Convert the element into a packbuffer
	pack, err := data.unpack()
	if err != nil {
		return nil, err
	}

	// Create a new Depolorizer in packed mode
	return &Depolorizer{pack: pack, packed: true}, nil
}

// Done returns whether all elements in the Depolorizer have been read.
func (depolorizer *Depolorizer) Done() bool {
	// Check if packbuffer is done if in packed mode
	if depolorizer.packed {
		return depolorizer.pack.done()
	}

	// Return flag for non-pack data
	return depolorizer.done
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

	// Check that the value is a settable pointer
	if !value.Elem().CanSet() {
		return ErrObjectNotSettable
	}

	// Obtain the type of the underlying type
	target := value.Type().Elem()
	// Depolorize the next element to the target type
	result, err := depolorizer.depolorizeValue(target)

	// Handle Errors or Nils
	switch {
	case err != nil && errors.Is(err, nilValue):
		return nil
	case err != nil:
		return err
	case result == zeroVal:
		return nil
	}

	// Convert and set the decoded value
	value.Elem().Set(result.Convert(target))
	return nil
}

// DepolorizeNull attempts to decode a null value from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireNull.
func (depolorizer *Depolorizer) DepolorizeNull() error {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return err
	}

	// Error if not WireNull
	if data.wire != WireNull {
		return IncompatibleWireType(data.wire, WireNull)
	}

	return nil
}

func allowNilValue[V any](value V, err error) (V, error) {
	if errors.Is(err, nilValue) {
		return value, nil
	}

	return value, err
}

// DepolorizeBytes attempts to decode a bytes value from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireWord.
// Returns a nil byte slice if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeBytes() ([]byte, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return nil, err
	}

	if depolorizer.cfg.packBytes {
		return allowNilValue(data.decodeBytesFromPack())
	}

	return allowNilValue(data.decodeBytes())
}

// DepolorizeString attempts to decode a string value from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireWord.
// Returns an empty string if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeString() (string, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return "", err
	}

	return allowNilValue(data.decodeString())
}

// DepolorizeBool attempts to decode a bool value from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireTrue or WireFalse.
// Returns a false if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeBool() (bool, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return false, err
	}

	return allowNilValue(data.decodeBool())
}

// DepolorizeUint attempts to decode an unsigned integer value from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WirePosInt.
// Returns 0 if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeUint() (uint64, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return 0, err
	}

	return allowNilValue(data.decodeUint64())
}

// DepolorizeInt attempts to decode a signed integer value from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WirePosInt or WireNegInt.
// Returns 0 if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeInt() (int64, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return 0, err
	}

	return allowNilValue(data.decodeInt64())
}

// DepolorizeFloat32 attempts to decode a single point precision float from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireFloat.
// Returns 0 if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeFloat32() (float32, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return 0, err
	}

	return allowNilValue(data.decodeFloat32())
}

// DepolorizeFloat64 attempts to decode a double point precision float from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireFloat.
// Returns 0 if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeFloat64() (float64, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return 0, err
	}

	return allowNilValue(data.decodeFloat64())
}

// DepolorizeBigInt attempts to decode a big.Int from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WirePosInt or WireNegInt.
// Returns a nil big.Int if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeBigInt() (*big.Int, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return nil, err
	}

	return allowNilValue(data.decodeBigInt())
}

// DepolorizeDocument attempts to decode a Document from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireDoc.
// Returns nil Document if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeDocument() (Document, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return nil, err
	}

	return data.decodeDocument()
}

// DepolorizeAny attempts to decode an Any from the Depolorizer, consuming one wire element.
// If a WireNull is encountered, it is returned as Any{0}.
// Returns an error if there are no elements left. Will succeed regardless of the WireType.
func (depolorizer *Depolorizer) DepolorizeAny() (Any, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return nil, err
	}

	return data.asAny(), nil
}

// DepolorizeRaw attempts to decode a Raw from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireRaw.
func (depolorizer *Depolorizer) DepolorizeRaw() (Raw, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return nil, err
	}

	return data.asRaw()
}

// DepolorizePacked attempts to decode another Depolorizer from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WirePack or WireDoc.
// Returns a ErrNullPack if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizePacked() (*Depolorizer, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return nil, err
	}

	switch data.wire {
	case WirePack, WireDoc:
		return newLoadDepolorizer(data)

	case WireNull:
		return nil, ErrNullPack

	default:
		return nil, IncompatibleWireType(data.wire, WireNull, WirePack, WireDoc)
	}
}

// depolorizeInner attempts to decode another Depolorizer from the Depolorizer, consuming one wire element.
// Unlike DepolorizePacked which will expect a compound element and convert it into a packed Depolorizer,
// depolorizeInner will return the atomic element as an atomic Depolorizer.
func (depolorizer *Depolorizer) depolorizeInner() (*Depolorizer, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return nil, err
	}

	// Create a non-pack Depolorizer
	return &Depolorizer{data: data}, nil
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
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return zeroVal, err
	}

	switch data.wire {
	case WirePack:
		// Get the next element as a pack depolorizer with the slice elements
		pack, err := newLoadDepolorizer(data)
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
		return reflect.New(target).Elem(), nil

	default:
		return zeroVal, IncompatibleWireType(data.wire, WireNull, WirePack)
	}
}

// depolorizeArrayValue accepts a reflect.Type and decodes a value from the Depolorizer into it.
// The target type must be an array and the next wire element must be WirePack.
func (depolorizer *Depolorizer) depolorizeArrayValue(target reflect.Type) (reflect.Value, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return zeroVal, err
	}

	switch data.wire {
	case WirePack:
		// Get the next element as a pack depolorizer with the array elements
		pack, err := newLoadDepolorizer(data)
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

	// Empty Array
	case WireNull:
		return reflect.New(target).Elem(), nil

	default:
		return zeroVal, IncompatibleWireType(data.wire, WireNull, WirePack)
	}
}

// depolorizeMapValue accepts a reflect.Type and decodes a value from the Depolorizer into it.
// The target type must be a map and the next wire element must be WirePack.
func (depolorizer *Depolorizer) depolorizeMapValue(target reflect.Type) (reflect.Value, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return zeroVal, err
	}

	switch data.wire {
	case WirePack:
		// Get the next element as a pack depolorizer with the map elements
		pack, err := newLoadDepolorizer(data)
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

	case WireDoc:
		// Only allow decoding from a document if the map's key type is string
		// AND the decoder config allows for string map decoding
		if !(depolorizer.cfg.docStrMaps && target.Key().Kind() == reflect.String) {
			return zeroVal, IncompatibleWireType(data.wire, WireNull, WirePack)
		}

		// Decode the wire object into a Document
		doc, err := data.decodeDocument()
		if err != nil {
			return zeroVal, err
		}

		valType := target.Elem()
		mapping := reflect.MakeMap(target)

		// Iterate over the document elements
		for key, raw := range doc {
			// Create a new decoder for the raw value (inherit configuration)
			decoder, err := NewDepolorizer(raw, inheritCfg(depolorizer.cfg))
			if err != nil {
				return zeroVal, err
			}

			// Depolorize the raw value for the key into map's value type
			val, err := decoder.depolorizeValue(valType)
			if err != nil && !errors.Is(err, nilValue) {
				return zeroVal, err
			}

			if val != zeroVal {
				// Set the key-value pair into the map value
				mapping.SetMapIndex(reflect.ValueOf(key), val.Convert(valType))
			}
		}

		return mapping, nil

	// Zero Value Map
	case WireNull:
		return reflect.New(target).Elem(), nil

	default:
		return zeroVal, IncompatibleWireType(data.wire, WireNull, WirePack)
	}
}

// depolorizeStructValue accepts a reflect.Type and decodes a value from the Depolorizer into it.
// The target type must be a struct and the next wire element must be WirePack or WireDoc.
func (depolorizer *Depolorizer) depolorizeStructValue(target reflect.Type) (reflect.Value, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return zeroVal, err
	}

	switch data.wire {
	case WirePack:
		// Get the next element as a pack depolorizer with the struct field elements
		pack, err := newLoadDepolorizer(data)
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
		if !depolorizer.cfg.docStructs {
			return zeroVal, IncompatibleWireType(data.wire, WireNull, WirePack)
		}

		doc, err := data.decodeDocument()
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
			data := doc.GetRaw(fieldName)
			if data == nil {
				continue
			}

			object, err := NewDepolorizer(data, inheritCfg(depolorizer.cfg))
			if err != nil {
				return zeroVal, err
			}

			fieldVal, err := object.depolorizeValue(field.Type)
			if err != nil && !errors.Is(err, nilValue) {
				return zeroVal, IncompatibleWireError{fmt.Sprintf("struct field [%v.%v <%v>]: %v", target, field.Name, field.Type, err)}
			}

			if fieldVal != zeroVal {
				structure.Field(index).Set(fieldVal.Convert(field.Type))
			}
		}

		return structure, nil

	// Null Struct
	case WireNull:
		return zeroVal, nil

	default:
		return zeroVal, IncompatibleWireType(data.wire, WireNull, WirePack)
	}
}

// depolorizePointer decodes a value of type target from the Depolorizer
func (depolorizer *Depolorizer) depolorizePointer(target reflect.Type) (reflect.Value, error) {
	// recursively call depolorize with the pointer element
	value, err := depolorizer.depolorizeValue(target.Elem())

	switch {
	case err != nil && errors.Is(err, nilValue):
		return zeroVal, nil
	case err != nil:
		return zeroVal, err
	case value == zeroVal:
		return zeroVal, nil
	}

	// Create a new pointer value and set its inner value and return it
	p := reflect.New(target.Elem())
	p.Elem().Set(value.Convert(p.Elem().Type()))

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
		// Read the next element
		data, err := depolorizer.read()
		if err != nil {
			return zeroVal, err
		}

		return reflected(data.decodeBool())

	// String Value
	case reflect.String:
		// Read the next element
		data, err := depolorizer.read()
		if err != nil {
			return zeroVal, err
		}

		return reflected(data.decodeString())

	// Uint8 Value
	case reflect.Uint8:
		// Read the next element
		data, err := depolorizer.read()
		if err != nil {
			return zeroVal, err
		}

		return reflected(data.decodeUint8())

	// Int8 Value
	case reflect.Int8:
		// Read the next element
		data, err := depolorizer.read()
		if err != nil {
			return zeroVal, err
		}

		return reflected(data.decodeInt8())

	// Uint16 Value
	case reflect.Uint16:
		// Read the next element
		data, err := depolorizer.read()
		if err != nil {
			return zeroVal, err
		}

		return reflected(data.decodeUint16())

	// Int16 Value
	case reflect.Int16:
		// Read the next element
		data, err := depolorizer.read()
		if err != nil {
			return zeroVal, err
		}

		return reflected(data.decodeInt16())

	// Uint32 Value
	case reflect.Uint32:
		// Read the next element
		data, err := depolorizer.read()
		if err != nil {
			return zeroVal, err
		}

		return reflected(data.decodeUint32())

	// Int32 Value
	case reflect.Int32:
		// Read the next element
		data, err := depolorizer.read()
		if err != nil {
			return zeroVal, err
		}

		return reflected(data.decodeInt32())

	// Uint64 Value
	case reflect.Uint, reflect.Uint64:
		// Read the next element
		data, err := depolorizer.read()
		if err != nil {
			return zeroVal, err
		}

		return reflected(data.decodeUint64())

	// Int64 Value
	case reflect.Int, reflect.Int64:
		// Read the next element
		data, err := depolorizer.read()
		if err != nil {
			return zeroVal, err
		}

		return reflected(data.decodeInt64())

	// Single Point Float
	case reflect.Float32:
		// Read the next element
		data, err := depolorizer.read()
		if err != nil {
			return zeroVal, err
		}

		return reflected(data.decodeFloat32())

	// Double Point Float
	case reflect.Float64:
		// Read the next element
		data, err := depolorizer.read()
		if err != nil {
			return zeroVal, err
		}

		return reflected(data.decodeFloat64())

	// Slice Value
	case reflect.Slice:
		// Any Bytes
		if target == reflect.TypeOf(Any{}) {
			return reflected(depolorizer.DepolorizeAny())
		}

		// Raw Bytes
		if target == reflect.TypeOf(Raw{}) {
			return reflected(depolorizer.DepolorizeRaw())
		}

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
// If it is in packed mode, it reads from the packbuffer, otherwise
// it returns the readbuffer data and set the done flag.
func (depolorizer *Depolorizer) read() (readbuffer, error) {
	// Check if there is another element to read
	if depolorizer.Done() {
		return readbuffer{}, ErrInsufficientWire
	}

	// Read from the packbuffer if in packed mode
	if depolorizer.packed {
		return depolorizer.pack.next()
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
