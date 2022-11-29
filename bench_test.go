package polo

import "testing"

type MixedObject struct {
	A string
	B int32
	C []string
	D map[string]string
	E float64
}

func BenchmarkMixed(b *testing.B) {
	object := MixedObject{
		A: "Sins & Virtues",
		B: 567822,
		C: []string{"pride", "greed", "lust", "gluttony", "envy", "wrath", "sloth"},
		D: map[string]string{"bravery": "piety", "friendship": "chastity"},
		E: 45.23,
	}

	wire, _ := Polorize(object)
	newObject := new(MixedObject)

	b.ResetTimer()

	b.Run("Polorize", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = Polorize(object)
		}
	})

	b.Run("Depolorize", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Depolorize(newObject, wire)
		}
	})
}

func BenchmarkDocument(b *testing.B) {
	object := MixedObject{
		A: "Sins & Virtues",
		B: 567822,
		C: []string{"pride", "greed", "lust", "gluttony", "envy", "wrath", "sloth"},
		D: map[string]string{"bravery": "piety", "friendship": "chastity"},
		E: 45.23,
	}

	docwire, _ := DocumentEncode(object)
	newObject := new(MixedObject)
	newDocument := make(Document)

	b.Run("Doc Encode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = DocumentEncode(object)
		}
	})

	b.Run("Doc Decode", func(b *testing.B) {
		b.Run("Struct", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = Depolorize(newObject, docwire)
			}
		})

		b.Run("Document", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = Depolorize(&newDocument, docwire)
			}
		})
	})
}
