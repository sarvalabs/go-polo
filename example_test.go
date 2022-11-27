package polo

import "fmt"

// nolint:lll
func ExamplePolorize() {
	type Fruit struct {
		Name  string
		Cost  int
		Alias []string
	}

	orange := &Fruit{"orange", 300, []string{"tangerine", "mandarin"}}

	wire, _ := Polorize(orange)
	fmt.Println(wire)

	// Output:
	// [14 79 6 99 142 1 111 114 97 110 103 101 1 44 63 6 150 1 116 97 110 103 101 114 105 110 101 109 97 110 100 97 114 105 110]
}

func ExampleDepolorize() {
	type Fruit struct {
		Name  string
		Cost  int
		Alias []string
	}

	wire := []byte{
		14, 79, 6, 99, 142, 1, 111, 114, 97, 110, 103, 101, 1, 44, 63, 6, 150, 1, 116,
		97, 110, 103, 101, 114, 105, 110, 101, 109, 97, 110, 100, 97, 114, 105, 110,
	}

	object := new(Fruit)
	if err := Depolorize(object, wire); err != nil {
		panic(err)
	}

	fmt.Println(object)

	// Output:
	// &{orange 300 [tangerine mandarin]}
}

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
