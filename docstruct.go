package polo

import (
	"fmt"
	"reflect"
	"sync"
)

// documentStructType is the reflect.Type of the DocumentStruct interface
var documentStructType = reflect.TypeOf((*DocumentStruct)(nil)).Elem()

// docField is an encoded field of a struct in a document: the index of the field and its document key
type docField struct {
	index int
	key   string
}

// structInfo is the document information of a struct type
type structInfo struct {
	// document is set if the type implements DocumentStruct (with a value or a pointer receiver)
	document bool
	// fields are the exported fields that are not skipped, in declaration order
	fields []docField
	// err is the error of the type as a document: a key that is not valid UTF-8, or a key that two fields have
	err error
}

// structInfoCache holds the structInfo of each struct type that was encoded or decoded (reflect.Type -> *structInfo)
var structInfoCache sync.Map

// getStructInfo returns the document information of a struct type. The result is computed once for each type.
func getStructInfo(t reflect.Type) *structInfo {
	if info, ok := structInfoCache.Load(t); ok {
		return info.(*structInfo) //nolint:forcetypeassert // the cache holds only *structInfo
	}

	info, _ := structInfoCache.LoadOrStore(t, newStructInfo(t))

	return info.(*structInfo) //nolint:forcetypeassert // the cache holds only *structInfo
}

// newStructInfo computes the document information of a struct type
func newStructInfo(t reflect.Type) *structInfo {
	info := &structInfo{
		document: t.Implements(documentStructType) || reflect.PointerTo(t).Implements(documentStructType),
		fields:   make([]docField, 0, t.NumField()),
	}

	keys := make(map[string]struct{}, t.NumField())

	for index := 0; index < t.NumField(); index++ {
		field := t.Field(index)

		// Skip the field if it is not exported or if it
		// is manually tagged to be skipped with a '-' tag
		tag := field.Tag.Get("polo")
		if !field.IsExported() || tag == "-" {
			continue
		}

		// The document key is the field name, or the name in the polo tag
		key := field.Name
		if tag != "" {
			key = tag
		}

		if info.err == nil {
			// The document key must be valid UTF-8
			if !validString(key) {
				info.err = invalidUTF8ValueError()
			}

			// Each field must have its own key. Otherwise, the document has one value for two fields
			if _, exists := keys[key]; exists {
				info.err = IncompatibleValueError{msg: fmt.Sprintf("duplicate document key '%v' for struct %v", key, t)}
			}
		}

		keys[key] = struct{}{}
		info.fields = append(info.fields, docField{index: index, key: key})
	}

	return info
}

// unknownKey returns the smallest key of a Document that is not the key of a field
func (info *structInfo) unknownKey(doc Document) string {
	known := make(map[string]struct{}, len(info.fields))
	for _, field := range info.fields {
		known[field.key] = struct{}{}
	}

	var (
		first string
		found bool
	)

	for key := range doc {
		if _, ok := known[key]; ok {
			continue
		}

		if !found || key < first {
			first, found = key, true
		}
	}

	return first
}
