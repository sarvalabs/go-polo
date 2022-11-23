package polo

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
