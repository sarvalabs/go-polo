package polo

import (
	"reflect"
)

// Packer is pack encoder that can continuously encode objects into a POLO wire.
// Field order is determined by order of insertion.
type Packer struct {
	wb *writebuffer
}

// NewPacker initializes and returns a new Packer object
func NewPacker() *Packer {
	return &Packer{wb: &writebuffer{}}
}

// Pack packs and encodes some object into Packer.
// Returns an error if object is of an unsupported type.
func (pack *Packer) Pack(object any) error {
	err := polorize(reflect.ValueOf(object), pack.wb)
	if err != nil {
		return PackError{EncodeError{err.Error()}.Error()}
	}

	return nil
}

// PackWire packs some encoded data into Packer.
// Checks if the wire begins with a valid WireType (no data validation).
// Returns an error if the wire is empty or if it begins with an invalid WireType.
func (pack *Packer) PackWire(wire []byte) error {
	// Check if wire is empty
	if len(wire) == 0 {
		return PackError{"wire is empty"}
	}

	// Separate wiretype and wiredata. Validate wiretype
	wiretype, wiredata := WireType(wire[0]), wire[1:]
	if !wiretype.IsValid() {
		return PackError{"invalid wiretype"}
	}

	// Write wire elements into writebuffer
	pack.wb.write(wiretype, wiredata)

	return nil
}

// Bytes returns the pack encoded data in Packer tagged with WirePack
func (pack *Packer) Bytes() []byte {
	return prepend(byte(WirePack), pack.wb.load())
}
