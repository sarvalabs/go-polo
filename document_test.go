package polo

import (
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ExampleDocument is an example for using the Document object's method to partially encode
// fields as properties into it and then serialize it into document encoded POLO bytes
//
//nolint:lll
func ExampleDocument() {
	// Create a new Document
	document := make(Document)

	// Encode the 'Name' field
	if err := document.Set("Name", "orange"); err != nil {
		log.Fatalln(err)
	}

	// Encode the 'cost' field
	if err := document.Set("cost", 300); err != nil {
		log.Fatalln(err)
	}

	// Encode the 'alias' field
	if err := document.Set("alias", []string{"tangerine", "mandarin"}); err != nil {
		log.Fatalln(err)
	}

	// Print the Document object and it serialized bytes
	fmt.Println(document)
	fmt.Println(document.Bytes())

	// Output:
	// map[Name:[6 111 114 97 110 103 101] alias:[14 63 6 150 1 116 97 110 103 101 114 105 110 101 109 97 110 100 97 114 105 110] cost:[3 1 44]]
	// [13 175 1 6 69 182 1 133 2 230 4 165 5 78 97 109 101 6 111 114 97 110 103 101 97 108 105 97 115 14 63 6 150 1 116 97 110 103 101 114 105 110 101 109 97 110 100 97 114 105 110 99 111 115 116 3 1 44]
}

// ExamplePolorizeDocument is an example for using PolorizeDocument to encode a
// struct into a Document and then further serializing it into document encoded POLO bytes
//
//nolint:lll
func ExamplePolorizeDocument() {
	// Create a Fruit object
	orange := &Fruit{"orange", 300, []string{"tangerine", "mandarin"}}

	// Encode the object into a Document
	document, err := PolorizeDocument(orange)
	if err != nil {
		log.Fatalln(err)
	}

	// Print the Document object
	fmt.Println(document)

	// Serialize the Document object
	// This can also be done with document.Bytes()
	wire, err := Polorize(document)
	if err != nil {
		log.Fatalln(err)
	}

	// Print the serialized Document
	fmt.Println(wire)

	// Output:
	// map[Name:[6 111 114 97 110 103 101] alias:[14 63 6 150 1 116 97 110 103 101 114 105 110 101 109 97 110 100 97 114 105 110] cost:[3 1 44]]
	// [13 175 1 6 69 182 1 133 2 230 4 165 5 78 97 109 101 6 111 114 97 110 103 101 97 108 105 97 115 14 63 6 150 1 116 97 110 103 101 114 105 110 101 109 97 110 100 97 114 105 110 99 111 115 116 3 1 44]
}

// ExampleDepolorizeDocument_ToDocument is an example of using the Depolorize
// function to decode a document-encoded wire into a Document object
//
//nolint:govet, lll
func ExampleDepolorizeDocument_ToDocument() {
	wire := []byte{
		13, 175, 1, 6, 69, 182, 1, 133, 2, 230, 4, 165, 5, 78, 97, 109, 101, 6, 111, 114, 97,
		110, 103, 101, 97, 108, 105, 97, 115, 14, 63, 6, 150, 1, 116, 97, 110, 103, 101, 114,
		105, 110, 101, 109, 97, 110, 100, 97, 114, 105, 110, 99, 111, 115, 116, 3, 1, 44,
	}

	// Create a new Document
	doc := make(Document)
	// Deserialize the document bytes into a Document
	if err := Depolorize(&doc, wire); err != nil {
		log.Fatalln(err)
	}

	// Print the decoded Document
	fmt.Println(doc)

	// Output:
	// map[Name:[6 111 114 97 110 103 101] alias:[14 63 6 150 1 116 97 110 103 101 114 105 110 101 109 97 110 100 97 114 105 110] cost:[3 1 44]]
}

// ExampleDepolorizeDocument_ToStruct is an example of using the Depolorize
// function to decode a document encoded wire into a Fruit object
//
//nolint:govet
func ExampleDepolorizeDocument_ToStruct() {
	wire := []byte{
		13, 175, 1, 6, 69, 182, 1, 133, 2, 230, 4, 165, 5, 78, 97, 109, 101, 6, 111, 114, 97,
		110, 103, 101, 97, 108, 105, 97, 115, 14, 63, 6, 150, 1, 116, 97, 110, 103, 101, 114,
		105, 110, 101, 109, 97, 110, 100, 97, 114, 105, 110, 99, 111, 115, 116, 3, 1, 44,
	}

	// Create a new instance of Fruit
	object := new(Fruit)
	// Deserialize the document bytes into the Fruit object
	if err := Depolorize(object, wire, DocStructs()); err != nil {
		log.Fatalln(err)
	}

	// Print the deserialized object
	fmt.Println(object)

	// Output:
	// &{orange 300 [tangerine mandarin]}
}

func TestDocument_Bytes(t *testing.T) {
	tests := []struct {
		name string
		doc  Document
		wire []byte
	}{
		{"empty doc", Document{}, []byte{13, 15}},
		{
			"doc with one key",
			Document{"foo": []byte{6, 1, 0, 1, 0}},
			[]byte{13, 47, 6, 53, 102, 111, 111, 6, 1, 0, 1, 0},
		},
		{
			"doc with multiple keys",
			Document{"foo": []byte{3, 1, 0, 1, 0}, "bar": []byte{6, 2, 1, 2, 1}},
			[]byte{13, 111, 6, 53, 134, 1, 181, 1, 98, 97, 114, 6, 2, 1, 2, 1, 102, 111, 111, 3, 1, 0, 1, 0},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wire := test.doc.Bytes()
			assert.Equal(t, test.wire, wire)
		})
	}
}

func TestDocument_Size(t *testing.T) {
	tests := []struct {
		name string
		doc  Document
		size int
	}{
		{"empty doc", Document{}, 0},
		{"doc with one key", Document{"foo": []byte{1, 0, 1, 0}}, 1},
		{"doc with multiple keys", Document{"foo": []byte{1, 0, 1, 0}, "bar": []byte{}}, 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			size := test.doc.Size()
			assert.Equal(t, test.size, size)
		})
	}
}

func TestDocument_GetSetRaw(t *testing.T) {
	// Create a Document
	doc := make(Document)
	// Set some fields into the document
	doc.SetRaw("foo", Raw{1, 0, 1, 0})
	doc.SetRaw("bar", Raw{2, 1, 2, 1})

	// Attempt to retrieve some unset keys from the document and confirm nil
	assert.Nil(t, doc.GetRaw("far"))
	assert.Nil(t, doc.GetRaw("boo"))
	// Attempt to retrieve some set keys from the document and confirm equality
	assert.Equal(t, Raw{1, 0, 1, 0}, doc.GetRaw("foo"))
	assert.Equal(t, Raw{2, 1, 2, 1}, doc.GetRaw("bar"))
}

func TestDocument_Set(t *testing.T) {
	// Create a Document
	doc := make(Document)

	var err error

	// Set some objects into the document and confirm nil errors
	err = doc.Set("foo", 25)
	require.Nil(t, err)
	err = doc.Set("bar", "hello")
	require.Nil(t, err)

	// Get the raw data for the keys from the document and confirm equality
	assert.Equal(t, Raw{3, 25}, doc.GetRaw("foo"))
	assert.Equal(t, Raw{6, 104, 101, 108, 108, 111}, doc.GetRaw("bar"))

	err = doc.Set("far", make(chan int))
	assert.EqualError(t, err, "document value could not be encoded for key 'far': incompatible value error: unsupported type: chan int [chan]") //nolint:lll
}

func TestDocument_Get(t *testing.T) {
	// Create a Document
	doc := make(Document)

	var err error

	// Set some objects into the document and confirm nil errors
	err = doc.Set("foo", 25)
	require.Nil(t, err)
	err = doc.Set("bar", "hello")
	require.Nil(t, err)

	// Attempt to retrieve the integer object from the Document.
	// Test for nil error and value equality
	var foo int
	err = doc.Get("foo", &foo)
	assert.Nil(t, err)
	assert.Equal(t, 25, foo)

	// Attempt to retrieve the string object from the Document.
	// Test for nil error and value equality
	var bar string
	err = doc.Get("bar", &bar)
	assert.Nil(t, err)
	assert.Equal(t, "hello", bar)

	// Attempt to retrieve object from non-existent field from the Document.
	// Tests error string
	err = doc.Get("far", &foo)
	assert.EqualError(t, err, "document value not found for key 'far'")

	// Attempt to retrieve object from a field with wrong type from Document.
	// Test error string
	err = doc.Get("bar", &foo)
	assert.EqualError(t, err, "document value could not be decoded for key 'bar': incompatible wire: unexpected wiretype 'word'. expected one of: {null, posint, negint}") //nolint:lll
}

//nolint:lll
func TestPolorizeDocument(t *testing.T) {
	type ObjectA struct {
		A string
		B string
	}

	type ObjectB struct {
		A string `polo:"-"`
		B uint64
		C bool `polo:"foo"`
		d float32
	}

	type ObjectC struct {
		A chan int
		B string
	}

	nilObject := func() *ObjectA { return nil }

	tests := []struct {
		name   string
		object any
		doc    Document
		bytes  []byte
		err    string
	}{
		{
			"map with string keys & values",
			map[string]string{"boo": "far"},
			Document{"boo": []byte{0x6, 102, 97, 114}},
			[]byte{13, 47, 6, 53, 98, 111, 111, 6, 102, 97, 114},
			"",
		},
		{
			"map with string keys & int values",
			map[string]int64{"bar": 54, "foo": -89},
			Document{"bar": []byte{3, 54}, "foo": []byte{4, 89}},
			[]byte{13, 95, 6, 53, 86, 133, 1, 98, 97, 114, 3, 54, 102, 111, 111, 4, 89},
			"",
		},
		{
			"struct with string fields",
			ObjectA{"foo", "bar"},
			Document{"A": []byte{0x6, 102, 111, 111}, "B": []byte{0x6, 98, 97, 114}},
			[]byte{13, 79, 6, 21, 86, 101, 65, 6, 102, 111, 111, 66, 6, 98, 97, 114},
			"",
		},
		{
			"pointer to struct with string fields",
			&ObjectA{"foo", "bar"},
			Document{"A": []byte{0x6, 102, 111, 111}, "B": []byte{0x6, 98, 97, 114}},
			[]byte{13, 79, 6, 21, 86, 101, 65, 6, 102, 111, 111, 66, 6, 98, 97, 114},
			"",
		},
		{
			"struct with mixed valid fields",
			ObjectB{"foo", 64, false, 54.2},
			Document{"B": []byte{3, 64}, "foo": []byte{1}},
			[]byte{13, 79, 6, 21, 54, 101, 66, 3, 64, 102, 111, 111, 1},
			"",
		},
		{
			"map with invalid value",
			map[string]chan int{"foo": make(chan int)},
			nil,
			nil,
			"could not encode into document: document value could not be encoded for key 'foo': incompatible value error: unsupported type: chan int [chan]",
		},
		{
			"struct with invalid field",
			ObjectC{make(chan int), "foo"},
			nil,
			nil,
			"could not encode into document: document value could not be encoded for key 'A': incompatible value error: unsupported type: chan int [chan]",
		},
		{
			"nil pointer to struct",
			nilObject(), nil, nil,
			"could not encode into document: unsupported type: nil pointer",
		},
		{
			"simple nil",
			nil, nil, nil,
			"could not encode into document: unsupported type",
		},
		{
			"map with non-string key",
			map[uint64]uint64{0: 56},
			nil,
			nil,
			"could not encode into document: unsupported type: map type with non string key",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := PolorizeDocument(test.object)
			if test.err == "" {
				assert.Nil(t, err)
				assert.Equal(t, test.doc, doc)
				assert.Equal(t, test.bytes, doc.Bytes())
			} else {
				assert.EqualError(t, err, test.err)
				assert.Nil(t, doc)
			}
		})
	}
}

//nolint:lll
func TestDocument_Encode(t *testing.T) {
	type ObjectA struct {
		A string
		B uint64
	}

	type ObjectB struct {
		A chan int
		B string
	}

	tests := []struct {
		name    string
		object  any
		options []EncodingOptions
		wire    []byte
		err     string
	}{
		{
			"valid struct & DocStructs enabled",
			ObjectA{A: "foo", B: 300},
			[]EncodingOptions{DocStructs()},
			[]byte{13, 79, 6, 21, 86, 101, 65, 6, 102, 111, 111, 66, 3, 1, 44},
			"",
		},
		{
			"valid string map & DocStringMaps enabled",
			map[string]string{"foo": "bar", "boo": "far"},
			[]EncodingOptions{DocStringMaps()},
			[]byte{13, 95, 6, 53, 118, 165, 1, 98, 111, 111, 6, 102, 97, 114, 102, 111, 111, 6, 98, 97, 114},
			"",
		},
		{
			"invalid struct & DocStructs enabled",
			ObjectB{make(chan int), "foo"},
			[]EncodingOptions{DocStructs()},
			nil,
			"could not encode into document: document value could not be encoded for key 'A': incompatible value error: unsupported type: chan int [chan]",
		},
		{
			"invalid string map & DocStringMaps enabled",
			map[string]chan int{"foo": make(chan int)},
			[]EncodingOptions{DocStringMaps()},
			nil,
			"could not encode into document: document value could not be encoded for key 'foo': incompatible value error: unsupported type: chan int [chan]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := Polorize(test.object, test.options...)
			if test.err == "" {
				assert.Nil(t, err)
				assert.Equal(t, test.wire, encoded)
			} else {
				assert.EqualError(t, err, test.err)
			}
		})
	}
}

func TestDocument_DecodeToDocument(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
		doc   Document
		err   string
	}{
		{
			"doc with one key",
			[]byte{13, 47, 6, 53, 98, 111, 111, 6, 102, 97, 114},
			Document{"boo": []byte{6, 102, 97, 114}},
			"",
		},
		{
			"doc with multiple integer values",
			[]byte{13, 95, 6, 53, 86, 133, 1, 98, 97, 114, 3, 54, 102, 111, 111, 3, 89},
			Document{"bar": []byte{3, 54}, "foo": []byte{3, 89}},
			"",
		},
		{
			"doc with multiple string values",
			[]byte{13, 79, 6, 21, 86, 101, 65, 6, 102, 111, 111, 66, 6, 98, 97, 114},
			Document{"A": []byte{6, 102, 111, 111}, "B": []byte{6, 98, 97, 114}},
			"",
		},
		{
			"doc with multiple fields",
			[]byte{13, 79, 6, 21, 54, 101, 66, 3, 64, 102, 111, 111, 1},
			Document{"B": []byte{3, 64}, "foo": []byte{1}},
			"",
		},
		{
			"pack wire",
			[]byte{14, 79, 6, 21, 54, 101, 66, 3, 64, 102, 111, 111, 1},
			Document{},
			"incompatible wire: unexpected wiretype 'pack'. expected one of: {null, document}",
		},
		{
			"null wire",
			[]byte{0},
			nil, "",
		},
		{
			"empty doc",
			[]byte{13, 15},
			Document{},
			"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc := make(Document)
			err := Depolorize(&doc, test.bytes)
			assert.Equal(t, test.doc, doc)

			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.EqualError(t, err, test.err)
			}
		})
	}
}

func TestDocument_DecodeToStruct(t *testing.T) {
	type Object struct {
		A int `polo:"boo"`
		B int `polo:"foo"`
		C int `polo:"-"`
	}

	tests := []struct {
		name    string
		bytes   []byte
		target  any
		object  any
		options []EncodingOptions
		err     string
	}{
		{
			"decode to struct with DocStructs enabled",
			[]byte{13, 47, 6, 53, 98, 111, 111, 3, 1, 44},
			new(Object),
			&Object{A: 300},
			[]EncodingOptions{DocStructs()},
			"",
		},
		{
			"decode to struct with DocStructs enabled & extra fields",
			[]byte{13, 95, 6, 53, 86, 133, 1, 98, 111, 111, 3, 54, 102, 111, 111, 0},
			new(Object),
			&Object{A: 54},
			[]EncodingOptions{DocStructs()},
			"",
		},
		{
			"decode to map[string]int with DocStringMaps enabled",
			[]byte{13, 47, 6, 53, 98, 111, 111, 3, 1, 44},
			new(map[string]int),
			&map[string]int{"boo": 300},
			[]EncodingOptions{DocStringMaps()},
			"",
		},
		{
			"decode to map[string]int with DocStringMaps enabled",
			[]byte{13, 95, 6, 53, 86, 133, 1, 98, 111, 111, 3, 54, 102, 111, 111, 0},
			new(map[string]int),
			&map[string]int{"boo": 54, "foo": 0},
			[]EncodingOptions{DocStringMaps()},
			"",
		},
		{
			"decode to map[string][]string with DocStringMaps enabled",
			[]byte{13, 79, 6, 53, 70, 117, 98, 97, 114, 0, 102, 111, 111, 14, 47, 6, 54, 98, 111, 111, 119, 111, 111},
			new(map[string][]string),
			&map[string][]string{"foo": {"boo", "woo"}, "bar": nil},
			[]EncodingOptions{DocStringMaps()},
			"",
		},
		{
			"decode to struct with DocStructs enabled",
			[]byte{13, 95, 6, 53, 86, 133, 1, 98, 111, 111, 3, 54, 102, 111, 111, 3, 89},
			new(Object),
			&Object{A: 54, B: 89},
			[]EncodingOptions{DocStructs()},
			"",
		},
		{
			"decode to struct with DocStructs enabled & extra incompatible fields",
			[]byte{13, 95, 7, 53, 86, 133, 1, 98, 111, 111, 3, 54, 102, 111, 111, 3, 89},
			new(Object),
			new(Object),
			[]EncodingOptions{DocStructs()},
			"incompatible wire: unexpected wiretype 'float'. expected one of: {null, word}",
		},
		{
			"decode to struct with DocStructs enabled & malformed fields",
			[]byte{13, 47, 6, 53, 98, 111, 111, 142},
			new(Object),
			new(Object),
			[]EncodingOptions{DocStructs()},
			"malformed tag: varint terminated prematurely",
		},
		{
			"decode to struct with DocStructs enabled & non raw value",
			[]byte{13, 47, 6, 54, 98, 111, 111, 3, 1, 44},
			new(Object),
			new(Object),
			[]EncodingOptions{DocStructs()},
			"incompatible wire: unexpected wiretype 'word'. expected one of: {raw}",
		},
		{
			"decode to struct with DocStructs enabled & incompatible field",
			[]byte{13, 47, 6, 53, 98, 111, 111, 6, 1, 44},
			new(Object),
			new(Object),
			[]EncodingOptions{DocStructs()},
			"incompatible wire: struct field [polo.Object.A <int>]: incompatible wire: unexpected wiretype 'word'. expected one of: {null, posint, negint}", //nolint:lll
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Depolorize(test.target, test.bytes, test.options...)
			assert.Equal(t, test.object, test.target)

			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.EqualError(t, err, test.err)
			}
		})
	}
}

//nolint:lll
func TestDocument_DeepStructs(t *testing.T) {
	type Object1 struct {
		A string
		B uint64
	}

	type Object2 struct {
		A uint32
		B Object1
	}

	type Object3 struct {
		A []string
		B Object2
	}

	object := Object3{
		A: []string{"foo", "boo"},
		B: Object2{
			A: 500,
			B: Object1{
				A: "foo",
				B: 300,
			},
		},
	}

	// Simple Encoding - Just serialize the object with regular POLO encoding
	// It will result in a pack-encoded wire
	simpleEncoded, err := Polorize(object)
	require.NoError(t, err)

	// Shallow Document Encoding - Serialize the object with shallow document encoding.
	// It will result in a doc-encoded wire with only Object3 fields being doc-encoded.
	// Any deeper structs will be regularly pack-encoded
	shallow, err := PolorizeDocument(object)
	require.NoError(t, err)

	shallowEncoded := shallow.Bytes()

	// Deep Document Encoding - Serialize the object with deep document encoding.
	// It will result in a doc-encoded wire with all structs at all depths being doc-encoded.
	deepEncoded, err := Polorize(object, DocStructs())
	require.NoError(t, err)

	// None of the 3 generated wires must be the same.
	require.NotEqual(t, simpleEncoded, deepEncoded)
	require.NotEqual(t, simpleEncoded, shallowEncoded)
	require.NotEqual(t, shallowEncoded, deepEncoded)

	require.Equal(t, simpleEncoded, []byte{14, 63, 14, 158, 1, 47, 6, 54, 102, 111, 111, 98, 111, 111, 47, 3, 46, 1, 244, 47, 6, 51, 102, 111, 111, 1, 44})
	require.Equal(t, shallowEncoded, []byte{13, 111, 6, 21, 182, 1, 197, 1, 65, 14, 47, 6, 54, 102, 111, 111, 98, 111, 111, 66, 14, 47, 3, 46, 1, 244, 47, 6, 51, 102, 111, 111, 1, 44})
	require.Equal(t, deepEncoded, []byte{13, 111, 6, 21, 182, 1, 197, 1, 65, 14, 47, 6, 54, 102, 111, 111, 98, 111, 111, 66, 13, 79, 6, 21, 70, 85, 65, 3, 1, 244, 66, 13, 79, 6, 21, 86, 101, 65, 6, 102, 111, 111, 66, 3, 1, 44})

	// Decode the object from the simple-encoded wire
	simpleDecoded := new(Object3)
	err = Depolorize(simpleDecoded, simpleEncoded)
	require.NoError(t, err)

	// Decode the object from the shallow doc-encoded wire.
	// Requires the DocStructs options to decode to top-level doc encoding
	shallowDecoded := new(Object3)
	err = Depolorize(shallowDecoded, shallowEncoded, DocStructs())
	require.NoError(t, err)

	// Decode the object from the deep doc-encoded wire
	deepDecoded := new(Object3)
	err = Depolorize(deepDecoded, deepEncoded, DocStructs())
	require.NoError(t, err)

	// All 3 decoded objects must be equal
	require.Equal(t, simpleDecoded, shallowDecoded)
	require.Equal(t, simpleDecoded, deepDecoded)
	require.Equal(t, shallowDecoded, deepDecoded)
}
