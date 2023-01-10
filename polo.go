package polo

// Polorize serializes an object into its POLO byte form.
// Returns an error if object is an unsupported type such as functions or channels.
func Polorize(object any) ([]byte, error) {
	// Create a new polorizer
	polorizer := NewPolorizer()

	// Polorize the object
	if err := polorizer.Polorize(object); err != nil {
		return nil, err
	}

	// Return the bytes of the writebuffer
	return polorizer.wb.bytes(), nil
}

// Depolorize deserializes a POLO encoded byte slice into an object.
// Throws an error if the wire cannot be parsed or if the object is not a pointer.
func Depolorize(object any, data []byte) error {
	// Create a new depolorizer from the data
	depolorizer, err := NewDepolorizer(data)
	if err != nil {
		return err
	}

	// Depolorize the object from the depolorizer
	if err = depolorizer.Depolorize(object); err != nil {
		return err
	}

	return nil
}
