```go

package sort

// Slice sorts the provided slice given the provided less function.
//
// The sort is not guaranteed to be stable. For a stable sort, use
// SliceStable.
//
// The function panics if the provided interface is not a slice.
//
// Slice根据提供的less函数对提供的slice进行排序。
//
// 不能保证排序是稳定的。 对于稳定的排序，请使用SliceStable。
//
// 如果提供的接口不是切片，则函数会出现panic情况。
func Slice(slice interface{}, less func(i, j int) bool) {
	rv := reflectValueOf(slice)
	swap := reflectSwapper(slice)
	length := rv.Len()
	quickSort_func(lessSwap{less, swap}, 0, length, maxDepth(length))
}

// SliceStable sorts the provided slice given the provided less
// function while keeping the original order of equal elements.
//
// The function panics if the provided interface is not a slice.
//
// SliceStable在给定提供较少功能的情况下对提供的切片进行排序，同时保持相等元素的原始顺序。
//
// 如果提供的接口不是切片，则函数会出现panic情况。
func SliceStable(slice interface{}, less func(i, j int) bool) {
	rv := reflectValueOf(slice)
	swap := reflectSwapper(slice)
	stable_func(lessSwap{less, swap}, rv.Len())
}

// SliceIsSorted tests whether a slice is sorted.
//
// The function panics if the provided interface is not a slice.
//
// SliceIsSorted测试切片是否已排序。
//
//如果提供的接口不是切片，则函数会出现panic情况。
func SliceIsSorted(slice interface{}, less func(i, j int) bool) bool {
	rv := reflectValueOf(slice)
	n := rv.Len()
	for i := n - 1; i > 0; i-- {
		if less(i, i-1) {
			return false
		}
	}
	return true
}
```
