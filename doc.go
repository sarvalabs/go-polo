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
//   - Pointers, slices (also []byte), maps, interfaces, Document, Any and Raw can be nil.
//     A Null decodes to nil. Depolorize overwrites the target, so a Null sets it to nil.
//   - For a pointer, the decoder consumes a Null and returns a nil pointer before it decodes the
//     element type. A custom Depolorize method of the element type is not called for a Null.
//   - Booleans, integers, floats, strings, arrays (also [N]byte), structs and big.Int values cannot
//     be nil. A Null is an error at every level: at the top level, in a struct field, in a slice,
//     array or map. The error is an IncompatibleWireError and matches ErrNullValue with errors.Is.
//   - A type that is not a pointer and has a custom Depolorize method receives the Null and decides
//     for itself. It can return a value or an error that matches ErrNullValue.
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
// Polorizer.PolorizeDocument do not return errors and do not check: the caller must pass valid UTF-8,
// or the result does not decode as a string.
//
// A field that can hold any bytes must be a []byte. Changing a field from string to []byte does not
// change the wire, so it is also the migration path for data written with an earlier version.
//
// # Canonical decoding
//
// Every value has exactly one encoding. The decoder accepts exactly the bytes that the encoder produces,
// so Polorize(Depolorize(wire)) == wire for every wire that decodes with the default options.
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
//   - Document keys are in strictly ascending order and every Document value is a WireRaw.
//   - Bytes in a pack (the PackedBytes option) are WirePosInt elements with exactly one byte each,
//     zero included, because that is how the encoder writes them.
//
// The options DocStructs, DocStringMaps and PackedBytes accept two forms by design: a struct or a string
// map as a pack or as a document, and bytes as a word or as a pack. Documents are the extensible format:
// when a struct is decoded from a document (DocStructs), the decoder ignores unknown keys and leaves the
// fields of absent keys at their zero values. Any and Raw
// hold encoded data as is: the decoder only checks the tag of an Any, not its content. A nil Raw is
// written as Raw{0} (a WireRaw that holds a Null), so a Null decodes to a nil Raw that encodes as Raw{0}.
package polo
