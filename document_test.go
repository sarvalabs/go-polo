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

// Example_depolorizeDocumentToDocument is an example of using the Depolorize
// function to decode a document-encoded wire into a Document object
//
//nolint:lll
func Example_depolorizeDocumentToDocument() {
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

// FruitDocument is a Fruit that is encoded as a document: it is a DocumentStruct
type FruitDocument Fruit

// PoloDocument marks FruitDocument as a DocumentStruct
func (FruitDocument) PoloDocument() {}

// Example_depolorizeDocumentToStruct is an example of using the Depolorize
// function to decode a document encoded wire into a FruitDocument object
func Example_depolorizeDocumentToStruct() {
	wire := []byte{
		13, 175, 1, 6, 69, 182, 1, 133, 2, 230, 4, 165, 5, 78, 97, 109, 101, 6, 111, 114, 97,
		110, 103, 101, 97, 108, 105, 97, 115, 14, 63, 6, 150, 1, 116, 97, 110, 103, 101, 114,
		105, 110, 101, 109, 97, 110, 100, 97, 114, 105, 110, 99, 111, 115, 116, 3, 1, 44,
	}

	// Create a new instance of FruitDocument
	object := new(FruitDocument)
	// Deserialize the document bytes into the FruitDocument object
	if err := Depolorize(object, wire); err != nil {
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

// docObjectA and docObjectB are DocumentStructs for the encode tests
type docObjectA struct {
	A string
	B uint64
}

func (docObjectA) PoloDocument() {}

type docObjectB struct {
	A chan int
	B string
}

func (docObjectB) PoloDocument() {}

//nolint:lll
func TestDocument_Encode(t *testing.T) {
	tests := []struct {
		name   string
		object any
		wire   []byte
		err    string
	}{
		{
			"valid document struct",
			docObjectA{A: "foo", B: 300},
			[]byte{13, 79, 6, 21, 86, 101, 65, 6, 102, 111, 111, 66, 3, 1, 44},
			"",
		},
		{
			"string map is a pack",
			map[string]string{"foo": "bar", "boo": "far"},
			[]byte{14, 95, 6, 54, 102, 150, 1, 98, 111, 111, 102, 97, 114, 102, 111, 111, 98, 97, 114},
			"",
		},
		{
			"invalid document struct",
			docObjectB{make(chan int), "foo"},
			nil,
			"could not encode into document: document value could not be encoded for key 'A': incompatible value error: unsupported type: chan int [chan]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := Polorize(test.object)
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

// docObject is a DocumentStruct with custom keys and a skipped field
type docObject struct {
	A int `polo:"boo"`
	B int `polo:"foo"`
	C int `polo:"-"`
}

func (docObject) PoloDocument() {}

func TestDocument_DecodeToStruct(t *testing.T) {
	type Object struct {
		A int `polo:"boo"`
		B int `polo:"foo"`
	}

	tests := []struct {
		name   string
		bytes  []byte
		target any
		object any
		err    string
	}{
		{
			"absent key gives the zero value",
			[]byte{13, 47, 6, 53, 98, 111, 111, 3, 1, 44},
			new(docObject),
			&docObject{A: 300},
			"",
		},
		{
			"empty document",
			[]byte{13, 15},
			new(docObject),
			new(docObject),
			"",
		},
		{
			"null field",
			[]byte{13, 95, 6, 53, 86, 133, 1, 98, 111, 111, 3, 54, 102, 111, 111, 0},
			new(docObject),
			new(docObject),
			"incompatible wire: struct field [polo.docObject.B <int>]: incompatible wire: nil value",
		},
		{
			"absent and unknown keys",
			[]byte{13, 95, 6, 53, 86, 133, 1, 98, 111, 111, 3, 54, 122, 122, 122, 0},
			new(docObject),
			new(docObject),
			"incompatible wire: unknown document key 'zzz' for struct polo.docObject",
		},
		{
			"unknown key",
			Document{"boo": Raw{3, 54}, "foo": Raw{3, 89}, "zzz": Raw{0}}.Bytes(),
			new(docObject),
			new(docObject),
			"incompatible wire: unknown document key 'zzz' for struct polo.docObject",
		},
		{
			"key of a skipped field",
			Document{"C": Raw{3, 1}, "boo": Raw{3, 54}, "foo": Raw{3, 89}}.Bytes(),
			new(docObject),
			new(docObject),
			"incompatible wire: unknown document key 'C' for struct polo.docObject",
		},
		{
			"pack for a document struct",
			[]byte{14, 47, 3, 19, 54, 89},
			new(docObject),
			new(docObject),
			"incompatible wire: unexpected wiretype 'pack'. expected one of: {null, document}",
		},
		{
			"document for a struct that is not a document struct",
			[]byte{13, 95, 6, 53, 86, 133, 1, 98, 111, 111, 3, 54, 102, 111, 111, 3, 89},
			new(Object),
			new(Object),
			"incompatible wire: unexpected wiretype 'document'. expected one of: {null, pack}",
		},
		{
			"document for a string map",
			[]byte{13, 47, 6, 53, 98, 111, 111, 3, 1, 44},
			new(map[string]int),
			new(map[string]int),
			"incompatible wire: unexpected wiretype 'document'. expected one of: {null, pack}",
		},
		{
			"all keys",
			[]byte{13, 95, 6, 53, 86, 133, 1, 98, 111, 111, 3, 54, 102, 111, 111, 3, 89},
			new(docObject),
			&docObject{A: 54, B: 89},
			"",
		},
		{
			"extra incompatible fields",
			[]byte{13, 95, 7, 53, 86, 133, 1, 98, 111, 111, 3, 54, 102, 111, 111, 3, 89},
			new(docObject),
			new(docObject),
			"incompatible wire: unexpected wiretype 'float'. expected one of: {null, word}",
		},
		{
			"malformed fields",
			[]byte{13, 47, 6, 53, 98, 111, 111, 142},
			new(docObject),
			new(docObject),
			"malformed tag: varint terminated prematurely",
		},
		{
			"non raw value",
			[]byte{13, 47, 6, 54, 98, 111, 111, 3, 1, 44},
			new(docObject),
			new(docObject),
			"incompatible wire: unexpected wiretype 'word'. expected one of: {raw}",
		},
		{
			"incompatible field",
			[]byte{13, 47, 6, 53, 98, 111, 111, 6, 1, 44},
			new(docObject),
			new(docObject),
			"incompatible wire: struct field [polo.docObject.A <int>]: incompatible wire: unexpected wiretype 'word'. expected one of: {null, posint, negint}", //nolint:lll
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Depolorize(test.target, test.bytes)
			assert.Equal(t, test.object, test.target)

			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.EqualError(t, err, test.err)
			}
		})
	}
}

// docObject1, docObject2 and docObject3 are DocumentStructs that are nested in each other
type docObject1 struct {
	A string
	B uint64
}

func (docObject1) PoloDocument() {}

type docObject2 struct {
	A uint32
	B docObject1
}

func (docObject2) PoloDocument() {}

type docObject3 struct {
	A []string
	B docObject2
}

func (docObject3) PoloDocument() {}

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

	docObject := docObject3{
		A: []string{"foo", "boo"},
		B: docObject2{
			A: 500,
			B: docObject1{
				A: "foo",
				B: 300,
			},
		},
	}

	// Simple Encoding - Just serialize the object with regular POLO encoding
	// It will result in a pack-encoded wire
	simpleEncoded, err := Polorize(object)
	require.NoError(t, err)

	// Shallow Document Encoding - Convert the object into a Document.
	// It will result in a doc-encoded wire with only Object3 fields being doc-encoded.
	// The structs in the fields are not DocumentStructs, so they are pack-encoded
	shallow, err := PolorizeDocument(object)
	require.NoError(t, err)

	shallowEncoded := shallow.Bytes()

	// Deep Document Encoding - Serialize the DocumentStruct object.
	// It will result in a doc-encoded wire with all structs at all depths being doc-encoded.
	deepEncoded, err := Polorize(docObject)
	require.NoError(t, err)

	// PolorizeDocument of a DocumentStruct gives the same wire as Polorize
	deep, err := PolorizeDocument(docObject)
	require.NoError(t, err)
	require.Equal(t, deepEncoded, deep.Bytes())

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

	// The shallow doc-encoded wire does not decode into a struct. Object3 rejects the document,
	// and docObject3 rejects the pack of Object2 in its field B (docObject2 is a document).
	shallowDecoded := new(Object3)
	require.EqualError(t, Depolorize(shallowDecoded, shallowEncoded),
		"incompatible wire: unexpected wiretype 'document'. expected one of: {null, pack}")
	require.EqualError(t, Depolorize(new(docObject3), shallowEncoded),
		"incompatible wire: struct field [polo.docObject3.B <polo.docObject2>]: incompatible wire: unexpected wiretype 'pack'. expected one of: {null, document}")

	// Decode the object from the shallow doc-encoded wire.
	// Decode the wire into a Document, and decode each field from the Document
	shallowDocument := make(Document)
	require.NoError(t, Depolorize(&shallowDocument, shallowEncoded))
	require.Equal(t, shallow, shallowDocument)
	require.NoError(t, shallowDocument.Get("A", &shallowDecoded.A))
	require.NoError(t, shallowDocument.Get("B", &shallowDecoded.B))

	// Decode the object from the deep doc-encoded wire
	deepDecoded := new(docObject3)
	err = Depolorize(deepDecoded, deepEncoded)
	require.NoError(t, err)

	// All decoded objects must be equal
	require.Equal(t, simpleDecoded, shallowDecoded)
	require.Equal(t, &object, simpleDecoded)
	require.Equal(t, &docObject, deepDecoded)
}
