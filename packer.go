package polo

//// Packer is pack encoder that can continuously encode objects into a POLO wire.
//// Field order is determined by order of insertion.
//type Packer struct {
//	wb *writebuffer
//}
//
//// NewPacker initializes and returns a new Packer object
//func NewPacker() *Packer {
//	return &Packer{wb: &writebuffer{}}
//}
//
//// Pack packs and encodes some object into Packer.
//// Returns an error if object is of an unsupported type.
//func (pack *Packer) Pack(object any) error {
//	err := polorize(reflect.ValueOf(object), pack.wb)
//	if err != nil {
//		return PackError{EncodeError{err.Error()}.Error()}
//	}
//
//	return nil
//}
//
//// PackWire packs some encoded data into Packer.
//// Checks if the wire begins with a valid WireType (no data validation).
//// Returns an error if the wire is empty or if it begins with an invalid WireType.
//func (pack *Packer) PackWire(wire []byte) error {
//	// Check if wire is empty
//	if len(wire) == 0 {
//		return PackError{"wire is empty"}
//	}
//
//	// Separate wiretype and wiredata. Validate wiretype
//	wiretype, wiredata := WireType(wire[0]), wire[1:]
//	if !wiretype.IsValid() {
//		return PackError{"invalid wiretype"}
//	}
//
//	// Write wire elements into writebuffer
//	pack.wb.write(wiretype, wiredata)
//
//	return nil
//}
//
//// Bytes returns the pack encoded data in Packer tagged with WirePack
//func (pack *Packer) Bytes() []byte {
//	return prepend(byte(WirePack), pack.wb.load())
//}

// Unpacker is pack decoder that can continuously decode objects from a POLO wire.
// Elements are retrieved in the encoded field order.
type Unpacker struct {
	load *loadreader
}

// NewUnpacker initializes and returns a new Unpacker object for the given wire.
// Returns an error if the wire has a malformed tag or is not a compound.
func NewUnpacker(wire []byte) (*Unpacker, error) {
	// Create a new readbuffer from the wire
	rb, err := newreadbuffer(wire)
	if err != nil {
		return nil, UnpackError{err.Error()}
	}

	// Convert readbuffer into a loadreader
	load, err := rb.load()
	if err != nil {
		return nil, UnpackError{err.Error()}
	}

	return &Unpacker{load: load}, nil
}

// Unpack unpacks and decodes an element into the given object.
// Returns an error if there are elements left to unpack, if the object is
// not a pointer or if the element cannot be decoded into the given object.
func (unpack *Unpacker) Unpack(object any) error {
	// Unpack the wire element from Unpacker
	element, err := unpack.UnpackWire()
	if err != nil {
		return err
	}

	// Decode the element into the object
	if err = Depolorize(object, element); err != nil {
		return UnpackError{err.Error()}
	}

	return nil
}

// UnpackWire unpacks some encoded data from the Unpacker.
// Returns an error if there are elements left to unpack or if the buffer is malformed
func (unpack *Unpacker) UnpackWire() ([]byte, error) {
	// Check if there are elements left in the buffer
	if unpack.Done() {
		return nil, UnpackError{"no elements left"}
	}

	// Load the next element from the loadreader
	buffer, err := unpack.load.next()
	if err != nil {
		return nil, UnpackError{err.Error()}
	}

	return buffer.bytes(), nil
}

// Done returns whether all elements have been unpacked.
func (unpack Unpacker) Done() bool {
	return unpack.load.done()
}
