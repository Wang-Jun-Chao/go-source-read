package main

import "fmt"

func main() {
	type P struct {
		Age [16]int
	}

	var a = make(map[P]int, 17)

	a[P{}] = 9999999

	for i := 0; i < 16; i++ {
		p := P{}
		p.Age[0] = i
		a[p] = i
	}
	fmt.Println(a)
}
