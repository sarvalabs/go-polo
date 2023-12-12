package polo

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	// zeroVal represents the zero value of reflect.Value.
	// It acts as a marker for encoding/decoding nil values.
	zeroVal = reflect.ValueOf(nil)
	// nilValue is an error for when a WireNull is encountered during reflective decoding.
	// It acts a signal for error and value handlers.
	errNilValue = errors.New("nil value")

	// ErrNullPack is an error for when a WireNull is attempted to be converted to a Depolorizer
	ErrNullPack = errors.New("null pack element")
	// ErrObjectNotPtr is an error for when a non pointer object is passed to the Depolorize function
	ErrObjectNotPtr = errors.New("object not a pointer")
	// ErrObjectNotSettable is an error for when a non-settable pointer is passed to the Depolorize function
	ErrObjectNotSettable = errors.New("object is not settable")
	// ErrInsufficientWire is an error for when the data in depolorizer is exhausted
	ErrInsufficientWire = errors.New("insufficient data in wire for decode")
)

// MalformedTagError is an error for when a consumed varint for a tag is malformed
type MalformedTagError struct {
	msg string
}

// Error implements the error interface for MalformedTagError
func (err MalformedTagError) Error() string {
	return fmt.Sprintf("malformed tag: %v", err.msg)
}

// IncompatibleWireError is an error for when an object cannot be decoded from some wire data
type IncompatibleWireError struct {
	msg string
}

// Error implements the error interface for IncompatibleWireError
func (err IncompatibleWireError) Error() string {
	return fmt.Sprintf("incompatible wire: %v", err.msg)
}

// IncompatibleWireType returns an IncompatibleWireError formatted to express
// the mismatch between an unexpected wire type and the list of expected ones.
func IncompatibleWireType(actual WireType, expected ...WireType) IncompatibleWireError {
	expects := make([]string, 0, len(expected))
	for _, wire := range expected {
		expects = append(expects, wire.String())
	}

	data := "{" + strings.Join(expects, `, `) + `}`

	return IncompatibleWireError{fmt.Sprintf("unexpected wiretype '%v'. expected one of: %v", actual, data)}
}

// IncompatibleValueError is an error for when an incompatible value is used for encoding
type IncompatibleValueError struct {
	msg string
}

// Error implements the error interface for IncompatibleValueError
func (err IncompatibleValueError) Error() string {
	return fmt.Sprintf("incompatible value error: %v", err.msg)
}

func UnsupportedTypeError(t reflect.Type) IncompatibleValueError {
	return IncompatibleValueError{fmt.Sprintf("unsupported type: %v [%v]", t, t.Kind())}
}
