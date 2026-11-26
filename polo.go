package polo

// Any is some raw POLO encoded data: one complete element that is not decoded.
// The data of Any can have each WireType that a target type can decode, thus not a reserved wire type
// and not WireLoad. The decoder and the encoder check the tag of an Any, but not the data after it.
// The canonical rules apply to that data when the Any is decoded into a typed target.
type Any []byte

// Raw is a container for raw POLO encoded data.
// The data of Raw must have type WireRaw with its body being valid POLO data
type Raw []byte

// Polorizable is an interface for an object that serialize into a Polorizer.
//
// The Polorize method creates its own Polorizer, so it does not receive the EncodingOptions of the caller.
// If the Polorizer that the method returns has an error (see Polorizer.Err), the encoder returns that error.
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

// Depolorizable is an interface for an object that deserialize its contents from a Depolorizer.
//
// The Depolorize method must read all the data that the Polorize method of the type writes, and no less.
// Data that the method does not read can have any content, so different wires would decode into the same
// value. The decoder returns an error if the method returns without an error and did not read:
//   - the element in the Depolorizer. The only exception is a WireNull, which has no data.
//   - all the elements of each pack that it unpacked with DepolorizePacked or Unpacked.
//
// The decoder cannot check a Depolorize method that the application calls itself. Such code must check Done.
//
// The Depolorizer has the default wire forms: it does not have the PackedBytes, DocStructs and DocStringMaps
// options of the caller, because the Polorize method does not receive them. It keeps the nesting limit (MaxDepth).
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
