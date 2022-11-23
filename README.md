![image](./banner.png)

# go-polo
**go-polo** is the Go implementation of the POLO Serialization Format.

**POLO** stands for *Prefix Ordered Lookup Offset*. It is meant for use in projects that prioritize
deterministic serialization, minimal wire size and code safety. POLO follows a very strict specification
that is optimized for lookups and differential messaging. The full POLO Wire Specification can be found [here](https://github.com/sarvalabs/polo).

### Features
- Deterministic Serialization
- Lookup Optimized Binary Wire Format
- Partial Element Deserialization *(Coming Soon)*
- Differential Messaging *(Coming Soon)*

### Usage Examples

```go
package main

import (
	"fmt"
	
	"github.com/sarvalabs/go-polo"
)

// Fruit is an example struct
type Fruit struct {
	Name  string   
	Cost  int      
	Alias []string 
}

func main() {
	// Declare a Fruit object
	orange := &Fruit{"orange", 300, []string{"tangerine", "mandarin"}}

	// Polorize the object
	wire, err := polo.Polorize(orange)
	if err != nil {
		panic(err)
    }
	
	fmt.Println(wire)
	
	// Declare a new Fruit object
	newfruit := new(Fruit)
	// Depolorize the Fruit object
	if err := polo.Depolorize(newfruit, wire); err != nil {
		panic(err)
    }
	
	fmt.Println(newfruit)
}

// Output:
// [143 4 78 100 34 182 2 111 114 97 110 103 101 1 44 62 148 1 100 116 97 110 103 101 114 105 110 101 111 114 97 110 103 101]
// &{orange 300 [tangerine mandarin]}
```

Check out more [examples](./example_test.go) here.