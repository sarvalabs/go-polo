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
// If the Polorizer that the method returns has an error (see Polorizer.Err), the encoder returns that error.
type Polorizable interface {
	Polorize() (*Polorizer, error)
}

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

// Depolorizable is an interface for an object that deserialize its contents from a Depolorizer.
//
// The Depolorize method must read all the data that the Polorize method of the type writes, and no less.
// Data that the method does not read can have any content, so different wires would decode into the same
// value. The decoder returns an error if the method returns without an error and did not read:
//   - the element in the Depolorizer. The only exception is a WireNull, which has no data.
//   - all the elements of each pack that it unpacked with DepolorizePacked or Unpacked.
//
// The decoder cannot check a Depolorize method that the application calls itself. Such code must check Done.
type Depolorizable interface {
	Depolorize(*Depolorizer) error
}

// DocumentStruct is an interface for a struct type that is encoded as a document, not as a pack.
// The method has no function: it only marks the type. It can have a value or a pointer receiver.
//
//	type Transfer struct {
//		Beneficiary [32]byte `polo:"beneficiary"`
//		Amount      uint64   `polo:"amount"`
//	}
//
//	func (Transfer) PoloDocument() {}
//
// The form is a property of the type. Thus, a document struct is a document at all levels: at the top level,
// in a field of another struct, in a map value and in a slice element. The decoder accepts only a document
// for it, and rejects a pack. A struct without this method is a pack, and the decoder rejects a document.
//
// The document has one key for each exported field that is not skipped: the field name, or the name in the
// polo tag of the field. The encoder writes all the fields, also the fields with zero values. The decoder:
//   - rejects a key that is not the key of a field.
//   - gives the zero value to the field of an absent key. An encoder that has a value without
//     a field value (for example, a class with a field that is not set) can leave out the key.
//
// Two fields with the same key are an error, on encode and on decode. A custom Polorize or Depolorize
// method of the type has precedence over this interface. A struct that embeds a DocumentStruct is a
// DocumentStruct too, because Go promotes the method to it.
type DocumentStruct interface {
	PoloDocument()
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
