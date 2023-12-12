package polo

// Any is some raw POLO encoded data.
// The data of Any can have any WireType
type Any []byte

// Raw is a container for raw POLO encoded data.
// The data of Raw must have type WireRaw with its body being valid POLO data
type Raw []byte

// Polorizable is an interface for an object that serialize into a Polorizer
type Polorizable interface {
	Polorize() (*Polorizer, error)
}

// Polorize serializes an object into its POLO byte form.
// Accepts EncodingOptions to modify the encoding behaviour.
// Returns an error if object is an unsupported type such as functions or channels.
func Polorize(object any, options ...EncodingOptions) ([]byte, error) {
	// Create a new polorizer
	polorizer := NewPolorizer(options...)

	// Polorize the object
	if err := polorizer.Polorize(object); err != nil {
		return nil, err
	}

	// Return the bytes of the writebuffer
	return polorizer.wb.bytes(), nil
}

// Depolorizable is an interface for an object that deserialize its contents from a Depolorizer
type Depolorizable interface {
	Depolorize(*Depolorizer) error
}

// Depolorize deserializes a POLO encoded byte slice into an object.
// Throws an error if the wire cannot be parsed or if the object is not a pointer.
func Depolorize(object any, data []byte, options ...EncodingOptions) error {
	// Create a new depolorizer from the data
	depolorizer, err := NewDepolorizer(data, options...)
	if err != nil {
		return err
	}

	// Depolorize the object from the depolorizer
	if err = depolorizer.Depolorize(object); err != nil {
		return err
	}

	return nil
}
