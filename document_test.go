package polo

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nolint:lll
func ExampleDocumentEncode() {
	type Fruit struct {
		Name  string
		Cost  int      `polo:"cost"`
		Alias []string `polo:"alias"`
	}

	orange := &Fruit{"orange", 300, []string{"tangerine", "mandarin"}}

	wire, err := DocumentEncode(orange)
	if err != nil {
		fmt.Println("error:", err)
	}

	fmt.Println(wire)

	// Output:
	// [13 175 1 6 70 182 1 246 1 166 2 246 2 78 97 109 101 6 111 114 97 110 103 101 99 111 115 116 3 1 44 97 108 105 97 115 14 63 6 150 1 116 97 110 103 101 114 105 110 101 109 97 110 100 97 114 105 110]
}

// nolint:lll
func ExampleDocument_DecodeToDocument() {
	wire := []byte{
		13, 175, 1, 6, 70, 182, 1, 246, 1, 166, 2, 246, 2, 78, 97, 109, 101, 6, 111, 114, 97,
		110, 103, 101, 99, 111, 115, 116, 3, 1, 44, 97, 108, 105, 97, 115, 14, 63, 6, 150, 1,
		116, 97, 110, 103, 101, 114, 105, 110, 101, 109, 97, 110, 100, 97, 114, 105, 110,
	}

	doc := make(Document)
	if err := Depolorize(&doc, wire); err != nil {
		fmt.Println("error:", err)
	}

	fmt.Println(doc)

	// Output:
	// map[Name:[6 111 114 97 110 103 101] alias:[14 63 6 150 1 116 97 110 103 101 114 105 110 101 109 97 110 100 97 114 105 110] cost:[3 1 44]]
}

func ExampleDocument_DecodeToStruct() {
	type Fruit struct {
		Name  string
		Cost  int      `polo:"cost"`
		Alias []string `polo:"alias"`
	}

	wire := []byte{
		13, 175, 1, 6, 70, 182, 1, 246, 1, 166, 2, 246, 2, 78, 97, 109, 101, 6, 111, 114, 97,
		110, 103, 101, 99, 111, 115, 116, 3, 1, 44, 97, 108, 105, 97, 115, 14, 63, 6, 150, 1,
		116, 97, 110, 103, 101, 114, 105, 110, 101, 109, 97, 110, 100, 97, 114, 105, 110,
	}

	object := new(Fruit)
	if err := Depolorize(object, wire); err != nil {
		fmt.Println("error:", err)
	}

	fmt.Println(object)

	// Output:
	// &{orange 300 [tangerine mandarin]}
}

func TestDocument_Bytes(t *testing.T) {
	tests := []struct {
		doc  Document
		wire []byte
	}{
		{Document{}, []byte{13, 15}},
		{Document{"foo": []byte{1, 0, 1, 0}}, []byte{13, 47, 6, 54, 102, 111, 111, 6, 1, 0, 1, 0}},
		{Document{"foo": []byte{1, 0, 1, 0}, "bar": []byte{2, 1, 2, 1}}, []byte{13, 111, 6, 54, 134, 1, 182, 1, 98, 97, 114, 6, 2, 1, 2, 1, 102, 111, 111, 6, 1, 0, 1, 0}},
	}

	for _, test := range tests {
		wire := test.doc.Bytes()
		assert.Equal(t, test.wire, wire)
	}
}

func TestDocument_Size(t *testing.T) {
	tests := []struct {
		doc  Document
		size int
	}{
		{Document{}, 0},
		{Document{"foo": []byte{1, 0, 1, 0}}, 1},
		{Document{"foo": []byte{1, 0, 1, 0}, "bar": []byte{}}, 2},
	}

	for _, test := range tests {
		size := test.doc.Size()
		assert.Equal(t, test.size, size)
	}
}

func TestDocument_Is(t *testing.T) {
	doc := Document{
		"far": nil,
		"bar": []byte{},
		"tar": []byte{0},
		"foo": []byte{6, 109, 97, 110, 105, 115, 104},
		"boo": []byte{3, 1, 44},
	}

	assert.True(t, doc.Is("foo", WireWord))
	assert.True(t, doc.Is("boo", WirePosInt))
	assert.False(t, doc.Is("boo", WireNegInt))

	assert.True(t, doc.Is("far", WireNull))
	assert.True(t, doc.Is("bar", WireNull))
	assert.True(t, doc.Is("car", WireNull))
	assert.True(t, doc.Is("tar", WireNull))
}

func TestDocument_GetSet(t *testing.T) {
	// Create a Document
	doc := make(Document)
	// Set some fields into the document
	doc.Set("foo", []byte{1, 0, 1, 0})
	doc.Set("bar", []byte{2, 1, 2, 1})

	// Attempt to retrieve some unset keys from the document and confirm nil
	assert.Nil(t, doc.Get("far"))
	assert.Nil(t, doc.Get("boo"))
	// Attempt to retrieve some set keys from the document and confirm equality
	assert.Equal(t, []byte{1, 0, 1, 0}, doc.Get("foo"))
	assert.Equal(t, []byte{2, 1, 2, 1}, doc.Get("bar"))
}

func TestDocument_SetObject(t *testing.T) {
	// Create a Document
	var err error
	doc := make(Document)

	// Set some objects into the document and confirm nil errors
	err = doc.SetObject("foo", 25)
	require.Nil(t, err)
	err = doc.SetObject("bar", "hello")
	require.Nil(t, err)

	// Get the raw data for the keys from the document and confirm equality
	assert.Equal(t, []byte{3, 25}, doc.Get("foo"))
	assert.Equal(t, []byte{6, 104, 101, 108, 108, 111}, doc.Get("bar"))

	err = doc.SetObject("far", make(chan int))
	assert.EqualError(t, err, "document value could not be encoded for key 'far': encode error: unsupported type: chan int [chan]")
}

func TestDocument_GetObject(t *testing.T) {
	// Create a Document
	var err error
	doc := make(Document)

	// Set some objects into the document and confirm nil errors
	err = doc.SetObject("foo", 25)
	require.Nil(t, err)
	err = doc.SetObject("bar", "hello")
	require.Nil(t, err)

	// Attempt to retrieve the integer object from the Document.
	// Test for nil error and value equality
	var foo int
	err = doc.GetObject("foo", &foo)
	assert.Nil(t, err)
	assert.Equal(t, 25, foo)

	// Attempt to retrieve the string object from the Document.
	// Test for nil error and value equality
	var bar string
	err = doc.GetObject("bar", &bar)
	assert.Nil(t, err)
	assert.Equal(t, "hello", bar)

	// Attempt to retrieve object from non-existent field from the Document.
	// Tests error string
	err = doc.GetObject("far", &foo)
	assert.EqualError(t, err, "document value not found for key 'far'")

	// Attempt to retrieve object from a field with wrong type from Document.
	// Test error string
	err = doc.GetObject("bar", &foo)
	assert.EqualError(t, err, "document value could not be decoded for key 'bar': decode error: incompatible wire type. expected: posint. got: word")
}

func TestDocumentEncode(t *testing.T) {
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
		object any
		err    string
		bytes  []byte
	}{
		{
			map[string]string{"boo": "far"},
			"",
			[]byte{13, 47, 6, 54, 98, 111, 111, 6, 102, 97, 114},
		},
		{
			map[string]uint64{"bar": 54, "foo": 89},
			"",
			[]byte{13, 95, 6, 54, 86, 134, 1, 98, 97, 114, 3, 54, 102, 111, 111, 3, 89},
		},
		{
			ObjectA{"foo", "bar"},
			"",
			[]byte{13, 79, 6, 22, 86, 102, 65, 6, 102, 111, 111, 66, 6, 98, 97, 114},
		},
		{
			&ObjectA{"foo", "bar"}, "",
			[]byte{13, 79, 6, 22, 86, 102, 65, 6, 102, 111, 111, 66, 6, 98, 97, 114},
		},
		{
			ObjectA{"foo", "bar"},
			"",
			[]byte{13, 79, 6, 22, 86, 102, 65, 6, 102, 111, 111, 66, 6, 98, 97, 114},
		},
		{
			ObjectB{"foo", 64, false, 54.2},
			"",
			[]byte{13, 79, 6, 22, 54, 102, 66, 3, 64, 102, 111, 111, 1},
		},

		{
			map[string]chan int{"foo": make(chan int)},
			"could not encode into document: encode error: unsupported type: chan int [chan]", nil,
		},
		{
			ObjectC{make(chan int), "foo"},
			"could not encode into document: encode error: unsupported type: chan int [chan]", nil,
		},
		{nilObject(), "could not encode into document: unsupported type: nil pointer", nil},
		{nil, "could not encode into document: unsupported type", nil},
		{map[uint64]uint64{0: 56}, "could not encode into document: unsupported type: map type with non string key", nil},
	}

	for _, test := range tests {
		bytes, err := DocumentEncode(test.object)
		if test.err == "" {
			assert.Nil(t, err)
			assert.Equal(t, test.bytes, bytes)
		} else {
			assert.EqualError(t, err, test.err)
			assert.Nil(t, bytes)
		}
	}
}

func TestDocument_DecodeToDocument(t *testing.T) {
	tests := []struct {
		bytes []byte
		doc   Document
		err   string
	}{
		{
			[]byte{13, 47, 6, 54, 98, 111, 111, 6, 102, 97, 114},
			Document{"boo": []byte{6, 102, 97, 114}},
			"",
		},
		{
			[]byte{13, 95, 6, 54, 86, 134, 1, 98, 97, 114, 3, 54, 102, 111, 111, 3, 89},
			Document{"bar": []byte{3, 54}, "foo": []byte{3, 89}},
			"",
		},
		{
			[]byte{13, 79, 6, 22, 86, 102, 65, 6, 102, 111, 111, 66, 6, 98, 97, 114},
			Document{"A": []byte{6, 102, 111, 111}, "B": []byte{6, 98, 97, 114}},
			"",
		},
		{
			[]byte{13, 79, 6, 22, 54, 102, 66, 3, 64, 102, 111, 111, 1},
			Document{"B": []byte{3, 64}, "foo": []byte{1}},
			"",
		},
		{
			[]byte{14, 79, 6, 22, 54, 102, 66, 3, 64, 102, 111, 111, 1},
			Document{},
			"decode error: incompatible wire type. expected: document. got: pack",
		},
		{
			[]byte{0},
			nil, "",
		},
		{
			[]byte{13, 15},
			Document{},
			"",
		},
	}

	for _, test := range tests {
		doc := make(Document)
		err := Depolorize(&doc, test.bytes)
		assert.Equal(t, test.doc, doc)

		if test.err == "" {
			assert.Nil(t, err)
		} else {
			assert.EqualError(t, err, test.err)
		}
	}
}
