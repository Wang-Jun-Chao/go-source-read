package main

import "fmt"

func main() {
	type P struct {
		Age  [16]int
		Male byte
	}

	var a = make(map[P]int, 16)

	for i := 0; i < 16; i++ {
		p := P{}
		p.Age[0] = i
		a[p] = i
	}
	fmt.Println(a)
}
