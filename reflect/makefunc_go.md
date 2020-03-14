
```go
package reflect

import (
	"unsafe"
)

// makeFuncImpl是实现MakeFunc返回的函数的闭包值。
// 此类型的前三个单词必须与methodValue和runtime.reflectMethodValue保持同步。 任何更改都应反映在这三个方面。
// makeFuncImpl is the closure value implementing the function
// returned by MakeFunc.
// The first three words of this type must be kept in sync with
// methodValue and runtime.reflectMethodValue.
// Any changes should be reflected in all three.
type makeFuncImpl struct {
	code   uintptr    // 指向代码
	stack  *bitVector // ptrmap for both args and results // 参数和结果和指针映射
	argLen uintptr    // just args // 指向参数长度的指针
	ftyp   *funcType  // 函数类型
	fn     func([]Value) []Value // 函数的通用表达式
}

// MakeFunc返回给定类型的新函数，该函数包装函数fn。在调用时，该新函数将执行以下操作：
//
// - 将其参数转换为值的一部分。
// - 运行结果：= fn（args）。
// - 将结果作为值的一部分返回，每个正式结果一个。
// 实现fn可以假设参数Value切片具有typ指定的参数的数量和类型。
// 如果typ描述了可变参数函数，则最终值本身就是代表可变参数的切片，就像可变函数的主体一样。 fn返回的结果Value slice必须具有typ指定的结果的数量和类型
// Value.Call方法允许调用者根据Values调用类型化的函数；相反，MakeFunc允许调用者根据值来实现类型化的函数。
//
// 文档的“示例”部分包含如何使用MakeFunc为不同类型构建交换函数的说明


// MakeFunc returns a new function of the given Type
// that wraps the function fn. When called, that new function
// does the following:
//
//	- converts its arguments to a slice of Values.
//	- runs results := fn(args).
//	- returns the results as a slice of Values, one per formal result.
//
// The implementation fn can assume that the argument Value slice
// has the number and type of arguments given by typ.
// If typ describes a variadic function, the final Value is itself
// a slice representing the variadic arguments, as in the
// body of a variadic function. The result Value slice returned by fn
// must have the number and type of results given by typ.
//
// The Value.Call method allows the caller to invoke a typed function
// in terms of Values; in contrast, MakeFunc allows the caller to implement
// a typed function in terms of Values.
//
// The Examples section of the documentation includes an illustration
// of how to use MakeFunc to build a swap function for different types.
//
/**
 * @param tpy 函数类型定义
 * @param tpy 函数的通用定义
 * @return Value 函数值
 * @date 2020-03-14 09:01:50
 **/
func MakeFunc(typ Type, fn func(args []Value) (results []Value)) Value {
	if typ.Kind() != Func { // 如果不函数类型
		panic("reflect: call of MakeFunc with non-Func type")
	}

	t := typ.common()
	ftyp := (*funcType)(unsafe.Pointer(t)) // 转换成函数引用类型

    // 间接转到func值（虚拟）以获取实际的代码地址。
    // （Go func值是指向C函数指针的指针。https://golang.org/s/go11func。）
	// Indirect Go func value (dummy) to obtain
	// actual code address. (A Go func value is a pointer
	// to a C function pointer. https://golang.org/s/go11func.)
	dummy := makeFuncStub
	code := **(**uintptr)(unsafe.Pointer(&dummy))

    // akeFuncImpl包含供运行时使用的堆栈映射
	// makeFuncImpl contains a stack map for use by the runtime
	_, argLen, _, stack, _ := funcLayout(ftyp, nil)

	impl := &makeFuncImpl{code: code, stack: stack, argLen: argLen, ftyp: ftyp, fn: fn}

	return Value{t, unsafe.Pointer(impl), flag(Func)}
}

// makeFuncStub是一个汇编函数，是从MakeFunc返回的半代码（half code）函数。
// 它期望* callReflectFunc作为其上下文寄存器，并且其工作是调用callReflect（ctxt，frame），其中ctxt是上下文寄存器，
// 而frame是指向传入参数帧中第一个字（word）的指针。
// makeFuncStub is an assembly function that is the code half of
// the function returned from MakeFunc. It expects a *callReflectFunc
// as its context register, and its job is to invoke callReflect(ctxt, frame)
// where ctxt is the context register and frame is a pointer to the first
// word in the passed-in argument frame.
func makeFuncStub()

// //此类型的前3个字(word)必须与makeFuncImpl和runtime.reflectMethodValue保持同步。
// 任何更改都应反映在这三个方面。// TODO 这个word代表什么意思
// The first 3 words of this type must be kept in sync with
// makeFuncImpl and runtime.reflectMethodValue.
// Any changes should be reflected in all three.
type methodValue struct {
	fn     uintptr    // 指向函数的指针
	stack  *bitVector // ptrmap for both args and results // 参数和结果和指针映射
	argLen uintptr    // just args  // 指向参数的指针
	method int
	rcvr   Value
}

// makeMethodValue将v从方法值的rcvr+方法索引表示形式转换为实际的方法功能值，
// 该值实际上是带有特殊位设置的接收器值，能成为真正的方法值-包含实际功能的值。
// 就包反射的用户而言，输出在语义上等效于输入，但是真正的func表示可以由Convert
// 和Interface和Assign之类的代码处理
//
// makeMethodValue converts v from the rcvr+method index representation
// of a method value to an actual method func value, which is
// basically the receiver value with a special bit set, into a true
// func value - a value holding an actual func. The output is
// semantically equivalent to the input as far as the user of package
// reflect can tell, but the true func representation can be handled
// by code like Convert and Interface and Assign.
func makeMethodValue(op string, v Value) Value {
	if v.flag&flagMethod == 0 { // 不是方法
		panic("reflect: internal error: invalid use of makeMethodValue")
	}

    // 忽略flagMethod位，v描述接收者，而不是方法类型。
	// Ignoring the flagMethod bit, v describes the receiver, not the method type.
	fl := v.flag & (flagRO | flagAddr | flagIndir)
	fl |= flag(v.typ.Kind())
	rcvr := Value{v.typ, v.ptr, fl}

    // v.Type返回方法值的实际类型。
	// v.Type returns the actual type of the method value.
	ftyp := (*funcType)(unsafe.Pointer(v.Type().(*rtype)))

    // 间接转到func值（虚拟）以获取实际的代码地址。
    // （Go func值是指向C函数指针的指针。https://golang.org/s/go11func。）
	// Indirect Go func value (dummy) to obtain
	// actual code address. (A Go func value is a pointer
	// to a C function pointer. https://golang.org/s/go11func.)
	dummy := methodValueCall
	code := **(**uintptr)(unsafe.Pointer(&dummy))

    // methodValue包含供运行时使用的堆栈映射
	// methodValue contains a stack map for use by the runtime
	_, argLen, _, stack, _ := funcLayout(ftyp, nil)

	fv := &methodValue{
		fn:     code,
		stack:  stack,
		argLen: argLen,
		method: int(v.flag) >> flagMethodShift,
		rcvr:   rcvr,
	}

    // 如果方法不合适，则会引起恐慌。
    // 如果省略此选项，则在调用过程中仍会发生恐慌，但我们希望Interface（）
    // 和其他操作尽早失败。
	// Cause panic if method is not appropriate.
	// The panic would still happen during the call if we omit this,
	// but we want Interface() and other operations to fail early.
	methodReceiver(op, fv.rcvr, fv.method)

	return Value{&ftyp.rtype, unsafe.Pointer(fv), v.flag&flagRO | flag(Func)}
}

// methodValueCall是一个汇编函数，是makeMethodValue返回的半代码函数。
// 它期望* methodValue作为其上下文寄存器，并且它的工作是调用callMethod(ctxt,frame)，
// 其中ctxt是上下文寄存器，而frame是指向传入参数帧中第一个单词的指针。
// methodValueCall is an assembly function that is the code half of
// the function returned from makeMethodValue. It expects a *methodValue
// as its context register, and its job is to invoke callMethod(ctxt, frame)
// where ctxt is the context register and frame is a pointer to the first
// word in the passed-in argument frame.
func methodValueCall()
```