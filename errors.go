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

	// ErrObjectNotPtr is an error for when a non pointer object is passed to the Depolorize function
	ErrObjectNotPtr = errors.New("object not a pointer")
	// ErrObjectNotSettable is an error for when a non-settable pointer is passed to the Depolorize function
	ErrObjectNotSettable = errors.New("object is not settable")
	// ErrInsufficientWire is an error for when the data in depolorizer is exhausted
	ErrInsufficientWire = errors.New("insufficient data in wire for decode")
	// ErrMaxDepthExceeded is an error for when a wire is nested deeper than the decoder allows (see MaxDepth)
	ErrMaxDepthExceeded = errors.New("maximum nesting depth exceeded")
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

// structFieldError is an IncompatibleWireError for a struct field that could not be decoded.
// The field description and the field's error are kept apart and only rendered on demand, so
// that a failure deep inside nested structs costs linear (not quadratic) time in the nesting depth.
// It is retrievable as an IncompatibleWireError with errors.As.
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

// Is reports whether the error of the field matches the target, so that errors.Is sees through
// struct field errors. The internal nil value marker is excluded: a null that was reported as a
// field error must not be mistaken for a null of the whole struct by the callers that check it.
func (err structFieldError) Is(target error) bool {
	return !errors.Is(target, errNilValue) && errors.Is(err.cause, target)
}

// As allows a structFieldError to be retrieved as an IncompatibleWireError with errors.As.
// Any other target is matched against the error of the field.
func (err structFieldError) As(target any) bool {
	if wireErr, ok := target.(*IncompatibleWireError); ok {
		*wireErr = IncompatibleWireError{strings.TrimPrefix(err.Error(), "incompatible wire: ")}

		return true
	}

	return errors.As(err.cause, target)
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
