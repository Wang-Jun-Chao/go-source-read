```go

/
/*
    unsafe 包含有关于Go程序类型安全的所有操作。
	Package unsafe contains operations that step around the type safety of Go programs.

    导入不安全的软件包可能是不可移植的，并且不受Go 1兼容性准则的保护。
	Packages that import unsafe may be non-portable and are not protected by the
	Go 1 compatibility guidelines.
*/
package unsafe

// ArbitraryType此处仅出于文档目的，实际上并不是不安全软件包的一部分。 它表示任意Go表达式的类型。
// ArbitraryType is here for the purposes of documentation only and is not actually
// part of the unsafe package. It represents the type of an arbitrary Go expression.
type ArbitraryType int

// Pointer 代表一个指向任意类型的指针。
// 有四种特殊的操作可用于类型指针而不能用于其它类型。
//	1) 任意类型的指针值均可转换为 Pointer。
//	2) Pointer 均可转换为任意类型的指针值。
//	3) uintptr 均可转换为 Pointer。
//	4) Pointer 均可转换为 uintptr。
// 因此 Pointer 允许程序击溃类型系统并读写任意内存。它应当被用得非常小心。
//
// 以下涉及Pointer的模式是有效的。 不使用这些模式的代码今天可能无效，或者将来可能无效。 甚至下面的有效模式也带有重要的警告。
//
// 运行“ go vet”可以帮助查找不符合这些模式的Pointer用法，但是对“go vet”的静默模式并不能保证代码有效。
// 
// （1）将* T1转换为指向* T2的指针。
// 如果T2不大于T1，并且两个共享相同的内存布局，则此转换允许将一种类型的数据重新解释为另一种类型的数据。 一个示例是math.Float64bits的实现：
//	func Float64bits(f float64) uint64 {
//		return *(*uint64)(unsafe.Pointer(&f))
//	}
//
// （2）将Pointer转换为uintptr（但不返回给Pointer）。
// 
// 将Pointer转换为uintptr会生成指向整数的指针的内存地址。这种uintptr的通常用法是打印它。
// 将uintptr转换回Pointer通常是无效的。
// uintptr是整数，不是引用。将Pointer转换为uintptr会创建一个没有指针语义的整数值。
// 即使uintptr拥有某个对象的地址，垃圾回收器也不会在对象移动时更新该uintptr的值，也不会使uintptr阻止回收该对象。
// 其余模式列举从uintptr到Pointer的唯一有效转换。
//
//（3）使用算术将指针转换为uintptr并返回。
//
// 如果p指向已分配的对象，则可以通过转换为uintptr，添加偏移量并将其转换回Pointer的方式将其推进对象。
// p = unsafe.Pointer(uintptr(p) + offset)
// 此模式最常见的用法是访问结构中的元素，或者数组中的字段：
// // 等效于 f := unsafe.Pointer(&s.f)
// f := unsafe.Pointer(uintptr(unsafe.Pointer(&s)) + unsafe.Offsetof(s.f))
// // 等效于 e := unsafe.Pointer(&x[i])
// e := unsafe.Pointer(uintptr(unsafe.Pointer(&x[0])) + i*unsafe.Sizeof(x[0]))
// 以这种方式从指针添加和减去偏移量都是有效的。通常使用＆^舍入指针（通常用于对齐）也是有效的。在所有情况下，结果都必须继续指向原始分配的对象。
// 与C语言不同，将指针好超出其原始分配的末尾是无效的：
// 
// // INVALID：结束指针在分配的空间之外。
// var s thing
// end = unsafe.Pointer(uintptr(unsafe.Pointer(&s)) + unsafe.Sizeof(s))
// 
// // INVALID：结束指针在分配的空间之外。
// b := make([]byte, n)
// end = unsafe.Pointer(uintptr(unsafe.Pointer(&b[0])) + uintptr(n))
//
// 请注意，两个转换必须出现在同一表达式中，并且它们之间只有中间的算术运算：
// //INVALID：uintptr在转换回Pointer之前不能存储在变量中。
// u := uintptr(p)
// p = unsafe.Pointer(u+offset)
//
// 注意，指针必须指向已分配的对象，因此它不能为nil。
// // 无效：nil指针的转换
// u := unsafe.Pointer(nil)
// p := unsafe.Pointer(uintptr(u) + offset)
//
//（4）调用syscall.Syscall时将指针转换为uintptr。
//
// 软件包syscall中的Syscall函数将其uintptr参数直接传递给操作系统，然后，操作系统可以根据调用的详细信息将其中一些参数重新解释为指针。也就是说，系统调用实现正在将某些参数从uintptr隐式转换回指针。
// 如果必须将指针参数转换为uintptr用作参数，则该转换必须出现在调用表达式本身中：
// syscall.Syscall(SYS_READ, uintptr(fd), uintptr(unsafe.Pointer(p)), uintptr(n))
// 编译器通过安排引用的已分配对象，处理在汇编中实现的函数的调用的参数列表中转换为uintptr的Pointer，
// 如果有的话，对象会保留，并且直到调用完成才移动，即使仅从类型来看，在调用过程中似乎不再需要该对象。 // TODO 英文本身句法复杂，翻译不准备
// 
// 为了使编译器能够识别这种模式，转换必须出现在参数列表中：
// //无效：在系统调用期间隐式转换回Pointer之前，不能将uintptr存储在变量中。
// u := uintptr(unsafe.Pointer(p))
// syscall.Syscall(SYS_READ, uintptr(fd), u, uintptr(n))
//
// （5）将reflect.Value.Pointer或reflect.Value.UnsafeAddr的结果从uintptr转换为Pointer。
// 包reflect的Value结构的名为Pointer和UnsafeAddr方法返回uintptr类型而不是不安全类型。Pointer可防止调用者将结果更改为任意类型，而无需先导入“不安全”。但是，这意味着结果很脆弱，必须在调用后立即使用相同的表达式将其转换为Pointer：
// p := (*int)(unsafe.Pointer(reflect.ValueOf(new(int)).Pointer()))
//
// 与上述情况一样，在转换之前存储结果无效：
// //无效：uintptr在转换回Pointer之前不能存储在变量中。
// u := reflect.ValueOf(new(int)).Pointer()
// p := (*int)(unsafe.Pointer(u))
//
// （6）将reflect.SliceHeader或reflect.StringHeader数据字段与指针进行转换。
// 与前面的情况一样，反射数据结构SliceHeader和StringHeader将字段Data声明为uintptr，以防止调用者将结果更改为任意类型，而无需首先导入“ unsafe”。但是，这意味着SliceHeader和StringHeader仅在解释实际切片或字符串值的内容时才有效。
// var s string
// hdr := (*reflect.StringHeader)(unsafe.Pointer(&s)) // 情况 1
// hdr.Data = uintptr(unsafe.Pointer(p))              // 情况 6 (当前情况)
// hdr.Len = n
//
// 在这种用法中，hdr.Data实际上是在字符串header中引用基础指针的替代方法，而不是uintptr变量本身。
// 通常，reflect.SliceHeader和reflect.StringHeader只能用作指向实际切片或字符串的*reflect.SliceHeader和*reflect.StringHeader，而不能用作纯结构。程序不应声明或分配这些结构类型的变量。
// 
// // INVALID：直接声明的标头将不保存Data作为引用。
// var hdr reflect.StringHeader
// hdr.Data = uintptr(unsafe.Pointer(p))
// hdr.Len = n
// s := *(*string)(unsafe.Pointer(&hdr)) // p可能已经丢失

// Pointer represents a pointer to an arbitrary type. There are four special operations
// available for type Pointer that are not available for other types:
//	- A pointer value of any type can be converted to a Pointer.
//	- A Pointer can be converted to a pointer value of any type.
//	- A uintptr can be converted to a Pointer.
//	- A Pointer can be converted to a uintptr.
// Pointer therefore allows a program to defeat the type system and read and write
// arbitrary memory. It should be used with extreme care.
//
// The following patterns involving Pointer are valid.
// Code not using these patterns is likely to be invalid today
// or to become invalid in the future.
// Even the valid patterns below come with important caveats.
//
// Running "go vet" can help find uses of Pointer that do not conform to these patterns,
// but silence from "go vet" is not a guarantee that the code is valid.
//
// (1) Conversion of a *T1 to Pointer to *T2.
//
// Provided that T2 is no larger than T1 and that the two share an equivalent
// memory layout, this conversion allows reinterpreting data of one type as
// data of another type. An example is the implementation of
// math.Float64bits:
//
//	func Float64bits(f float64) uint64 {
//		return *(*uint64)(unsafe.Pointer(&f))
//	}
//
// (2) Conversion of a Pointer to a uintptr (but not back to Pointer).
//
// Converting a Pointer to a uintptr produces the memory address of the value
// pointed at, as an integer. The usual use for such a uintptr is to print it.
//
// Conversion of a uintptr back to Pointer is not valid in general.
//
// A uintptr is an integer, not a reference.
// Converting a Pointer to a uintptr creates an integer value
// with no pointer semantics.
// Even if a uintptr holds the address of some object,
// the garbage collector will not update that uintptr's value
// if the object moves, nor will that uintptr keep the object
// from being reclaimed.
//
// The remaining patterns enumerate the only valid conversions
// from uintptr to Pointer.
//
// (3) Conversion of a Pointer to a uintptr and back, with arithmetic.
//
// If p points into an allocated object, it can be advanced through the object
// by conversion to uintptr, addition of an offset, and conversion back to Pointer.
//
//	p = unsafe.Pointer(uintptr(p) + offset)
//
// The most common use of this pattern is to access fields in a struct
// or elements of an array:
//
//	// equivalent to f := unsafe.Pointer(&s.f)
//	f := unsafe.Pointer(uintptr(unsafe.Pointer(&s)) + unsafe.Offsetof(s.f))
//
//	// equivalent to e := unsafe.Pointer(&x[i])
//	e := unsafe.Pointer(uintptr(unsafe.Pointer(&x[0])) + i*unsafe.Sizeof(x[0]))
//
// It is valid both to add and to subtract offsets from a pointer in this way.
// It is also valid to use &^ to round pointers, usually for alignment.
// In all cases, the result must continue to point into the original allocated object.
//
// Unlike in C, it is not valid to advance a pointer just beyond the end of
// its original allocation:
//
//	// INVALID: end points outside allocated space.
//	var s thing
//	end = unsafe.Pointer(uintptr(unsafe.Pointer(&s)) + unsafe.Sizeof(s))
//
//	// INVALID: end points outside allocated space.
//	b := make([]byte, n)
//	end = unsafe.Pointer(uintptr(unsafe.Pointer(&b[0])) + uintptr(n))
//
// Note that both conversions must appear in the same expression, with only
// the intervening arithmetic between them:
//
//	// INVALID: uintptr cannot be stored in variable
//	// before conversion back to Pointer.
//	u := uintptr(p)
//	p = unsafe.Pointer(u + offset)
//
// Note that the pointer must point into an allocated object, so it may not be nil.
//
//	// INVALID: conversion of nil pointer
//	u := unsafe.Pointer(nil)
//	p := unsafe.Pointer(uintptr(u) + offset)
//
// (4) Conversion of a Pointer to a uintptr when calling syscall.Syscall.
//
// The Syscall functions in package syscall pass their uintptr arguments directly
// to the operating system, which then may, depending on the details of the call,
// reinterpret some of them as pointers.
// That is, the system call implementation is implicitly converting certain arguments
// back from uintptr to pointer.
//
// If a pointer argument must be converted to uintptr for use as an argument,
// that conversion must appear in the call expression itself:
//
//	syscall.Syscall(SYS_READ, uintptr(fd), uintptr(unsafe.Pointer(p)), uintptr(n))
//
// The compiler handles a Pointer converted to a uintptr in the argument list of
// a call to a function implemented in assembly by arranging that the referenced
// allocated object, if any, is retained and not moved until the call completes,
// even though from the types alone it would appear that the object is no longer
// needed during the call.
//
// For the compiler to recognize this pattern,
// the conversion must appear in the argument list:
//
//	// INVALID: uintptr cannot be stored in variable
//	// before implicit conversion back to Pointer during system call.
//	u := uintptr(unsafe.Pointer(p))
//	syscall.Syscall(SYS_READ, uintptr(fd), u, uintptr(n))
//
// (5) Conversion of the result of reflect.Value.Pointer or reflect.Value.UnsafeAddr
// from uintptr to Pointer.
//
// Package reflect's Value methods named Pointer and UnsafeAddr return type uintptr
// instead of unsafe.Pointer to keep callers from changing the result to an arbitrary
// type without first importing "unsafe". However, this means that the result is
// fragile and must be converted to Pointer immediately after making the call,
// in the same expression:
//
//	p := (*int)(unsafe.Pointer(reflect.ValueOf(new(int)).Pointer()))
//
// As in the cases above, it is invalid to store the result before the conversion:
//
//	// INVALID: uintptr cannot be stored in variable
//	// before conversion back to Pointer.
//	u := reflect.ValueOf(new(int)).Pointer()
//	p := (*int)(unsafe.Pointer(u))
//
// (6) Conversion of a reflect.SliceHeader or reflect.StringHeader Data field to or from Pointer.
//
// As in the previous case, the reflect data structures SliceHeader and StringHeader
// declare the field Data as a uintptr to keep callers from changing the result to
// an arbitrary type without first importing "unsafe". However, this means that
// SliceHeader and StringHeader are only valid when interpreting the content
// of an actual slice or string value.
//
//	var s string
//	hdr := (*reflect.StringHeader)(unsafe.Pointer(&s)) // case 1
//	hdr.Data = uintptr(unsafe.Pointer(p))              // case 6 (this case)
//	hdr.Len = n
//
// In this usage hdr.Data is really an alternate way to refer to the underlying
// pointer in the string header, not a uintptr variable itself.
//
// In general, reflect.SliceHeader and reflect.StringHeader should be used
// only as *reflect.SliceHeader and *reflect.StringHeader pointing at actual
// slices or strings, never as plain structs.
// A program should not declare or allocate variables of these struct types.
//
//	// INVALID: a directly-declared header will not hold Data as a reference.
//	var hdr reflect.StringHeader
//	hdr.Data = uintptr(unsafe.Pointer(p))
//	hdr.Len = n
//	s := *(*string)(unsafe.Pointer(&hdr)) // p possibly already lost
//
type Pointer *ArbitraryType

// Sizeof接受任何类型的表达式x并返回假设变量v的字节大小，就好像v是通过var v = x声明的一样。
该大小不包括x可能引用的任何内存。
例如，如果x是切片，则Sizeof返回切片描述符的大小，而不是该切片所引用的内存的大小。 Sizeof的返回值是Go常数。
// Sizeof takes an expression x of any type and returns the size in bytes
// of a hypothetical variable v as if v was declared via var v = x.
// The size does not include any memory possibly referenced by x.
// For instance, if x is a slice, Sizeof returns the size of the slice
// descriptor, not the size of the memory referenced by the slice.
// The return value of Sizeof is a Go constant.
func Sizeof(x ArbitraryType) uintptr

// Offsetof返回x表示的结构内的字段偏移量，该偏移量的格式必须为structValue.field。 换句话说，它返回结构开始与字段开始之间的字节数。 Offsetof的返回值为Go常数。
// Offsetof returns the offset within the struct of the field represented by x,
// which must be of the form structValue.field. In other words, it returns the
// number of bytes between the start of the struct and the start of the field.
// The return value of Offsetof is a Go constant.
func Offsetof(x ArbitraryType) uintptr

// Alignof 接受一个任意类型的表达式 x 并返回假定的变量 v 的对齐，这里的 v 可看做通过
// var v = x 声明的变量。它是最大值 m 使其满足 v 的地址取模 m 为零。
// 它与reflect.TypeOf（x）.Align（）返回的值相同。 作为一种特殊情况，如果变量s是结构类型，而f是该结构中的字段，则Alignof（s.f）将返回结构中该类型字段的所需对齐方式。 这种情况与reflect.TypeOf（s.f）.FieldAlign（）返回的值相同。
Alignof的返回值是Go常数。
// Alignof takes an expression x of any type and returns the required alignment
// of a hypothetical variable v as if v was declared via var v = x.
// It is the largest value m such that the address of v is always zero mod m.
// It is the same as the value returned by reflect.TypeOf(x).Align().
// As a special case, if a variable s is of struct type and f is a field
// within that struct, then Alignof(s.f) will return the required alignment
// of a field of that type within a struct. This case is the same as the
// value returned by reflect.TypeOf(s.f).FieldAlign().
// The return value of Alignof is a Go constant.
func Alignof(x ArbitraryType) uintptr
```

