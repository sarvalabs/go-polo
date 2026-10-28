# Changelog

## v0.4.11 (unreleased)

The decoder is now canonical: it accepts exactly the bytes that the encoder produces, so every value
has one encoding. Null and UTF-8 handling follow one rule each. The encoder writes the same bytes as
v0.4.10 for every value it encodes, with the exceptions listed under "Behaviour changes for data go-polo wrote".

### Fixed

- GO-1: `math.MinInt64` decodes (it used to fail with "overflow for signed integer").
- GO-5: a Null element in a slice or map is an error. It used to return no error and leave the target untouched.
- GO-6: `int8`, `int16` and `int32` reject out-of-range values instead of wrapping (`03ff` used to decode to `int8(-1)`).
- GO-7: non-canonical wires are rejected (see "Canonical decoding" in the README).
- GO-10: NaN decodes, with its bits preserved.
- GO-12: a string must be valid UTF-8, on decode and on encode.
- go-pisa #130: a Null decodes to a nil pointer before a custom `Depolorize` method is called.
- `IsNull` returns false when no elements are left, as documented (it used to report the last element of an atomic `Depolorizer`).

### Added

- `ErrNullValue`: matched by the error for a Null decoded into a value that cannot be nil.
- `ErrInvalidUTF8`: matched by the error for a string that is not valid UTF-8.
- `IncompatibleWireError` and `IncompatibleValueError` unwrap to these sentinels.
- A nil `*Depolorizer` is safe to use: it has no elements.

### Behaviour changes for wires go-polo never produced

These wires used to decode and are now errors:

- non-minimal varints in tags, and a top-level tag with an offset
- integers with a leading zero byte, negative zero, and out-of-range values for `int8`, `int16` and `int32`
- data in a Null, False or True element
- a `[N]byte` from a Word that is not exactly N bytes (an empty Word used to give the zero array)
- a `[N]T` from a pack with more than N elements, and a struct pack with extra elements
- a pack whose first element doesn't start at offset 0, or a pack without elements that has data
- unsorted or duplicate map keys and document keys, NaN map keys, and map key types that the encoder can't order
- a `[]byte` from a pack without the `PackedBytes` option
- a Null into a bool, integer, float, string, array, struct or `big.Int` value, at any level
  (this used to give the zero value or leave the target untouched)

Null now decodes to nil for `Any` (it used to give `Any{0}`) and for `Raw`, `DepolorizeRaw` and
`DepolorizePacked` (they used to return an error).

### Behaviour changes for data go-polo wrote

- **Strings that aren't valid UTF-8** used to encode and decode. Now `Polorize` returns an error for them,
  and data written with an earlier version doesn't decode into a string. Decode such data into `[]byte`
  instead: the wire is the same.
- **A Null into a value that cannot be nil** is an error. go-polo doesn't write a Null for such a value,
  but hand-written code can (`PolorizeNull`, an empty custom `Polorize`, or a placeholder `[]byte{0}`).
- **A pointer to a type with a custom codec** decodes a Null as nil. It used to call the custom `Depolorize`,
  which often returned a non-nil zero value. `Polorizer.Bytes()` returns a Null for a Polorizer without
  elements, so a custom `Polorize` that writes nothing for an empty value now decodes as nil.
- **A DocStructs field with a Null value** is an error unless the field can be nil. It used to be skipped.
- **A map value from a document (DocStringMaps) that is Null** is kept as a nil entry. It used to be dropped.
- **A float32 signaling NaN** keeps its bits on encode. It used to be written as a quiet NaN.

## v0.4.10

- Fixed decoder crashes on malformed wires: unchecked load lengths (GO-2), element offsets (GO-3)
  and Null map keys (GO-4).
- Custom `Depolorize` methods receive the options of the caller (GO-8).
- `Polorize` returns an error for map keys that can't be ordered (GO-9).
- Added a nesting limit for decoding: `MaxDepth`, `DefaultMaxDepth`, `ErrMaxDepthExceeded`.
- The input of `NewDepolorizer` is copied once.
- Requires Go 1.26. golangci-lint v2.
