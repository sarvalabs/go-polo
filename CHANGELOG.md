# Changelog

## v0.5.0 (unreleased)

The decoder is now canonical: it accepts exactly the bytes that the encoder produces, so every value
has one encoding. Null and UTF-8 handling follow one rule each. There are no encoding options: each type
has one wire form, and a struct type selects the document form with the `DocumentStruct` interface.
The encoder writes the same bytes as v0.4.10 for every value it encodes without options, with the
exceptions listed under "Behaviour changes for data go-polo wrote".

### Fixed

- GO-1: `math.MinInt64` decodes (it used to fail with "overflow for signed integer").
- GO-5: a Null element in a slice or map is an error. It used to return no error and leave the target untouched.
- GO-6: `int8`, `int16` and `int32` reject out-of-range values instead of wrapping (`03ff` used to decode to `int8(-1)`).
- GO-7: non-canonical wires are rejected (see "Canonical decoding" in the README).
- GO-10: NaN decodes, with its bits preserved.
- GO-12: a string must be valid UTF-8, on decode and on encode.
- go-pisa #130: a Null decodes to a nil pointer before a custom `Depolorize` method is called.
- `IsNull` returns false when no elements are left, as documented (it used to report the last element of an atomic `Depolorizer`).
- `Depolorizer.IsNull` returns false for the reserved wire types 8 to 12, as documented. It used to return true, so a custom `Depolorize` method that checks `IsNull` could accept a reserved wire type as a Null. `WireType.IsNull` does not change.
- A nil interface whose type has a `Polorize` method, or a nil pointer in such an interface, encodes as a Null.
  It used to panic. The wire is the same as for a field of the concrete type. Interfaces are still not a
  target for the decoder.

### Added

- `ErrNullValue`: matched by the error for a Null decoded into a value that cannot be nil.
- `ErrInvalidUTF8`: matched by the error for a string that is not valid UTF-8.
- `IncompatibleWireError` and `IncompatibleValueError` unwrap to these sentinels.
- A nil `*Depolorizer` is safe to use: it has no elements.
- `DocumentStruct`: a struct type with the method `PoloDocument()` is encoded as a document at all levels,
  and the decoder accepts only a document for it. It replaces the `DocStructs` option.
- `MaxDepth`: the nesting limit of the decoder, a constant (1,000 levels).
- `Polorizer.Err`: the first error of `PolorizeString`, `PolorizeDocument` and `PolorizePacked`, which have no error result.

### Removed

There are no encoding options. Each type has one wire form, so the encoder and the decoder cannot use
different forms, and a custom codec cannot get other forms than the reflective code around it.

- `EncodingOptions`, `PackedBytes`, `DocStructs`, `DocStringMaps` and the `MaxDepth` option.
  `DefaultMaxDepth` is replaced by the constant `MaxDepth`.
- The `options` parameter of `Polorize`, `Depolorize`, `NewPolorizer`, `NewDepolorizer`, `PolorizeDocument`,
  `Document.Get` and `Document.Set`. A call without options compiles unchanged. A call with an option does
  not compile:

| v0.4.10 | v0.5.0 |
|---|---|
| `Polorize(x, DocStructs())`, `Depolorize(&x, data, DocStructs())` | add `func (T) PoloDocument() {}` to the struct type `T`, and to each struct type in it that is a document, then `Polorize(x)` and `Depolorize(&x, data)` |
| `PolorizeDocument(x, DocStructs())` for a struct without struct fields | `PolorizeDocument(x)` (the same bytes) |
| `DocStringMaps()` | a map is a pack. For a document, use a `Document` or a `DocumentStruct` |
| `PackedBytes()` | bytes are a word |
| `MaxDepth(n)` | the limit is fixed at `MaxDepth` (1,000) |

### Behaviour changes for wires go-polo never produced

These wires used to decode and are now errors:

- non-minimal varints in tags, and a top-level tag with an offset
- integers with a leading zero byte, negative zero, and out-of-range values for `int8`, `int16` and `int32`
- data in a Null, False or True element
- a `[N]byte` from a Word that is not exactly N bytes (an empty Word used to give the zero array)
- a `[N]T` from a pack with more than N elements, and a struct pack with extra elements
- a pack whose first element doesn't start at offset 0, or a pack without elements that has data
- unsorted or duplicate map keys and document keys, NaN map keys, and map key types that the encoder can't order
- a `[]byte` or `[N]byte` from a pack
- a Null into a bool, integer, float, string, array, struct or `big.Int` value, at any level
  (this used to give the zero value or leave the target untouched)
- a document for a struct with a key that is not the key of a field (`DocStructs` used to ignore it, with any
  data as its value). An absent key still gives the zero value
- a document for a struct that is not a `DocumentStruct`, a pack for a `DocumentStruct`, and a document for a map
- an element with a reserved wire type (8 to 12) or a Load (15) into an `Any`

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
- **A document struct field with a Null value** is an error unless the field can be nil. It used to be skipped.
- **A float32 signaling NaN** keeps its bits on encode. It used to be written as a quiet NaN.
- **An `Any` that the decoder rejects** is an error on encode: a tag that is not minimal, a tag with an offset,
  a reserved wire type or a Load, or a Null, False or True with data. v0.4.10 rewrote the tag (`83 00 05`
  became `03 05`) and wrote `00 ff`, which decoded as `00` without the `ff`. An `Any` from `DepolorizeAny`
  is always accepted.
- **Data written with an option** decodes only into types with the same forms. Data written with
  `DocStructs` decodes into `DocumentStruct` types (also the nested structs must be `DocumentStruct` types).
  Data written with `PackedBytes` (bytes as a pack) or `DocStringMaps` (string maps as documents) no longer
  decodes into bytes or maps. Read such a pack with `DepolorizePacked` and `DepolorizeAny`, and such a
  document as a `Document`. A custom `Depolorize` method no longer gets options from the caller (v0.4.10
  added this as GO-8), because there are none.
- **A custom `Depolorize` method must read all the data of its element.** The decoder returns an error if the
  method returns without an error and leaves the element unread, or does not read all the elements of a pack
  that it unpacked with `DepolorizePacked`. A method can leave a Null unread. Data that a method skipped
  used to be accepted with any content.
- **A custom `Polorize` method that writes a string or a `Document` key that is not valid UTF-8** makes
  `Polorize` return an error. `PolorizeString` and `PolorizeDocument` still write the value, and the
  `Polorizer` keeps the error (`Polorizer.Err`). The bytes of the `Polorizer` are the same as before.
- **A float map key that is `-0`** is written as `+0`, also in an array key, because the two zeros are the
  same key of a Go map. A map with a `-0` key that an earlier version wrote no longer decodes.
  A `-0` that is not a map key keeps its bits.
- **A wire with more than 1,000 nesting levels** is an error (`ErrMaxDepthExceeded`). The limit is the constant
  `MaxDepth`. It was `DefaultMaxDepth` (10,000) and the `MaxDepth` option could change it. 10,000 levels
  overflow a 2 MiB thread stack in rs-polo, and both libraries keep the same limit.

## v0.4.10

- Fixed decoder crashes on malformed wires: unchecked load lengths (GO-2), element offsets (GO-3)
  and Null map keys (GO-4).
- Custom `Depolorize` methods receive the options of the caller (GO-8).
- `Polorize` returns an error for map keys that can't be ordered (GO-9).
- Added a nesting limit for decoding: `MaxDepth`, `DefaultMaxDepth`, `ErrMaxDepthExceeded`.
- The input of `NewDepolorizer` is copied once.
- Requires Go 1.26. golangci-lint v2.
