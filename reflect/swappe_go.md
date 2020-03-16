```go
package reflect

import "unsafe"

// Swapper返回一个函数，该函数交换a切片中的元素。
// Swapper returns a function that swaps the elements in the provided
// slice.
//
// 如果提供的接口不是切片，则Swapper会出现panic。
// Swapper panics if the provided interface is not a slice.
/**
 * @param slice 切片
 * @return func(i, j int) 交换函数
 * @date 2020-03-15 10:20:57
 **/
func Swapper(slice interface{}) func(i, j int) {
	v := ValueOf(slice) // ValueOf返回一个新的Value，初始化为存储在接口i中的具体值。 ValueOf（nil）返回零值。
	if v.Kind() != Slice { // 类型不是切片，panic
		panic(&ValueError{Method: "Swapper", Kind: v.Kind()})
	}
	// 大小为0和1的片的快速路径。无须交换。
	// Fast path for slices of size 0 and 1. Nothing to swap.
	switch v.Len() {
	case 0:
		return func(i, j int) { panic("reflect: slice index out of range") }
	case 1:
		return func(i, j int) {
			if i != 0 || j != 0 {
				panic("reflect: slice index out of range")
			}
		}
	}

	typ := v.Type().Elem().(*rtype) // 将切片转成rtype指针类型
	size := typ.Size() // TODO rtype.size 代表当前元素的值有多少个字节
	hasPtr := typ.ptrdata != 0 // 判断是否有指针

    // 一些通用和常见的情况，不需要内存移动
	// Some common & small cases, without using memmove:
	if hasPtr {
	    // 真的是切片类型指针
		if size == ptrSize { // ptrSize = 8
			ps := *(*[]unsafe.Pointer)(v.ptr)
			// Question: 函数使用了ps数据，如果此函数被外部使用会出现什么情况？
			// Answer: 函数内部保持了对原数据的引用，可以理解成此函数内部的ps是一个新变量。
			// 当交换的时候，会使用此变量和索引一起实现功能
			return func(i, j int) { ps[i], ps[j] = ps[j], ps[i] }
		}
		// 如果是字符串类型
		if typ.Kind() == String {
		    // Value.ptr 指针值数据；如果设置了flagIndir，则为数据的指针。
		    // 当设置flagIndir或typ.pointers（）为true时有效。
			ss := *(*[]string)(v.ptr)
			return func(i, j int) { ss[i], ss[j] = ss[j], ss[i] }
		}
	} else {
	    // Question: 为什么这里可以判断出来是整数数组
		switch size {
		case 8:
			is := *(*[]int64)(v.ptr)
			return func(i, j int) { is[i], is[j] = is[j], is[i] }
		case 4:
			is := *(*[]int32)(v.ptr)
			return func(i, j int) { is[i], is[j] = is[j], is[i] }
		case 2:
			is := *(*[]int16)(v.ptr)
			return func(i, j int) { is[i], is[j] = is[j], is[i] }
		case 1:
			is := *(*[]int8)(v.ptr)
			return func(i, j int) { is[i], is[j] = is[j], is[i] }
		}
	}

    // 转换成切片头结构
	s := (*sliceHeader)(v.ptr)
	tmp := unsafe_New(typ) // swap scratch space // 交换暂存空间

	return func(i, j int) {
	    // 越界
		if uint(i) >= uint(s.Len) || uint(j) >= uint(s.Len) {
			panic("reflect: slice index out of range")
		}
        // 取元素值
		val1 := arrayAt(s.Data, i, size, "i < s.Len")
		val2 := arrayAt(s.Data, j, size, "j < s.Len")
		// 移动元素
		typedmemmove(typ, tmp, val1)
		typedmemmove(typ, val1, val2)
		typedmemmove(typ, val2, tmp)
	}
}
```