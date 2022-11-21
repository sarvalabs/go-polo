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
