package polo

import "reflect"

// Polorize serializes an object into its POLO byte form.
// Returns an error if object is an unsupported type such as functions or channels.
func Polorize(object any) ([]byte, error) {
	var wb writebuffer
	// Serialize the object into a writebuffer
	if err := polorize(reflect.ValueOf(object), &wb); err != nil {
		return nil, err
	}

	// Return the bytes of the writebuffer
	return wb.bytes(), nil
}

// Depolorize deserializes a POLO encoded byte slice into an object.
// Throws an error if the wire cannot be parsed or if the object is not a pointer.
func Depolorize(object any, b []byte) error {
	// Reflect the object and check if it is a pointer
	v := reflect.ValueOf(object)
	if v.Kind() != reflect.Ptr {
		return ErrObjectNotPtr
	}

	// Create a new readbuffer from the given byte slice
	rb, err := newreadbuffer(b)
	if err != nil {
		return DecodeError{err.Error()}
	}

	t := v.Type().Elem()

	// Depolorize the readbuffer into the object
	result, err := depolorize(t, rb)

	// If there was a parse error or the result is a nil, return
	if err != nil {
		return DecodeError{err.Error()}
	} else if result == nil {
		return nil
	}

	// Set the value of the given object
	v.Elem().Set(reflect.ValueOf(result).Convert(t))

	return nil
}
