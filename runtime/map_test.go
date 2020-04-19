package runtime

import "testing"

func TestSmallAlloc(t *testing.T) {
	m := make(map[int64]int64, 64)
	print(m)
}
