package polo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocument_Bytes(t *testing.T) {
	tests := []struct {
		doc  Document
		wire []byte
	}{
		{Document{}, []byte{14, 15}},
		{Document{"foo": []byte{1, 0, 1, 0}}, []byte{14, 47, 6, 54, 102, 111, 111, 1, 0, 1, 0}},
		{Document{"foo": []byte{1, 0, 1, 0}, "bar": []byte{2, 1, 2, 1}}, []byte{14, 95, 6, 54, 118, 166, 1, 98, 97, 114, 2, 1, 2, 1, 102, 111, 111, 1, 0, 1, 0}},
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
	assert.EqualError(t, err, "document value could not be encoded for key 'far': unsupported type: chan int [chan]")
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
