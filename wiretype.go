package polo

// WireType is an enum for different wire types.
// A wire type indicates the kind of encoding for some data.
type WireType byte

const (
	// WireNull represents a null wire. Used for consuming field orders without data.
	WireNull WireType = 0

	// WireFalse represents a Boolean False
	WireFalse WireType = 1
	// WireTrue represents a Boolean True
	WireTrue WireType = 2

	// WirePosInt represents a Binary encoded +ve Integer in BigEndian Order.
	WirePosInt WireType = 3
	// WireNegInt represents a Binary encoded -ve Integer in BigEndian Order.
	// The number is encoded as its absolute value and must be multiplied with -1 to get its actual value.
	WireNegInt WireType = 4

	// WireBigInt represents a Binary encoded arbitrary sized integer
	// WireBigInt WireType = 5

	// WireWord represents UTF-8 encoded string/bytes
	WireWord WireType = 6
	// WireFloat represents some floating point data encoded in the IEEE754 standard. (floats)
	WireFloat WireType = 7

	// WireDoc represents some doc encoded data (string keyed maps, tagged structs and Document objects)
	WireDoc WireType = 13
	// WirePack represents some pack encoded data (slices, arrays, maps, structs)
	WirePack WireType = 14
	// WireLoad represents a load tag for compound wire type
	WireLoad WireType = 15
)

// String returns a string representation of the wiretype.
//
// If the wiretype is not recognized, "unknown" is returned.
// Implements the Stringer interface for wiretype.
func (wiretype WireType) String() string {
	switch wiretype {
	case WireNull:
		return "null"
	case WireFalse:
		return "false"
	case WireTrue:
		return "true"
	case WirePosInt:
		return "posint"
	case WireNegInt:
		return "negint"
	case WireWord:
		return "word"
	case WireFloat:
		return "float"
	case WireDoc:
		return "document"
	case WirePack:
		return "pack"
	case WireLoad:
		return "load"
	default:
		if wiretype > 15 {
			return "unknown"
		} else {
			return "reserved"
		}
	}
}

// IsNull returns whether a given wiretype is null.
// A wiretype is null if it is WireNull, has a value greater than 15 or is between 8 and 12 (reserved)
func (wiretype WireType) IsNull() bool {
	if wiretype == WireNull || !wiretype.IsValid() {
		return true
	} else {
		return false
	}
}

// IsValid returns whether a wiretype is valid.
// A wiretype is valid if its value is less than 15 and not one of the reserved types
func (wiretype WireType) IsValid() bool {
	if wiretype > 15 || (wiretype >= 8 && wiretype <= 12) {
		return false
	} else {
		return true
	}
}

// IsCompound returns whether a given wiretype is a compound type. (contains a load inside it)
func (wiretype WireType) IsCompound() bool {
	return wiretype == WirePack || wiretype == WireDoc
}
