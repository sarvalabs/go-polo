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
