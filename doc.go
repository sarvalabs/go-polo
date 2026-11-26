// Package polo is the Go implementation of POLO (Prefix Ordered Lookup Offsets), a deterministic
// encoding and object serialization scheme. See https://github.com/sarvalabs/polo for the specification.
//
// Polorize encodes a value into its POLO wire and Depolorize decodes a wire into a value, using reflection.
// Types can implement Polorizable and Depolorizable for a custom encoding, built with a Polorizer and
// read with a Depolorizer.
//
// # Null values
//
// Null (WireNull) means nil. The target type decides what a Null decodes to:
//
//   - Pointers, slices (also []byte), maps, Document, Any and Raw can be nil.
//     A Null decodes to nil. Depolorize overwrites the target, so a Null sets it to nil.
//   - For a pointer, the decoder consumes a Null and returns a nil pointer before it decodes the
//     element type. A custom Depolorize method of the element type is not called for a Null.
//   - Booleans, integers, floats, strings, arrays (also [N]byte), structs and big.Int values cannot
//     be nil. A Null is an error at every level: at the top level, in a struct field, in a slice,
//     array or map. The error is an IncompatibleWireError and matches ErrNullValue with errors.Is.
//   - A type that is not a pointer and has a custom Depolorize method receives the Null and decides
//     for itself. It can return a value or an error that matches ErrNullValue.
//   - An interface is not a target, also not for a Null, because the wire does not tell which concrete
//     type to decode. Use the concrete type, or decode into Any and decode the Any later. An interface
//     with a Polorize method encodes the value that it holds, so a nil interface encodes as a Null.
//
// The typed methods of a Depolorizer follow the same rule. DepolorizeBytes, DepolorizeBigInt,
// DepolorizeDocument, DepolorizeAny, DepolorizeRaw and DepolorizePacked return nil for a Null.
// DepolorizePacked returns a nil *Depolorizer, which has no elements: Done returns true and every
// decode returns ErrInsufficientWire. DepolorizeBool, DepolorizeString, the integer and float methods
// and DepolorizeBytes32 return an error that matches ErrNullValue. Use IsNull to check for a Null before
// a decode, and DepolorizeNull to consume it.
//
// # Strings and bytes
//
// The wire is the same for strings and bytes. A POLO string is always valid UTF-8; data that may
// hold any bytes is encoded as bytes.
//
// WireWord carries both. The wire does not say which one it holds, so the target type decides:
//
//   - Decoded into a string (string, a named string type, a string map key or a Document key),
//     the data must be valid UTF-8, as reported by utf8.Valid. There is no normalization: NUL,
//     the byte order mark and U+FFFD are ordinary characters. Otherwise the decoder returns an
//     error that matches ErrInvalidUTF8.
//   - Decoded into bytes ([]byte, a named byte slice, [N]byte, Raw or Any, or with DepolorizeBytes),
//     the data can be any bytes.
//
// Examples of WireWord data and how it decodes:
//
//	data                                      utf8.Valid  into a string  into bytes
//	empty, 61, 00, efbbbf, efbfbd, f48fbfbf   yes         accepted       accepted
//	ff, 80, c3, 61ff62                        no          error          accepted
//	c080, e080af, f0808080 (overlong)         no          error          accepted
//	eda080, edbfbf (surrogates)               no          error          accepted
//	f4908080, f5808080, f888808080            no          error          accepted
//
// The encoder applies the same rule. Polorize returns an error that matches ErrInvalidUTF8 for a string,
// a string map key or a Document key that is not valid UTF-8. Polorizer.PolorizeString and
// Polorizer.PolorizeDocument have no error result. They write the value, and the Polorizer keeps the
// error (see Polorizer.Err). Polorize returns this error for a type whose custom Polorize method wrote
// the value. Document.Bytes has no error result and does not check the keys.
//
// A field that can hold any bytes must be a []byte. Changing a field from string to []byte does not
// change the wire, so it is also the migration path for data written with an earlier version.
//
// # Canonical decoding
//
// Every value has exactly one encoding. Up to the nesting limit (see MaxDepth), the decoder accepts exactly
// the bytes that the encoder produces, so Polorize(Depolorize(wire)) == wire for every wire that decodes.
// Other wires are errors:
//
//   - Varints in tags (the top-level tag, load tags and head tags) are minimally encoded.
//   - The offset in a top-level tag is zero.
//   - Integers have no leading zero byte. Zero is a WirePosInt without data. A WireNegInt without data
//     (negative zero) is an error. The value must fit the target type: a signed type accepts a
//     magnitude up to its maximum for WirePosInt and up to its maximum + 1 (the minimum) for WireNegInt.
//     The same rules apply to big.Int, which has no size limit.
//   - WireNull, WireFalse and WireTrue have no data.
//   - A [N]byte needs a WireWord of exactly N bytes and a [N]T needs a pack of exactly N elements.
//     A float needs exactly 4 or 8 bytes. NaN is accepted and keeps its bits.
//   - In a pack, the first element starts at offset zero and a pack without elements has no data.
//     A struct pack has exactly one element for every exported field that is not skipped.
//   - Map keys are in strictly ascending order (the order of ValueSort, without duplicates) and are
//     not NaN. Like the encoder, the decoder only accepts the map key types that it can order:
//     booleans, strings, integers, floats and arrays of those. The encoder rejects a map with a NaN key.
//   - A float map key is not negative zero. Negative zero and positive zero are equal, so they are the
//     same key of a Go map. The encoder writes that key as positive zero, also in an array key, and
//     the decoder rejects a negative zero key. A negative zero that is not a map key keeps its bits.
//   - Document keys are in strictly ascending order and every Document value is a WireRaw.
//
// # Wire forms
//
// Each type has one form, and there are no options that change it. The decoder accepts only that form:
//
//   - Bytes ([]byte, [N]byte and named byte types) are a WireWord. A pack of bytes is an error.
//   - A struct is a WirePack, and a DocumentStruct is a WireDoc (see DocumentStruct). The form is a
//     property of the type, so it is the same at all levels: in a field, in a map value or in a slice.
//   - A map is a WirePack, also with string keys. A Document is a WireDoc.
//
// PolorizeDocument converts a struct or a map with string keys into a Document. For a struct that is not
// a DocumentStruct, the bytes of that Document do not decode into the struct: decode them into a Document
// and use Document.Get for each value.
//
// The canonical rule stops in these cases:
//
//   - Any and Raw hold encoded data as is. The decoder checks only the tag of an Any: the tag is canonical,
//     the wire type is not reserved and not WireLoad, and a WireNull, WireFalse or WireTrue has no data.
//     The encoder (PolorizeAny) applies the same checks, so it cannot write an Any that the decoder rejects.
//     The data after the tag is not examined, because its rules depend on the target type: for example,
//     06 ff is valid bytes but not a valid string. Thus, an Any accepts 03 05 and 03 00 05. The content of
//     a Raw is not examined at all. The canonical rules apply when the data is decoded into a typed target.
//     A Null decodes to a nil Raw, and Polorize writes a nil Raw as a Null. Polorizer.PolorizeRaw writes
//     a nil Raw as Raw{0} (a WireRaw that holds a Null), which is a different wire.
//   - Absent keys of a DocumentStruct. The decoder gives the zero value to the field of an absent key,
//     because another encoder (for example, the PISA runtime) leaves out the key of a field that has no
//     value. The encoder writes all the keys. Thus, a document without a key and the document with the
//     zero value for that key decode into the same struct. An unknown key is always an error, so a wire
//     cannot hold data that the struct does not read.
//   - Custom codecs. The author of a Polorize and Depolorize method must make sure that the type has one
//     wire. The decoder checks one part of this: it returns an error if a custom Depolorize method does not
//     read all the data of its element (see Depolorizable).
//
// The encoder has no nesting limit. It can write a wire with more levels than the decoder accepts
// (MaxDepth), and the decoder rejects that wire with ErrMaxDepthExceeded.
package polo
