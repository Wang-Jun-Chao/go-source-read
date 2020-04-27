package main

import (
	"encoding/json"
	"fmt"
	"runtime"
	"time"
)

type Garbage struct{ a int }

func notify(f *Garbage) {
	stats := &runtime.MemStats{}
	runtime.ReadMemStats(stats)

	bytes, _ := json.MarshalIndent(stats, "", "    ")
	fmt.Println(string(bytes))

	go ProduceFinalizedGarbage()
}

func ProduceFinalizedGarbage() {
	x := &Garbage{}
	runtime.SetFinalizer(x, notify)
}

func main() {
	go ProduceFinalizedGarbage()

	for {
		runtime.GC()
		time.Sleep(10 * time.Second)
	}
}
