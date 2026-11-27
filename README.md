![image](./banner.png)

[godoclink]: https://godoc.org/github.com/sarvalabs/go-polo
[latestrelease]: https://github.com/sarvalabs/go-polo/releases/latest
[issueslink]: https://github.com/sarvalabs/go-polo/issues
[pullslink]: https://github.com/sarvalabs/go-polo/pulls

![go version](https://img.shields.io/github/go-mod/go-version/sarvalabs/go-polo?style=for-the-badge)
![license](https://img.shields.io/badge/license-MIT%2FApache--2.0-informational?style=for-the-badge)
[![go docs](http://img.shields.io/badge/go-documentation-blue.svg?style=for-the-badge)][godoclink]
[![latest tag](https://img.shields.io/github/v/tag/sarvalabs/go-polo?color=blue&label=latest%20tag&sort=semver&style=for-the-badge)][latestrelease]

[![issue count](https://img.shields.io/github/issues/sarvalabs/go-polo?style=for-the-badge&color=yellow)][issueslink]
[![pulls count](https://img.shields.io/github/issues-pr/sarvalabs/go-polo?style=for-the-badge&color=brightgreen)][pullslink]
![test status](https://img.shields.io/github/actions/workflow/status/sarvalabs/go-polo/ci.yaml?label=tests&style=for-the-badge)


# go-polo
**go-polo** is a Go implementation of the POLO Encoding and Object Serialization Scheme. [**POLO**](https://github.com/sarvalabs/polo) stands for ***Prefix Ordered Lookup Offsets***. 

It is intended for use in projects that prioritize deterministic serialization, minimal wire sizes and serialization safety. POLO follows a very strict specification that is optimized for partial decoding and differential messaging. This implementation is compliant with the [POLO Specification](https://github.com/sarvalabs/polo) that describes both the encoding (wire) format as well as implementation guidelines for several languages.

## Installation
Install the latest [release](https://github.com/sarvalabs/go-polo/releases) using the following command
```sh
go get -u github.com/sarvalabs/go-polo
```

## Features
### Deterministic Serialization
POLO's strict specification is intended to create the same serialized wire for an object regardless of implementation. This is critical for cryptographic security with operations such as hashing which is used to guarantee data consistency and tamper proofing.

Since v0.5.0 the decoder is also strict: it accepts exactly the bytes that the encoder produces, so every value has one encoding (see [Canonical Decoding](#canonical-decoding)).

### High Wire Efficiency
POLO has a highly optimized wire format allows messages to be relatively small, even surpassing [Protocol Buffers](https://protobuf.dev/programming-guides/encoding/) occassionaly. This is mainly because it supports a larger type based wire tagging that allows some information (especially metadata) to be passed around inferentially and thus reducing the total amount of information actually present in the wire. 

This is augmented by the fact that POLO supports **Atomic Encoding**, because of which simple objects such as integers & strings can be encoded without needing to be wrapped within a larger (structural) message

### Partial Encoding/Decoding Constructs
The POLO wire format prefixes all metadata for the wire in the front, this metadata includes the wire type information as well as the offset position of the data. This kind of *lookup* based offset tags allows us to directly access the data for a particular field order. This capability is currently supported using the `Document` and `Raw` constructs. 

The `Document` construct is used to create a string indexed collection of `Raw` object and it collapses into a special form of the POLO wire called the **document-encoded wire** with the `WireDoc` wire type. It's unique in that it preserves the field name (string index) for the data unlike regular POLO encoding and is similar to encoding schemes like JSON or YAML and consequently consumes more wire space.

The `Raw` construct is useful for capturing the wire data for a specific field in the struct. It can also be used to define a structure that 'skips' the unrequired fields by capturing their raw wire instead of decoding them.

Partial encodign/decoding capabilities for this implementation are unfinished can be extended to support field order based access/write for regular POLO encoded wires in the future.

### Custom Encoding/Decoding Buffers
POLO describes two buffers, `Polorizer` and `Depolorizer` which are write-only and read-only respectively, allowing sequential encoding/decoding of objects and wire elements into them. This capability can be leveraged to implement the `Polorizable` and `Depolorizable` interfaces which describe the custom serialization form for an object.

**Note**: This capability can be dangerous if not implemented correctly, it generally recommended that both interfaces be implemented and are evenly capable of encoding/decoding the same contents to avoid inconsistency. It is intended to be used for object such as Go Interfaces which are not supported by default when using the reflection based `Polorize` and `Depolorize` functions.

A custom `Depolorize` method must read all the data of its element. The decoder returns an error if the method leaves the element unread (a Null is the only exception), or if it does not read all the elements of a pack that it unpacked. If you call a `Depolorize` method yourself, check `Done()`.

### Differential Messaging (Coming Soon)
POLO's partially encoding and field order based indexing (and string based indexing for document encoded wires) allows the possibility for messaging that only allows the transmission of the difference between two states, this is useful for any version managment system where the same data is incrementally updated and transmitted, the ability to index the difference and only transmit the difference can result in massive reduction in the wire sizes for these use cases that often re-transmit already available information.

## Null Values
Null means nil. The target type decides what a Null (`WireNull`) decodes to:

- Pointers, slices (also `[]byte`), maps, `Document`, `Any` and `Raw` can be nil. A Null decodes to nil, and `Depolorize` sets the target to nil.
- For a pointer, go-polo consumes the Null and returns a nil pointer before it decodes the element type. A custom `Depolorize` method of the element type is not called.
- Booleans, integers, floats, strings, arrays, structs and `big.Int` values cannot be nil. A Null is an error at every level (top level, struct field, slice, array or map element). The error matches `polo.ErrNullValue` with `errors.Is`.
- A type that is not a pointer and has a custom `Depolorize` method receives the Null and decides for itself.
- Interfaces are not supported as targets, also not for a Null: the wire does not tell which concrete type to decode. Use the concrete type, or decode into `Any` and decode the `Any` later. An interface with a `Polorize` method encodes the value that it holds, so a nil interface encodes as a Null, as a nil pointer does.

The typed `Depolorizer` methods follow the same rule. `DepolorizeBytes`, `DepolorizeBigInt`, `DepolorizeDocument`, `DepolorizeAny`, `DepolorizeRaw` and `DepolorizePacked` return nil for a Null. A nil `*Depolorizer` has no elements. `DepolorizeBool`, `DepolorizeString`, the integer and float methods and `DepolorizeBytes32` return an error that matches `polo.ErrNullValue`.

## Strings and Bytes
The wire is the same for strings and bytes. A POLO string is always valid UTF-8; data that may hold any bytes is encoded as bytes.

`WireWord` carries both, and the target type decides:

- Decoded into a string (`string`, a named string type, a string map key or a `Document` key), the data must be valid UTF-8 (`utf8.Valid`). Otherwise the error matches `polo.ErrInvalidUTF8`. There is no normalization: NUL, the byte order mark and U+FFFD are ordinary characters.
- Decoded into bytes (`[]byte`, `[N]byte`, `Raw` or `Any`), the data can be any bytes.

| `WireWord` data | `utf8.Valid` | into a string | into bytes |
|---|---|---|---|
| empty, `61`, `00` (NUL), `efbbbf` (BOM), `efbfbd` (U+FFFD), `f09f9880`, `f48fbfbf` (U+10FFFF) | yes | accepted | accepted |
| `ff`, `80` (lone continuation byte), `c3` (truncated), `61ff62` | no | error | accepted |
| `c080`, `e080af`, `f0808080` (overlong) | no | error | accepted |
| `eda080`, `edbfbf` (surrogates) | no | error | accepted |
| `f4908080` (above U+10FFFF), `f5808080`, `f888808080` (5-byte form) | no | error | accepted |

The encoder applies the same rule: `Polorize` returns an error that matches `polo.ErrInvalidUTF8` for a string, a string map key or a `Document` key that is not valid UTF-8.

`Polorizer.PolorizeString` and `Polorizer.PolorizeDocument` have no error result. They write the value, and the `Polorizer` keeps the error. `Polorizer.Err()` returns it, and `Polorize` returns it for a type whose custom `Polorize` method wrote the value. If you use a `Polorizer` directly, check `Err()` before you use `Bytes()`. `Document.Bytes()` does not check the keys: use `Polorize(document)` for a check.

A field that can hold any bytes must be a `[]byte`. Changing a field from `string` to `[]byte` does not change the wire.

## Canonical Decoding
Up to the nesting limit, the decoder accepts exactly the bytes that the encoder produces, so `Polorize(Depolorize(wire)) == wire` for every wire that decodes:

- Tag varints are minimally encoded, and the offset of a top-level tag is zero.
- Integers have no leading zero byte, zero is an empty `WirePosInt` and negative zero is an error. The value must fit the target type (`int8` accepts -128 to 127). `math.MinInt64` decodes.
- `WireNull`, `WireFalse` and `WireTrue` have no data.
- A `[N]byte` needs exactly N bytes, a `[N]T` exactly N elements, and a float exactly 4 or 8 bytes. NaN decodes and keeps its bits.
- In a pack, the first element starts at offset zero. A struct pack has exactly one element for every exported field that is not skipped.
- Map keys and document keys are in strictly ascending order, without duplicates. NaN map keys are rejected on encode and decode.
- A float map key is not `-0`. `-0` and `+0` are the same key of a Go map, so the encoder writes that key as `+0` (also in an array key) and the decoder rejects a `-0` key. A `-0` that is not a map key keeps its bits.

### One form for each type
There are no encoding options. Each type has one wire form, and the decoder accepts only that form:

| Type | Form |
|---|---|
| `[]byte`, `[N]byte` | Word |
| struct | Pack |
| struct that is a `DocumentStruct` | Document |
| map (also with string keys) | Pack |
| `Document` | Document |

A struct type is a `DocumentStruct` if it has the method `PoloDocument()`. The method has no function: it only marks the type. The form is a property of the type, so a `DocumentStruct` is a document at all levels: at the top level, in a field, in a map value and in a slice element.

```go
type Transfer struct {
	Beneficiary [32]byte `polo:"beneficiary"`
	Amount      uint64   `polo:"amount"`
}

func (Transfer) PoloDocument() {}
```

The document of a `DocumentStruct` has one key for each exported field that is not skipped: the field name, or the name in the `polo` tag. The encoder writes all the keys. The decoder rejects an unknown key, and gives the zero value to the field of an absent key. Two fields with the same key are an error.

`PolorizeDocument(object)` converts a struct or a map with string keys into a `Document`. Each value has the form of its own type. For a struct that is not a `DocumentStruct`, the bytes of that `Document` do not decode into the struct: decode them into a `Document` and use `Get` for each value.

### Where the canonical rule stops
- `Any` and `Raw` hold encoded data as is. For an `Any`, the decoder and the encoder check only the tag: it is canonical, the wire type is not reserved and not Load, and a Null, False or True has no data. They do not examine the data after the tag, because its rules depend on the target type. Thus, an `Any` accepts `03 05` and also `03 00 05`. The content of a `Raw` is not examined at all. The canonical rules apply when you decode the data into a typed target.
- Absent keys of a `DocumentStruct`. Another encoder, for example the PISA runtime, leaves out the key of a field that has no value, so the decoder accepts an absent key. Thus, a document without a key and the document with the zero value for that key decode into the same struct. An unknown key is always an error, so a wire cannot hold data that the struct does not read.
- Custom codecs. The author of a `Polorize` and a `Depolorize` method must make sure that the type has one wire. The decoder checks only that the `Depolorize` method reads all the data of its element.

The encoder has no nesting limit. It can write a wire with more levels than the decoder accepts (`MaxDepth`, 1000 levels), and the decoder rejects that wire with `ErrMaxDepthExceeded`.

## Examples
### Simple Polorization & Depolorization (Encoding/Decoding)
https://github.com/sarvalabs/go-polo/blob/22a975e4d1d5329e16aaedd7207aee382e64d30e/polo_test.go#L16-L64

### Custom Polorization & Depolorization (Encoding/Decoding)
https://github.com/sarvalabs/go-polo/blob/22a975e4d1d5329e16aaedd7207aee382e64d30e/polo_test.go#L66-L173

### Wire Decoding with `Any`
https://github.com/sarvalabs/go-polo/blob/22a975e4d1d5329e16aaedd7207aee382e64d30e/polo_test.go#L175-L212

### `Polorizer` (Encoding Buffer)
https://github.com/sarvalabs/go-polo/blob/22a975e4d1d5329e16aaedd7207aee382e64d30e/polorizer_test.go#L12-L42

### `Depolorizer` (Decoding Buffer)
https://github.com/sarvalabs/go-polo/blob/22a975e4d1d5329e16aaedd7207aee382e64d30e/depolorizer_test.go#L13-L70

### Document Encoding
https://github.com/sarvalabs/go-polo/blob/22a975e4d1d5329e16aaedd7207aee382e64d30e/document_test.go#L12-L74

### Document Decoding
https://github.com/sarvalabs/go-polo/blob/22a975e4d1d5329e16aaedd7207aee382e64d30e/document_test.go#L76-L124

## Contributing
Unless you explicitly state otherwise, any contribution intentionally submitted
for inclusion in the work by you, as defined in the Apache-2.0 license, shall be
dual licensed as below, without any additional terms or conditions.

## License
&copy; 2024 Sarva Labs Inc. & MOI Protocol Developers.

This project is licensed under either of
- [Apache License, Version 2.0](https://www.apache.org/licenses/LICENSE-2.0) ([`LICENSE-APACHE`](LICENSE-APACHE))
- [MIT license](https://opensource.org/licenses/MIT) ([`LICENSE-MIT`](LICENSE-MIT))

at your option.

The [SPDX](https://spdx.dev) license identifier for this project is `MIT OR Apache-2.0`.