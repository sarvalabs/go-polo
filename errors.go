package polo

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	// zeroVal represents the zero value of reflect.Value.
	// It is returned together with errors by the reflective decoding functions.
	zeroVal = reflect.ValueOf(nil)

	// ErrObjectNotPtr is an error for when a non pointer object is passed to the Depolorize function
	ErrObjectNotPtr = errors.New("object not a pointer")
	// ErrObjectNotSettable is an error for when a non-settable pointer is passed to the Depolorize function
	ErrObjectNotSettable = errors.New("object is not settable")
	// ErrInsufficientWire is an error for when the data in depolorizer is exhausted
	ErrInsufficientWire = errors.New("insufficient data in wire for decode")
	// ErrMaxDepthExceeded is an error for when a wire is nested deeper than the decoder allows (see MaxDepth)
	ErrMaxDepthExceeded = errors.New("maximum nesting depth exceeded")

	// ErrNullValue is matched (with errors.Is) by the error for a WireNull that is decoded into a value
	// that cannot be nil: booleans, integers, floats, strings, arrays and structs (big.Int included).
	// A WireNull decodes to nil for pointers, slices, maps, interfaces, Document, Any and Raw.
	// The error is an IncompatibleWireError and is the same at every nesting level.
	ErrNullValue = errors.New("nil value")

	// ErrInvalidUTF8 is matched (with errors.Is) by the error for a string that is not valid UTF-8 (see utf8.Valid).
	// A POLO string is always valid UTF-8. The decoder rejects a WireWord that is not valid UTF-8 for
	// a string target, and the reflective encoder (Polorize) rejects a string that is not valid UTF-8.
	// Data that can hold any bytes must be encoded and decoded as bytes ([]byte). The wire is the same.
	ErrInvalidUTF8 = errors.New("invalid UTF-8 string")
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
	// err is an optional sentinel error (ErrNullValue, ErrInvalidUTF8) that the error matches with errors.Is
	err error
}

// Error implements the error interface for IncompatibleWireError
func (err IncompatibleWireError) Error() string {
	return fmt.Sprintf("incompatible wire: %v", err.msg)
}

// Unwrap returns the sentinel error that the IncompatibleWireError wraps (nil if there is none)
func (err IncompatibleWireError) Unwrap() error {
	return err.err
}

// IncompatibleWireType returns an IncompatibleWireError formatted to express
// the mismatch between an unexpected wire type and the list of expected ones.
func IncompatibleWireType(actual WireType, expected ...WireType) IncompatibleWireError {
	expects := make([]string, 0, len(expected))
	for _, wire := range expected {
		expects = append(expects, wire.String())
	}

	data := "{" + strings.Join(expects, `, `) + `}`

	return IncompatibleWireError{msg: fmt.Sprintf("unexpected wiretype '%v'. expected one of: %v", actual, data)}
}

// nullValueError returns the error for a WireNull that is decoded into a value that cannot be nil
func nullValueError() IncompatibleWireError {
	return IncompatibleWireError{msg: ErrNullValue.Error(), err: ErrNullValue}
}

// invalidUTF8WireError returns the error for a WireWord that is not valid UTF-8 and is decoded into a string
func invalidUTF8WireError() IncompatibleWireError {
	return IncompatibleWireError{msg: ErrInvalidUTF8.Error(), err: ErrInvalidUTF8}
}

// invalidUTF8ValueError returns the error for a string that is not valid UTF-8 and is encoded
func invalidUTF8ValueError() IncompatibleValueError {
	return IncompatibleValueError{msg: ErrInvalidUTF8.Error(), err: ErrInvalidUTF8}
}

// structFieldError is an IncompatibleWireError for a struct field that could not be decoded.
// The field description and the field's error are kept apart and only rendered on demand, so
// that a failure deep inside nested structs costs linear (not quadratic) time in the nesting depth.
// It is retrievable as an IncompatibleWireError with errors.As, and errors.Is sees through it to its cause.
type structFieldError struct {
	field string // the field description as "<struct type>.<field name> <field type>"
	cause error
}

// newStructFieldError returns a structFieldError for the given field of the target struct type
func newStructFieldError(target reflect.Type, field reflect.StructField, cause error) structFieldError {
	return structFieldError{fmt.Sprintf("%v.%v <%v>", target, field.Name, field.Type), cause}
}

// Error implements the error interface for structFieldError.
// A chain of nested struct field errors is rendered iteratively.
func (err structFieldError) Error() string {
	var sb strings.Builder

	for {
		sb.WriteString("incompatible wire: struct field [")
		sb.WriteString(err.field)
		sb.WriteString("]: ")

		next, ok := err.cause.(structFieldError) //nolint:errorlint // only directly nested field errors chain
		if !ok {
			sb.WriteString(err.cause.Error())

			return sb.String()
		}

		err = next
	}
}

// Unwrap returns the error of the field, so that errors.Is and errors.As see through struct field errors
func (err structFieldError) Unwrap() error {
	return err.cause
}

// As allows a structFieldError to be retrieved as an IncompatibleWireError with errors.As.
// The retrieved error has the full message and wraps the error of the field.
// Any other target is matched against the error of the field (see Unwrap).
func (err structFieldError) As(target any) bool {
	if wireErr, ok := target.(*IncompatibleWireError); ok {
		*wireErr = IncompatibleWireError{msg: strings.TrimPrefix(err.Error(), "incompatible wire: "), err: err.cause}

		return true
	}

	return false
}

// IncompatibleValueError is an error for when an incompatible value is used for encoding
type IncompatibleValueError struct {
	msg string
	// err is an optional sentinel error (ErrInvalidUTF8) that the error matches with errors.Is
	err error
}

// Error implements the error interface for IncompatibleValueError
func (err IncompatibleValueError) Error() string {
	return fmt.Sprintf("incompatible value error: %v", err.msg)
}

// Unwrap returns the sentinel error that the IncompatibleValueError wraps (nil if there is none)
func (err IncompatibleValueError) Unwrap() error {
	return err.err
}

func UnsupportedTypeError(t reflect.Type) IncompatibleValueError {
	return IncompatibleValueError{msg: fmt.Sprintf("unsupported type: %v [%v]", t, t.Kind())}
}
