value文件主要提供值的一些调用方法

```go
package reflect

import (
	"math"
	"runtime"
	"unsafe"
)

const ptrSize = 4 << (^uintptr(0) >> 63) // unsafe.Sizeof(uintptr(0)) but an ideal const

// Value is the reflection interface to a Go value.
//
// Not all methods apply to all kinds of values. Restrictions,
// if any, are noted in the documentation for each method.
// Use the Kind method to find out the kind of value before
// calling kind-specific methods. Calling a method
// inappropriate to the kind of type causes a run time panic.
//
// The zero Value represents no value.
// Its IsValid method returns false, its Kind method returns Invalid,
// its String method returns "<invalid Value>", and all other methods panic.
// Most functions and methods never return an invalid value.
// If one does, its documentation states the conditions explicitly.
//
// A Value can be used concurrently by multiple goroutines provided that
// the underlying Go value can be used concurrently for the equivalent
// direct operations.
//
// To compare two Values, compare the results of the Interface method.
// Using == on two Values does not compare the underlying values
// they represent.
/**
 *  Value是Go值的反射接口。
 *
 * 并非所有方法都适用于所有类型的值。 在每种方法的文档中都注明了限制（如果有）。
 * 在调用特定于种类的方法之前，使用Kind方法找出值的种类。 调用不适合该类型的方法会导致运行时恐慌。
 *
 * 零值表示无值。
 * 它的IsValid方法返回false，其Kind方法返回Invalid，其String方法返回"<invalid Value>"，所有其他方法均会出现恐慌情况。
 * 大多数函数和方法从不返回无效值。
 * 如果是，则其文档会明确说明条件。
 *
 * 一个值可以由多个goroutine并发使用，前提是可以将基础Go值同时用于等效的直接操作。
 *
 * 要比较两个值，请比较Interface方法的结果。
 * 在两个值上使用==不会比较它们表示的基础值。
 */
type Value struct {
	// typ holds the type of the value represented by a Value.
	/**
	 * typ包含由值表示的值的类型。
	 */
	typ *rtype

	// Pointer-valued data or, if flagIndir is set, pointer to data.
	// Valid when either flagIndir is set or typ.pointers() is true.
	/**
	 * 指针值的数据；如果设置了flagIndir，则为数据的指针。
     * 在设置flagIndir或typ.pointers()为true时有效。
	 */
	ptr unsafe.Pointer

	// flag holds metadata about the value.
	// The lowest bits are flag bits:
	//	- flagStickyRO: obtained via unexported not embedded field, so read-only
	//	- flagEmbedRO: obtained via unexported embedded field, so read-only
	//	- flagIndir: val holds a pointer to the data
	//	- flagAddr: v.CanAddr is true (implies flagIndir)
	//	- flagMethod: v is a method value.
	// The next five bits give the Kind of the value.
	// This repeats typ.Kind() except for method values.
	// The remaining 23+ bits give a method number for method values.
	// If flag.kind() != Func, code can assume that flagMethod is unset.
	// If ifaceIndir(typ), code can assume that flagIndir is set.
	/**
	 * 标志保存有关该值的元数据。
     * 最低位是标志位：
     * -flagStickyRO：通过未导出的未嵌入字段获取，因此为只读
     * -flagEmbedRO：通过未导出的嵌入式字段获取，因此为只读
     * -flagIndir：val保存指向数据的指针
     * -flagAddr：v.CanAddr为true（暗示flagIndir）
     * -flagMethod：v是方法值。
     * 接下来的五位给出值的种类。
     * 重复typ.Kind()中的值，方法值除外。
     * 其余的23+位给出方法值的方法编号。
     * 如果flag.kind() != Func，则代码可以假定flagMethod未设置。
     * 如果是ifaceIndir(typ)，则代码可以假定设置了flagIndir。
	 */
	flag

	// A method value represents a curried method invocation
	// like r.Read for some receiver r. The typ+val+flag bits describe
	// the receiver r, but the flag's Kind bits say Func (methods are
	// functions), and the top bits of the flag give the method number
	// in r's type's method table.
	/**
	 * 方法值表示类似r.Read的经过当前（curried）的方法调用，用于某些接收方r。
     * typ + val + flag位描述接收方r，但标志的Kind位表示Func（方法是函数），
     * 并且标志的高位给出r的类型的方法表中的方法编号。
	 */
}

type flag uintptr

const (
	flagKindWidth        = 5 // there are 27 kinds // 种类的宽度，具体种类在type.go文件中
	flagKindMask    flag = 1<<flagKindWidth - 1 // 数据类型的掩码
	flagStickyRO    flag = 1 << 5 // 通过未导出的未嵌入字段获取，因此为只读
	flagEmbedRO     flag = 1 << 6 // 通过未导出的嵌入式字段获取，因此为只读
	flagIndir       flag = 1 << 7 // val保存指向数据的指针，val指的是Value中的ptr属性
	flagAddr        flag = 1 << 8 // v.CanAddr为true（暗示flagIndir），v指Value的实例
	flagMethod      flag = 1 << 9 // v是方法值，v指Value的实例
	flagMethodShift      = 10
	flagRO          flag = flagStickyRO | flagEmbedRO // 表示是否只读
)

/**
 * 求数值类型
 * @return 数值类型
 **/
func (f flag) kind() Kind {
	return Kind(f & flagKindMask)
}

/**
 * 获取只读标记
 */
func (f flag) ro() flag {
	if f&flagRO != 0 {
		return flagStickyRO
	}
	return 0
}

// pointer returns the underlying pointer represented by v.
// v.Kind() must be Ptr, Map, Chan, Func, or UnsafePointer
/**
 * 指针返回v表示的底层指针。
 * v.Kind()必须是Ptr，Map，Chan，Func或UnsafePointer
 * @return
 **/
func (v Value) pointer() unsafe.Pointer {
	if v.typ.size != ptrSize || !v.typ.pointers() {
		panic("can't call pointer on a non-pointer Value")
	}
	if v.flag&flagIndir != 0 {
		return *(*unsafe.Pointer)(v.ptr)
	}
	return v.ptr
}

// packEface converts v to the empty interface.
/**
 * packEface将v转换为空接口。
 * @return 空接口
 **/
func packEface(v Value) interface{} {
	t := v.typ
	var i interface{}
	e := (*emptyInterface)(unsafe.Pointer(&i))
	// First, fill in the data portion of the interface.
	// 首先，填写接口的数据部分。
	switch {
	case ifaceIndir(t): // ifaceIndir报告t是否间接存储在接口值中。
		if v.flag&flagIndir == 0 {
			panic("bad indir")
		}
		// Value is indirect, and so is the interface we're making.
		// 值是间接的，我们正在建立的接口也是间接的。
		ptr := v.ptr
		if v.flag&flagAddr != 0 {
			// TODO: pass safe boolean from valueInterface so
			// we don't need to copy if safe==true?
			// TODO：从valueInterface传递安全布尔值，因此如果safe == true，我们不需要复制吗？
			c := unsafe_New(t) // 在runtime包中实验，用于创建一个Pointer类型
			typedmemmove(t, c, ptr) // typedmemmove将类型t的值从prt复制到c。
			ptr = c
		}
		e.word = ptr
	case v.flag&flagIndir != 0:
		// Value is indirect, but interface is direct. We need
		// to load the data at v.ptr into the interface data word.
		// 值是间接的，但接口是直接的。 我们需要将v.ptr处的数据加载到接口数据字中。
		e.word = *(*unsafe.Pointer)(v.ptr)
	default:
		// Value is direct, and so is the interface.
		// 值是直接的，接口也是直接的。
		e.word = v.ptr
	}
	// Now, fill in the type portion. We're very careful here not
	// to have any operation between the e.word and e.typ assignments
	// that would let the garbage collector observe the partially-built
	// interface value.
	// 现在，填写类型部分。 在这里，我们非常小心，不要在e.word和e.typ分配之间进行任何操作，
	// 以免垃圾回收器观察部分构建的接口值。
	e.typ = t
	return i
}

// unpackEface converts the empty interface i to a Value.
/**
 * unpackEface将空接口i转换为Value。
 * @param 接口
 * @return Value类型
 **/
func unpackEface(i interface{}) Value {
	e := (*emptyInterface)(unsafe.Pointer(&i))
	// NOTE: don't read e.word until we know whether it is really a pointer or not.
	// 注意：在我们知道e.word是否真的是指针之前，不要读它。
	t := e.typ
	if t == nil { // i对应的类型为空，则不需要设置相关值
		return Value{}
	}
	f := flag(t.Kind())
	if ifaceIndir(t) { // 设置接口值标记
		f |= flagIndir
	}
	return Value{t, e.word, f}
}

// A ValueError occurs when a Value method is invoked on
// a Value that does not support it. Such cases are documented
// in the description of each method.
/**
 * 在不支持Value的Value方法上调用Value方法时，发生ValueError。
 * 在每种方法的说明中都记录了这种情况。
 */
type ValueError struct {
	Method string
	Kind   Kind
}

func (e *ValueError) Error() string {
	if e.Kind == 0 {
		return "reflect: call of " + e.Method + " on zero Value"
	}
	return "reflect: call of " + e.Method + " on " + e.Kind.String() + " Value"
}

// methodName returns the name of the calling method,
// assumed to be two stack frames above.
/**
 * methodName返回调用方法的名称，假定为上面有两个堆栈帧。
 */
func methodName() string {
    /**
     * func Caller(skip int) (pc uintptr, file string, line int, ok bool)
     * Caller报告有关调用goroutine堆栈上函数调用的文件和行号信息。
     * 参数skip是要提升的堆栈帧数，其中0标识Caller的调用者。
     *（由于历史原因，在Caller和Callers之间，跳过的含义有所不同。）
     * 返回值报告相应调用文件中的程序计数器，文件名和行号。 如果该信息不可能恢复，
     * 则ok布尔值为False。
     */
	pc, _, _, _ := runtime.Caller(2) // Question: 为什么是2？
	/**
	 * 给定程序计数器地址，否则为nil。
     * 如果pc由于内联而表示多个函数，它将返回一个* Func描述最内部的函数，
     * 但带有最外部的函数的条目。
	 */
	f := runtime.FuncForPC(pc)
	if f == nil {
		return "unknown method"
	}
	return f.Name()
}

// emptyInterface is the header for an interface{} value.
/**
 * emptyInterface是interface{}值的头部。
 */
type emptyInterface struct {
	typ  *rtype
	word unsafe.Pointer
}

// nonEmptyInterface is the header for an interface value with methods.
/**
 * nonEmptyInterface是带有方法的接口值的头部。
 */
type nonEmptyInterface struct {
	// see ../runtime/iface.go:/Itab
	itab *struct {
		ityp *rtype // static interface type // 静态接口类型
		typ  *rtype // dynamic concrete type // 动态创建类型
		hash uint32 // copy of typ.hash      // hash值
		_    [4]byte                         // Question: 用于对齐？
		fun  [100000]unsafe.Pointer // method table  // Question: 最多保存10W个方法？
	}
	word unsafe.Pointer
}

// mustBe panics if f's kind is not expected.
// Making this a method on flag instead of on Value
// (and embedding flag in Value) means that we can write
// the very clear v.mustBe(Bool) and have it compile into
// v.flag.mustBe(Bool), which will only bother to copy the
// single important word for the receiver.
/**
 * 如果f的种类不是期望类型，则必须惊慌。
 * 将此方法设置为基于标志而不是基于Value的方法（并在Value中嵌入flag标志）
 * 意味着我们可以编写非常清晰的v.mustBe（Bool）并将其编译为v.flag.mustBe（Bool），
 * 唯一麻烦是只需要为接收者复制一个重要的单词。
 */
func (f flag) mustBe(expected Kind) {
	// TODO(mvdan): use f.kind() again once mid-stack inlining gets better
	// TODO(mvdan): mid-stack的内联变得更好后，再次使用f.kind()
    // Question: mid-stack是什么？
	if Kind(f&flagKindMask) != expected {
		panic(&ValueError{methodName(), f.kind()})
	}
}

// mustBeExported panics if f records that the value was obtained using
// an unexported field.
/**
 * 如果f记录了使用未导出字段获得的值，则必须惊慌。
 */
func (f flag) mustBeExported() {
    // Enhance: mustBeExported和mustBeExportedSlow两个方法一样，
	if f == 0 || f&flagRO != 0 {
		f.mustBeExportedSlow()
	}
}

func (f flag) mustBeExportedSlow() {
	if f == 0 {
		panic(&ValueError{methodName(), Invalid})
	}
	if f&flagRO != 0 {
		panic("reflect: " + methodName() + " using value obtained using unexported field")
	}
}

// mustBeAssignable panics if f records that the value is not assignable,
// which is to say that either it was obtained using an unexported field
// or it is not addressable.
/**
 * 如果f记录该值不可分配，则mustBeAssignable会发生panic，
 * 这意味着它是使用未导出的字段获取的，或者它是不可寻址的。
 */
func (f flag) mustBeAssignable() {
	if f&flagRO != 0 || f&flagAddr == 0 {
		f.mustBeAssignableSlow()
	}
}

func (f flag) mustBeAssignableSlow() {
	if f == 0 {
		panic(&ValueError{methodName(), Invalid})
	}
	// Assignable if addressable and not read-only.
	// 如果可寻址且不是只读，则可分配。
	if f&flagRO != 0 {
		panic("reflect: " + methodName() + " using value obtained using unexported field")
	}
	if f&flagAddr == 0 {
		panic("reflect: " + methodName() + " using unaddressable value")
	}
}

// Addr returns a pointer value representing the address of v.
// It panics if CanAddr() returns false.
// Addr is typically used to obtain a pointer to a struct field
// or slice element in order to call a method that requires a
// pointer receiver.
/**
 * Addr返回表示v地址的指针值。
 * 如果CanAddr（）返回false，则会感到恐慌。
 * Addr通常用于获取指向struct字段或slice元素的指针，以便调用需要指针接收器的方法。
 */
func (v Value) Addr() Value {
	if v.flag&flagAddr == 0 {
		panic("reflect.Value.Addr of unaddressable value")
	}
	return Value{v.typ.ptrTo(), v.ptr, v.flag.ro() | flag(Ptr)} // Ptr是指标类型的类型值
}

// Bool returns v's underlying value.
// It panics if v's kind is not Bool.
/**
 * Bool返回v的基础值，如果v的种类不是Bool则会恐慌。
 */
func (v Value) Bool() bool {
	v.mustBe(Bool)
	return *(*bool)(v.ptr) // 先转成bool类型指针，再取值
}

// Bytes returns v's underlying value.
// It panics if v's underlying value is not a slice of bytes.
/**
 * 字节返回v的底层值，如果v的底层值不是字节的片段则恐慌。
 */
func (v Value) Bytes() []byte {
	v.mustBe(Slice)
	if v.typ.Elem().Kind() != Uint8 {
		panic("reflect.Value.Bytes of non-byte slice")
	}
	// Slice is always bigger than a word; assume flagIndir.
	// 切片总是比字（word）大； 假设flagIndir已经被设置值。
	return *(*[]byte)(v.ptr)
}

// runes returns v's underlying value.
// It panics if v's underlying value is not a slice of runes (int32s).
/**
 * runes返回v的底层值。如果v的底层值不是一小段符文（int32s），它将惊慌。
 * @param 
 * @return 
 **/
func (v Value) runes() []rune {
	v.mustBe(Slice)
	if v.typ.Elem().Kind() != Int32 {
		panic("reflect.Value.Bytes of non-rune slice")
	}
	// Slice is always bigger than a word; assume flagIndir.
	return *(*[]rune)(v.ptr)
}

// CanAddr reports whether the value's address can be obtained with Addr.
// Such values are called addressable. A value is addressable if it is
// an element of a slice, an element of an addressable array,
// a field of an addressable struct, or the result of dereferencing a pointer.
// If CanAddr returns false, calling Addr will panic.
/**
 * CanAddr报告是否可以通过Addr获取值的地址。
 * 这样的值称为可寻址的。 如果值是切片的元素，可寻址数组的元素，可寻址结构的字段或取消引用指针的结果，则该值是可寻址的。
 * 如果CanAddr返回false，则调用Addr会引起恐慌。
 **/
func (v Value) CanAddr() bool {
	return v.flag&flagAddr != 0
}

// CanSet reports whether the value of v can be changed.
// A Value can be changed only if it is addressable and was not
// obtained by the use of unexported struct fields.
// If CanSet returns false, calling Set or any type-specific
// setter (e.g., SetBool, SetInt) will panic.
/**
 * CanSet报告v的值是否可以更改。
 * 仅当值是可寻址的并且不是通过使用未导出的结构字段获得的，才可以更改它。
 * 如果CanSet返回false，则调用Set或任何特定于类型的setter（例如SetBool，SetInt）都会引起恐慌。
 * @param
 * @return
 **/
func (v Value) CanSet() bool {
	return v.flag&(flagAddr|flagRO) == flagAddr
}

// Call calls the function v with the input arguments in.
// For example, if len(in) == 3, v.Call(in) represents the Go call v(in[0], in[1], in[2]).
// Call panics if v's Kind is not Func.
// It returns the output results as Values.
// As in Go, each input argument must be assignable to the
// type of the function's corresponding input parameter.
// If v is a variadic function, Call creates the variadic slice parameter
// itself, copying in the corresponding values.
/**
 * Call使用输入参数调用函数v。
 * 例如，如果len(in)== 3，则v.Call(in)表示Go调用v(in[0], in[1], in[2])。
 * 如果v的Kind不是Func，则Call方法引起恐慌。
 * 将输出结果作为Value切片返回。
 * 和Go一样，每个输入参数必须可分配给函数相应输入参数的类型。
 * 如果v是可变参数函数，则Call会自己创建可变参数切片参数，并复制相应的值。
 * @param
 * @return
 **/
func (v Value) Call(in []Value) []Value {
	v.mustBe(Func)
	v.mustBeExported()
	return v.call("Call", in)
}

// CallSlice calls the variadic function v with the input arguments in,
// assigning the slice in[len(in)-1] to v's final variadic argument.
// For example, if len(in) == 3, v.CallSlice(in) represents the Go call v(in[0], in[1], in[2]...).
// CallSlice panics if v's Kind is not Func or if v is not variadic.
// It returns the output results as Values.
// As in Go, each input argument must be assignable to the
// type of the function's corresponding input parameter.
/**
 * CallSlice使用输入参数in调用可变参数函数v，将切片in [len(in)-1]分配给v的最终可变参数。
 * 例如，如果len(in) == 3，则v.CallSlice(in)表示Go调用v(in [0]，in [1]，in [2] ...)。
 * 如果v的Kind不是Func或v不是可变参数，则CallSlice会慌张。
 * 将输出结果作为Value切片返回。
 * 和Go一样，每个输入参数必须可分配给函数相应输入参数的类型。
 * @param
 * @return
 **/
func (v Value) CallSlice(in []Value) []Value {
	v.mustBe(Func)
	v.mustBeExported()
	return v.call("CallSlice", in)
}

var callGC bool // for testing; see TestCallMethodJump

/**
 * 真正的方法调用在这里
 * @param
 * @return
 **/
func (v Value) call(op string, in []Value) []Value {
	// Get function pointer, type.
	t := (*funcType)(unsafe.Pointer(v.typ))
	var (
		fn       unsafe.Pointer
		rcvr     Value
		rcvrtype *rtype
	)
	if v.flag&flagMethod != 0 { // v是方法的接收者
		rcvr = v
		rcvrtype, t, fn = methodReceiver(op, v, int(v.flag)>>flagMethodShift)
	} else if v.flag&flagIndir != 0 { // 有间接指针
		fn = *(*unsafe.Pointer)(v.ptr)
	} else {
		fn = v.ptr
	}

	if fn == nil {
		panic("reflect.Value.Call: call of nil function")
	}

	isSlice := op == "CallSlice"
	n := t.NumIn()
	if isSlice { // CallSlice方法
		if !t.IsVariadic() { // 必须要有可变参数
			panic("reflect: CallSlice of non-variadic function")
		}
		if len(in) < n { // 参数不足
			panic("reflect: CallSlice with too few input arguments")
		}
		if len(in) > n { // 参数过多
			panic("reflect: CallSlice with too many input arguments")
		}
	} else {
		if t.IsVariadic() { // 有可变参数
			n--
		}
		if len(in) < n {
			panic("reflect: Call with too few input arguments")
		}
		if !t.IsVariadic() && len(in) > n {
			panic("reflect: Call with too many input arguments")
		}
	}
	for _, x := range in { // 参数都要有效
		if x.Kind() == Invalid {
			panic("reflect: " + op + " using zero Value argument")
		}
	}
	for i := 0; i < n; i++ { // 对应参数可以赋值
		if xt, targ := in[i].Type(), t.In(i); !xt.AssignableTo(targ) {
			panic("reflect: " + op + " using " + xt.String() + " as type " + targ.String())
		}
	}
	if !isSlice && t.IsVariadic() { // 非CallSlice方法，且没有可变参数
		// prepare slice for remaining values
		// 将[len(in)-n, len(n)-1]位置的元素装入slice作为最后一个参数
		m := len(in) - n
		slice := MakeSlice(t.In(n), m, m)
		elem := t.In(n).Elem()
		for i := 0; i < m; i++ {
			x := in[n+i]
			if xt := x.Type(); !xt.AssignableTo(elem) { // n位置之后的元素必须是可赋值成elem
				panic("reflect: cannot use " + xt.String() + " as type " + elem.String() + " in " + op)
			}
			slice.Index(i).Set(x)
		}
		origIn := in
		in = make([]Value, n+1)
		copy(in[:n], origIn)
		in[n] = slice
	}

	nin := len(in)
	if nin != t.NumIn() { // 入参个数和需要的不相同
		panic("reflect.Value.Call: wrong argument count")
	}
	nout := t.NumOut()

	// Compute frame type.
	// 计算帧类型
	frametype, _, retOffset, _, framePool := funcLayout(t, rcvrtype)

	// Allocate a chunk of memory for frame.
	// 为帧分匹配大片内存
	var args unsafe.Pointer
	if nout == 0 { // 没有出参
		args = framePool.Get().(unsafe.Pointer)
	} else {
		// Can't use pool if the function has return values.
		// We will leak pointer to args in ret, so its lifetime is not scoped.
		// 如果函数具有返回值，则不能使用缓存池。
        // 我们将在ret中泄漏指向args的指针，因此其生存期不受限制。
		args = unsafe_New(frametype)
	}
	off := uintptr(0)

	// Copy inputs into args.
	// 将输入复制到args。
	if rcvrtype != nil {
		storeRcvr(rcvr, args)
		off = ptrSize
	}
	// 计算偏移量off
	for i, v := range in {
		v.mustBeExported()
		targ := t.In(i).(*rtype)
		a := uintptr(targ.align)
		off = (off + a - 1) &^ (a - 1)
		n := targ.size
		if n == 0 {
			// Not safe to compute args+off pointing at 0 bytes,
			// because that might point beyond the end of the frame,
			// but we still need to call assignTo to check assignability.
			// 计算指向0字节的args + off并不安全，因为它可能指向超出帧末尾的位置，
			// 但是我们仍然需要调用assignTo来检查可分配性。
			v.assignTo("reflect.Value.Call", targ, nil)
			continue
		}
		addr := add(args, off, "n > 0")
		v = v.assignTo("reflect.Value.Call", targ, addr)
		if v.flag&flagIndir != 0 {
			typedmemmove(targ, addr, v.ptr)
		} else {
			*(*unsafe.Pointer)(addr) = v.ptr
		}
		off += n
	}

	// Call.
	// 进行方法调用
	call(frametype, fn, args, uint32(frametype.size), uint32(retOffset))

	// For testing; see TestCallMethodJump.
	if callGC {
		runtime.GC()
	}

	var ret []Value
	if nout == 0 { // 没有出参
		typedmemclr(frametype, args)
		framePool.Put(args)
	} else { // 包装返回值
		// Zero the now unused input area of args,
		// because the Values returned by this function contain pointers to the args object,
		// and will thus keep the args object alive indefinitely.
		// 将现在未使用的args输入区域归零，因为此函数返回的值包含指向args对象的指针，
		// 因此将使args对象无限期地保持活动状态。
		typedmemclrpartial(frametype, args, 0, retOffset)

		// Wrap Values around return values in args.
		// args中的返回值进行包装
		ret = make([]Value, nout)
		off = retOffset
		for i := 0; i < nout; i++ {
			tv := t.Out(i)
			a := uintptr(tv.Align())
			off = (off + a - 1) &^ (a - 1)
			if tv.Size() != 0 {
				fl := flagIndir | flag(tv.Kind())
				ret[i] = Value{tv.common(), add(args, off, "tv.Size() != 0"), fl}
				// Note: this does introduce false sharing between results -
				// if any result is live, they are all live.
				// (And the space for the args is live as well, but as we've
				// cleared that space it isn't as big a deal.)
				// 注意：这确实会导致结果之间的错误共享-如果有任何结果存活，则它们都是在存活。
				// （并且用于args的空间也可以使用，但是正如我们已经清除的那样，空间并不大。）
			} else {
				// For zero-sized return value, args+off may point to the next object.
				// In this case, return the zero value instead.
				// 对于零大小的返回值，args+off可能指向下一个对象。 在这种情况下，请返回零值。
				ret[i] = Zero(tv)
			}
			off += tv.Size()
		}
	}

	return ret
}

// callReflect is the call implementation used by a function
// returned by MakeFunc. In many ways it is the opposite of the
// method Value.call above. The method above converts a call using Values
// into a call of a function with a concrete argument frame, while
// callReflect converts a call of a function with a concrete argument
// frame into a call using Values.
// It is in this file so that it can be next to the call method above.
// The remainder of the MakeFunc implementation is in makefunc.go.
//
// NOTE: This function must be marked as a "wrapper" in the generated code,
// so that the linker can make it work correctly for panic and recover.
// The gc compilers know to do that for the name "reflect.callReflect".
//
// ctxt is the "closure" generated by MakeFunc.
// frame is a pointer to the arguments to that closure on the stack.
// retValid points to a boolean which should be set when the results
// section of frame is set.
/**
 * callReflect是MakeFunc返回的函数使用的调用实现。 在许多方面，它与上面的Value.call方法相反。
 * 上面的方法将使用Values的调用转换为具有具体参数框架的函数的调用，
 * 而callReflect将具有具体参数框架的函数调用转换为使用Values的调用。
 * 它在此文件中，因此可以位于上面的call方法旁边。
 *  MakeFunc实现的其余部分位于makefunc.go中。
 *
 * 注意：此函数必须在生成的代码中标记为“wrapper”，以便链接器可以使其正常工作，以免发生混乱和恢复。
 *  gc编译器知道这样做的名称是“reflect.callReflect”。
 *
 *  ctxt是MakeFunc生成的“closure”。
 *  frame是指向堆栈上该闭包的参数的指针。
 *  retValid指向一个布尔值，当设置框架的结果部分时应设置此布尔值。
 * @param
 * @param
 * @param retValid 指标返回值是否有效
 **/
func callReflect(ctxt *makeFuncImpl, frame unsafe.Pointer, retValid *bool) {
	ftyp := ctxt.ftyp
	f := ctxt.fn

	// Copy argument frame into Values.
	// 将参数框帧复制到Value结构体中。
	ptr := frame
	off := uintptr(0) // 计算偏移量off
	in := make([]Value, 0, int(ftyp.inCount))
	for _, typ := range ftyp.in() { // 处理入参
		off += -off & uintptr(typ.align-1)
		v := Value{typ, nil, flag(typ.Kind())}
		if ifaceIndir(typ) { // 有间接指针
			// value cannot be inlined in interface data.
			// Must make a copy, because f might keep a reference to it,
			// and we cannot let f keep a reference to the stack frame
			// after this function returns, not even a read-only reference.
			// 不能在接口数据中内联值。
            // 必须创建一个副本，因为f可能会保留对它的引用，并且在此函数返回后，
            // 我们不能让f保留对堆栈帧的引用，甚至不能是只读引用。
			v.ptr = unsafe_New(typ)
			if typ.size > 0 {
				typedmemmove(typ, v.ptr, add(ptr, off, "typ.size > 0"))
			}
			v.flag |= flagIndir
		} else {
			v.ptr = *(*unsafe.Pointer)(add(ptr, off, "1-ptr"))
		}
		in = append(in, v)
		off += typ.size
	}

	// Call underlying function.
	// 调用底层函数
	out := f(in)
	numOut := ftyp.NumOut()
	if len(out) != numOut { // 出参和实际不同
		panic("reflect: wrong return count from function created by MakeFunc")
	}

	// Copy results back into argument frame.
	// 将结果拷贝回参数帧
	if numOut > 0 {
		off += -off & (ptrSize - 1)
		for i, typ := range ftyp.out() {
			v := out[i]
			if v.typ == nil {
				panic("reflect: function created by MakeFunc using " + funcName(f) +
					" returned zero Value")
			}
			if v.flag&flagRO != 0 {
				panic("reflect: function created by MakeFunc using " + funcName(f) +
					" returned value obtained from unexported field")
			}
			off += -off & uintptr(typ.align-1)
			if typ.size == 0 {
				continue
			}
			addr := add(ptr, off, "typ.size > 0")

			// Convert v to type typ if v is assignable to a variable
			// of type t in the language spec.
			// See issue 28761.
			// 如果在语言规范中v可分配给类型t的变量，则将v转换为typ类型。
            // 参见问题28761。
			v = v.assignTo("reflect.MakeFunc", typ, addr)

			// We are writing to stack. No write barrier.
			// 我们正在写堆栈。 没有写屏障。
			if v.flag&flagIndir != 0 {
				memmove(addr, v.ptr, typ.size)
			} else {
				*(*uintptr)(addr) = uintptr(v.ptr)
			}
			off += typ.size
		}
	}

	// Announce that the return values are valid.
	// After this point the runtime can depend on the return values being valid.
	// 声明返回值有效。
    // 在此之后，运行时可以依赖于有效的返回值。
	*retValid = true

	// We have to make sure that the out slice lives at least until
	// the runtime knows the return values are valid. Otherwise, the
	// return values might not be scanned by anyone during a GC.
	// (out would be dead, and the return slots not yet alive.)
	// 我们必须确保out输出切片至少存在，直到运行时知道返回值有效为止。
	// 否则，任何人都可能在GC期间不扫描返回值。（输出将失效，返回slots还没有激活。）
	runtime.KeepAlive(out)

	// runtime.getArgInfo expects to be able to find ctxt on the
	// stack when it finds our caller, makeFuncStub. Make sure it
	// doesn't get garbage collected.
	// runtime.getArgInfo期望能够在找到我们的调用者makeFuncStub时在堆栈上找到ctxt。
	// 需求确保没有收集垃圾。
	runtime.KeepAlive(ctxt)
}

// methodReceiver returns information about the receiver
// described by v. The Value v may or may not have the
// flagMethod bit set, so the kind cached in v.flag should
// not be used.
// The return value rcvrtype gives the method's actual receiver type.
// The return value t gives the method type signature (without the receiver).
// The return value fn is a pointer to the method code.
/**
 * methodReceiver返回有关v描述的接收者的信息。值v可能会或可能不会设置flagMethod位，因此不应使用在v.flag中缓存的种类。
 * 返回值rcvrtype给出了方法的实际接收者类型。
 * 返回值t给出方法类型签名（没有接收者）。
 * 返回值fn是方法代码的指针。
 * @param
 * @return
 **/
func methodReceiver(op string, v Value, methodIndex int) (rcvrtype *rtype, t *funcType, fn unsafe.Pointer) {
	i := methodIndex
	if v.typ.Kind() == Interface { // 接口类型
		tt := (*interfaceType)(unsafe.Pointer(v.typ))
		if uint(i) >= uint(len(tt.methods)) { // 参数个数判断
			panic("reflect: internal error: invalid method index")
		}
		m := &tt.methods[i]
		if !tt.nameOff(m.name).isExported() { // 非导出方法
			panic("reflect: " + op + " of unexported method")
		}
		iface := (*nonEmptyInterface)(v.ptr)
		if iface.itab == nil {
			panic("reflect: " + op + " of method on nil interface value")
		}
		rcvrtype = iface.itab.typ
		fn = unsafe.Pointer(&iface.itab.fun[i])
		t = (*funcType)(unsafe.Pointer(tt.typeOff(m.typ)))
	} else { // 非接口类型
		rcvrtype = v.typ
		ms := v.typ.exportedMethods()
		if uint(i) >= uint(len(ms)) {
			panic("reflect: internal error: invalid method index")
		}
		m := ms[i]
		if !v.typ.nameOff(m.name).isExported() {
			panic("reflect: " + op + " of unexported method")
		}
		ifn := v.typ.textOff(m.ifn)
		fn = unsafe.Pointer(&ifn)
		t = (*funcType)(unsafe.Pointer(v.typ.typeOff(m.mtyp)))
	}
	return
}

// v is a method receiver. Store at p the word which is used to
// encode that receiver at the start of the argument list.
// Reflect uses the "interface" calling convention for
// methods, which always uses one word to record the receiver.
/**
 * v是方法接收者。 在参数列表的开头将用于对接收方进行编码的字（word）存储在p处。
 * Reflect使用方法的“interface”调用约定，该约定始终使用一个字（word）来记录接收者。
 * @param
 * @return
 **/
func storeRcvr(v Value, p unsafe.Pointer) {
	t := v.typ
	if t.Kind() == Interface { // 接口类型
		// the interface data word becomes the receiver word
		// 接口数据字(word)成为接收者（word）
		iface := (*nonEmptyInterface)(v.ptr)
		*(*unsafe.Pointer)(p) = iface.word
	} else if v.flag&flagIndir != 0 && !ifaceIndir(t) { // 间接指针
		*(*unsafe.Pointer)(p) = *(*unsafe.Pointer)(v.ptr)
	} else {
		*(*unsafe.Pointer)(p) = v.ptr
	}
}

// align returns the result of rounding x up to a multiple of n.
// n must be a power of two.
/**
 * align返回将x舍入为n的倍数的结果。 n必须是2的幂。
 * @param
 * @return
 **/
func align(x, n uintptr) uintptr {
	return (x + n - 1) &^ (n - 1)
}

// callMethod is the call implementation used by a function returned
// by makeMethodValue (used by v.Method(i).Interface()).
// It is a streamlined version of the usual reflect call: the caller has
// already laid out the argument frame for us, so we don't have
// to deal with individual Values for each argument.
// It is in this file so that it can be next to the two similar functions above.
// The remainder of the makeMethodValue implementation is in makefunc.go.
//
// NOTE: This function must be marked as a "wrapper" in the generated code,
// so that the linker can make it work correctly for panic and recover.
// The gc compilers know to do that for the name "reflect.callMethod".
//
// ctxt is the "closure" generated by makeVethodValue.
// frame is a pointer to the arguments to that closure on the stack.
// retValid points to a boolean which should be set when the results
// section of frame is set.
/**
 * callMethod是由makeMethodValue返回的函数使用的调用实现（由v.Method(i).Interface()使用）。
 * 这是常规反射调用的简化版本：调用者已经为我们设置了参数帧，因此我们不必为每个参数处理单独的Values。
 * 它在此文件中，因此可以位于上面的两个类似函数的旁边。
 * makeMethodValue实现的其余部分位于makefunc.go中。
 *
 * 注意：此函数必须在生成的代码中标记为“wrapper”，以便链接器可以使其正常工作，以免发生混乱和恢复。
 * gc编译器为名称是“reflect.callMethod”做什么。
 *
 * @param ctxt是makeVethodValue生成的“closure”闭包。
 * @param frame是指向堆栈上该闭包的参数的指针。
 * @param retValid指向一个布尔值，当设置框架的结果部分时应设置此布尔值。
 * @return
 **/
func callMethod(ctxt *methodValue, frame unsafe.Pointer, retValid *bool) {
	rcvr := ctxt.rcvr
	rcvrtype, t, fn := methodReceiver("call", rcvr, ctxt.method)
	frametype, argSize, retOffset, _, framePool := funcLayout(t, rcvrtype)

	// Make a new frame that is one word bigger so we can store the receiver.
	// This space is used for both arguments and return values.
	// 制作一个新的帧，帧大一个字（word），这样我们就可以存储接收器。
    // 此空间用于参数和返回值。
	scratch := framePool.Get().(unsafe.Pointer)

	// Copy in receiver and rest of args.
	// 复制到接者和其他参数中。
	storeRcvr(rcvr, scratch)
	// Align the first arg. The alignment can't be larger than ptrSize.
	// 对齐第一个参数。 对齐不能大于ptrSize。
	argOffset := uintptr(ptrSize)
	if len(t.in()) > 0 {
		argOffset = align(argOffset, uintptr(t.in()[0].align))
	}
	// Avoid constructing out-of-bounds pointers if there are no args.
	// 如果没有参数，请避免构造越界指针。
	if argSize-argOffset > 0 {
		typedmemmovepartial(frametype, add(scratch, argOffset, "argSize > argOffset"), frame, argOffset, argSize-argOffset)
	}

	// Call.
	// Call copies the arguments from scratch to the stack, calls fn,
	// and then copies the results back into scratch.
	// call将参数从scratch复制到堆栈，调用fn，然后将结果复制回scratch。
	call(frametype, fn, scratch, uint32(frametype.size), uint32(retOffset))

	// Copy return values.
	// Ignore any changes to args and just copy return values.
	// Avoid constructing out-of-bounds pointers if there are no return values.
	// 复制返回值。
    // 忽略对args的任何更改，仅复制返回值。
    // 如果没有返回值，请避免构造越界指针。
	if frametype.size-retOffset > 0 {
		callerRetOffset := retOffset - argOffset
		// This copies to the stack. Write barriers are not needed.
		// 这将复制到堆栈。不需要写障碍。
		memmove(add(frame, callerRetOffset, "frametype.size > retOffset"),
			add(scratch, retOffset, "frametype.size > retOffset"),
			frametype.size-retOffset)
	}

	// Tell the runtime it can now depend on the return values
	// being properly initialized.
	// 告诉运行(runtime)时，它现在可以依赖正确初始化的返回值。
	*retValid = true

	// Clear the scratch space and put it back in the pool.
	// This must happen after the statement above, so that the return
	// values will always be scanned by someone.
	// 清除scratch空间并将其放回池中。
    // 这必须在上面的语句之后发生，以便返回值将始终被扫描到。
	typedmemclr(frametype, scratch)
	framePool.Put(scratch)

	// See the comment in callReflect.
	// 请参阅callReflect中的注释。
	runtime.KeepAlive(ctxt)
}

// funcName returns the name of f, for use in error messages.
/**
 * funcName返回f名称，以用于错误消息。
 * @param
 * @return
 **/
func funcName(f func([]Value) []Value) string {
	pc := *(*uintptr)(unsafe.Pointer(&f))
	rf := runtime.FuncForPC(pc)
	if rf != nil {
		return rf.Name()
	}
	return "closure"
}

// Cap returns v's capacity.
// It panics if v's Kind is not Array, Chan, or Slice.
/**
 * Cap返回v的容量。
 * 如果v的Kind不是Array，Chan或Slice，它会引起恐慌。
 * @param
 * @return
 **/
func (v Value) Cap() int {
	k := v.kind()
	switch k {
	case Array:
		return v.typ.Len()
	case Chan:
		return chancap(v.pointer())
	case Slice:
		// Slice is always bigger than a word; assume flagIndir.
		// 切片总是比单字（word）； 假设flagIndir已经被设置值。
		return (*sliceHeader)(v.ptr).Cap
	}
	panic(&ValueError{"reflect.Value.Cap", v.kind()})
}

// Close closes the channel v.
// It panics if v's Kind is not Chan.
/**
 * 关闭会关闭频道v。
 * 如果v's Kind不是Chan，就会引起恐慌。
 * @param
 * @return
 **/
func (v Value) Close() {
	v.mustBe(Chan)
	v.mustBeExported()
	chanclose(v.pointer())
}

// Complex returns v's underlying value, as a complex128.
// It panics if v's Kind is not Complex64 or Complex128
/**
 * Complex返回v的基础值，为complex128。
 * 如果v的Kind不是Complex64或Complex128，它会感到恐慌
 * @param
 * @return
 **/
func (v Value) Complex() complex128 {
	k := v.kind()
	switch k {
	case Complex64:
		return complex128(*(*complex64)(v.ptr))
	case Complex128:
		return *(*complex128)(v.ptr)
	}
	panic(&ValueError{"reflect.Value.Complex", v.kind()})
}

// Elem returns the value that the interface v contains
// or that the pointer v points to.
// It panics if v's Kind is not Interface or Ptr.
// It returns the zero Value if v is nil.
/**
 * Elem返回接口v包含的值或指针v指向的值。
 * 如果v的Kind不是Interface或Ptr，它会引起恐慌。
 * 如果v为nil，则返回零值。
 * @param
 * @return
 **/
func (v Value) Elem() Value {
	k := v.kind()
	switch k {
	case Interface:  // 接口类型
		var eface interface{}
		if v.typ.NumMethod() == 0 { // 没有方法
			eface = *(*interface{})(v.ptr)
		} else {
			eface = (interface{})(*(*interface {
				M()
			})(v.ptr))
		}
		x := unpackEface(eface)
		if x.flag != 0 {
			x.flag |= v.flag.ro()
		}
		return x
	case Ptr: // 指针类型
		ptr := v.ptr
		if v.flag&flagIndir != 0 { // 间接指针
			ptr = *(*unsafe.Pointer)(ptr)
		}
		// The returned value's address is v's value.
		// 返回值的地址是v的值。
		if ptr == nil {
			return Value{}
		}
		tt := (*ptrType)(unsafe.Pointer(v.typ))
		typ := tt.elem
		fl := v.flag&flagRO | flagIndir | flagAddr
		fl |= flag(typ.Kind())
		return Value{typ, ptr, fl}
	}
	panic(&ValueError{"reflect.Value.Elem", v.kind()})
}

// Field returns the i'th field of the struct v.
// It panics if v's Kind is not Struct or i is out of range.
/**
 * Field返回结构v的第i个字段。
 * 如果v的Kind不是Struct或i超出范围，它会引起恐慌。
 * @param
 * @return
 **/
func (v Value) Field(i int) Value {
	if v.kind() != Struct {
		panic(&ValueError{"reflect.Value.Field", v.kind()})
	}
	tt := (*structType)(unsafe.Pointer(v.typ))
	if uint(i) >= uint(len(tt.fields)) {
		panic("reflect: Field index out of range")
	}
	field := &tt.fields[i]
	typ := field.typ

	// Inherit permission bits from v, but clear flagEmbedRO.
	// 从v继承权限位，但清除flagEmbedRO。
	fl := v.flag&(flagStickyRO|flagIndir|flagAddr) | flag(typ.Kind())
	// Using an unexported field forces flagRO.
	// 使用未导出的字段会强制flagRO。
	if !field.name.isExported() {
		if field.embedded() {
			fl |= flagEmbedRO
		} else {
			fl |= flagStickyRO
		}
	}
	// Either flagIndir is set and v.ptr points at struct,
	// or flagIndir is not set and v.ptr is the actual struct data.
	// In the former case, we want v.ptr + offset.
	// In the latter case, we must have field.offset = 0,
	// so v.ptr + field.offset is still the correct address.
	/**
	 * 要么设置了flagIndir并且v.ptr指向结构体，要么未设置flagIndir且v.ptr是实际的结构数据。
     * 在前一种情况下，我们需要v.ptr+偏移量。
     * 在后一种情况下，我们必须具有field.offset = 0，因此v.ptr + field.offset仍然是正确的地址。
	 */
	ptr := add(v.ptr, field.offset(), "same as non-reflect &v.field")
	return Value{typ, ptr, fl}
}

// FieldByIndex returns the nested field corresponding to index.
// It panics if v's Kind is not struct.
/**
 * FieldByIndex返回与索引对应的嵌套字段。
 * 如果v的Kind不是struct，它会引起恐慌。
 * @param
 * @return
 **/
func (v Value) FieldByIndex(index []int) Value {
	if len(index) == 1 {
		return v.Field(index[0])
	}
	v.mustBe(Struct)
	for i, x := range index { // 递归求属性
		if i > 0 {
			if v.Kind() == Ptr && v.typ.Elem().Kind() == Struct {
				if v.IsNil() {
					panic("reflect: indirection through nil pointer to embedded struct")
				}
				v = v.Elem()
			}
		}
		v = v.Field(x)
	}
	return v
}

// FieldByName returns the struct field with the given name.
// It returns the zero Value if no field was found.
// It panics if v's Kind is not struct.
/**
 * FieldByName返回具有给定名称的struct字段。
 * 如果未找到任何字段，则返回零值。
 * 如果v的Kind不是struct，它会引起恐慌。
 * @param
 * @return
 **/
func (v Value) FieldByName(name string) Value {
	v.mustBe(Struct)
	if f, ok := v.typ.FieldByName(name); ok {
		return v.FieldByIndex(f.Index)
	}
	return Value{}
}

// FieldByNameFunc returns the struct field with a name
// that satisfies the match function.
// It panics if v's Kind is not struct.
// It returns the zero Value if no field was found.
/**
 * FieldByNameFunc返回具有满足match函数名称的struct字段。
 * 如果v的Kind不是struct，它会引起恐慌。
 * 如果未找到任何字段，则返回零值。
 * @param
 * @return
 **/
func (v Value) FieldByNameFunc(match func(string) bool) Value {
	if f, ok := v.typ.FieldByNameFunc(match); ok {
		return v.FieldByIndex(f.Index)
	}
	return Value{}
}

// Float returns v's underlying value, as a float64.
// It panics if v's Kind is not Float32 or Float64
/**
 * Float返回v的底层值，作为float64。
 * 如果v的Kind不是Float32或Float64，则会发生恐慌
 * @param
 * @return
 **/
func (v Value) Float() float64 {
	k := v.kind()
	switch k {
	case Float32:
		return float64(*(*float32)(v.ptr))
	case Float64:
		return *(*float64)(v.ptr)
	}
	panic(&ValueError{"reflect.Value.Float", v.kind()})
}

var uint8Type = TypeOf(uint8(0)).(*rtype)

// Index returns v's i'th element.
// It panics if v's Kind is not Array, Slice, or String or i is out of range.
/**
 * index返回v的第i个元素。
 * 如果v的Kind不是Array，Slice或String或i不在范围之内，它会引起恐慌。
 * @param
 * @return
 **/
func (v Value) Index(i int) Value {
	switch v.kind() {
	case Array:
		tt := (*arrayType)(unsafe.Pointer(v.typ))
		if uint(i) >= uint(tt.len) {
			panic("reflect: array index out of range")
		}
		typ := tt.elem
		offset := uintptr(i) * typ.size

		// Either flagIndir is set and v.ptr points at array,
		// or flagIndir is not set and v.ptr is the actual array data.
		// In the former case, we want v.ptr + offset.
		// In the latter case, we must be doing Index(0), so offset = 0,
		// so v.ptr + offset is still the correct address.
		// 或者设置了flagIndir并且v.ptr指向数组，或者没有设置flagIndir且v.ptr是实际的数组数据。
        // 在前一种情况下，我们需要v.ptr+偏移量。
        // 在后一种情况下，我们必须执行Index(0)，所以offset = 0，所以v.ptr + offset仍然是正确的地址。
		val := add(v.ptr, offset, "same as &v[i], i < tt.len")
		fl := v.flag&(flagIndir|flagAddr) | v.flag.ro() | flag(typ.Kind()) // bits same as overall array // 位标记和整个数据相同
		return Value{typ, val, fl}

	case Slice:
		// Element flag same as Elem of Ptr.
		// Addressable, indirect, possibly read-only.
		// 元素标志与Ptr的Elem相同。 可寻址，间接或可能是只读的。
		s := (*sliceHeader)(v.ptr)
		if uint(i) >= uint(s.Len) {
			panic("reflect: slice index out of range")
		}
		tt := (*sliceType)(unsafe.Pointer(v.typ))
		typ := tt.elem
		val := arrayAt(s.Data, i, typ.size, "i < s.Len")
		fl := flagAddr | flagIndir | v.flag.ro() | flag(typ.Kind())
		return Value{typ, val, fl}

	case String:
		s := (*stringHeader)(v.ptr)
		if uint(i) >= uint(s.Len) {
			panic("reflect: string index out of range")
		}
		p := arrayAt(s.Data, i, 1, "i < s.Len")
		fl := v.flag.ro() | flag(Uint8) | flagIndir
		return Value{uint8Type, p, fl}
	}
	panic(&ValueError{"reflect.Value.Index", v.kind()})
}

// Int returns v's underlying value, as an int64.
// It panics if v's Kind is not Int, Int8, Int16, Int32, or Int64.
/**
 * Int返回v的底层值，作为int64。
 * 如果v的Kind不是Int，Int8，Int16，Int32或Int64，则会发生恐慌。
 * @param
 * @return
 **/
func (v Value) Int() int64 {
	k := v.kind()
	p := v.ptr
	switch k {
	case Int:
		return int64(*(*int)(p))
	case Int8:
		return int64(*(*int8)(p))
	case Int16:
		return int64(*(*int16)(p))
	case Int32:
		return int64(*(*int32)(p))
	case Int64:
		return *(*int64)(p)
	}
	panic(&ValueError{"reflect.Value.Int", v.kind()})
}

// CanInterface reports whether Interface can be used without panicking.
/**
 * CanInterface报告是否可以在不引起恐慌的情况下使用Interface。
 * @param
 * @return
 **/
func (v Value) CanInterface() bool {
	if v.flag == 0 {
		panic(&ValueError{"reflect.Value.CanInterface", Invalid})
	}
	return v.flag&flagRO == 0
}

// Interface returns v's current value as an interface{}.
// It is equivalent to:
//	var i interface{} = (v's underlying value)
// It panics if the Value was obtained by accessing
// unexported struct fields.
/**
 * Interface返回v的当前值作为interface{}。
 * 等同于：
 *      var i interface {} =(v的底层值)
 * 如果通过访问未导出的struct字段获得了Value，则会感到恐慌。
 * @param 
 * @return 
 **/
func (v Value) Interface() (i interface{}) {
	return valueInterface(v, true)
}

/**
 * 将value值转成interface
 * @param
 * @return 
 **/
func valueInterface(v Value, safe bool) interface{} {
	if v.flag == 0 { // 最低5位表类型，0表示非法类型
		panic(&ValueError{"reflect.Value.Interface", Invalid})
	}
	if safe && v.flag&flagRO != 0 {
		// Do not allow access to unexported values via Interface,
		// because they might be pointers that should not be
		// writable or methods or function that should not be callable.
		// 不允许通过接口访问未导出的值，因为它们可能是不应写的指针或不应被调用的方法或函数。
		panic("reflect.Value.Interface: cannot return value obtained from unexported field or method")
	}
	if v.flag&flagMethod != 0 {
		v = makeMethodValue("Interface", v) // 构造方法值
	}

	if v.kind() == Interface { // 接口类型
		// Special case: return the element inside the interface.
		// Empty interface has one layout, all interfaces with
		// methods have a second layout.
		// 特殊情况：返回接口内的元素。
        // 空接口具有一种布局，所有带方法的接口具有另一种布局。
		if v.NumMethod() == 0 {
			return *(*interface{})(v.ptr)
		}
		return *(*interface { // 非空接口的布局
			M()
		})(v.ptr)
	}

	// TODO: pass safe to packEface so we don't need to copy if safe==true?
	// TODO: 将safe传递给packEface，这样如果safe == true，我们不需要复制吗？
	return packEface(v)
}

// InterfaceData returns the interface v's value as a uintptr pair.
// It panics if v's Kind is not Interface.
/**
 * InterfaceData接口返回v两个uintptr值。
 * 如果v的Kind不是Interface，它会引起恐慌。
 * @param
 * @return
 **/
func (v Value) InterfaceData() [2]uintptr {
	// TODO: deprecate this
	v.mustBe(Interface)
	// We treat this as a read operation, so we allow
	// it even for unexported data, because the caller
	// has to import "unsafe" to turn it into something
	// that can be abused.
	// Interface value is always bigger than a word; assume flagIndir.
	// 我们将其视为读取操作，因此即使未导出的数据也允许，
	// 因为调用者必须导入“unsafe”才能将其转换为可滥用的内容。
    // 接口值始终大于一个字（word）； 假设flagIndir已经被设置值。
	return *(*[2]uintptr)(v.ptr)
}

// IsNil reports whether its argument v is nil. The argument must be
// a chan, func, interface, map, pointer, or slice value; if it is
// not, IsNil panics. Note that IsNil is not always equivalent to a
// regular comparison with nil in Go. For example, if v was created
// by calling ValueOf with an uninitialized interface variable i,
// i==nil will be true but v.IsNil will panic as v will be the zero
// Value.
/**
 * IsNil报告其参数v是否为nil。 参数必须是chan，func，interface，map，pointer或slice值；
 * 如果不是，则IsNil引起恐慌。 请注意，IsNil并不总是等同于Go中与nil的常规比较。
 * 例如，如果v是通过使用未初始化的接口变量i调用ValueOf来创建的，则i == nil将为true，
 * 但v.IsNil将引起恐慌，因为v将为零值。
 * @param
 * @return
 **/
func (v Value) IsNil() bool {
	k := v.kind()
	switch k {
	case Chan, Func, Map, Ptr, UnsafePointer:
		if v.flag&flagMethod != 0 { // 方法标记不为0
			return false
		}
		ptr := v.ptr
		if v.flag&flagIndir != 0 { // 间接指针
			ptr = *(*unsafe.Pointer)(ptr)
		}
		return ptr == nil
	case Interface, Slice:
		// Both interface and slice are nil if first word is 0.
		// Both are always bigger than a word; assume flagIndir.
		// 如果第一个字为0，则interface和slice均为零。
		// 两者值始终大于一个字（word）； 假设flagIndir已经被设置值。
		return *(*unsafe.Pointer)(v.ptr) == nil
	}
	panic(&ValueError{"reflect.Value.IsNil", v.kind()})
}

// IsValid reports whether v represents a value.
// It returns false if v is the zero Value.
// If IsValid returns false, all other methods except String panic.
// Most functions and methods never return an invalid Value.
// If one does, its documentation states the conditions explicitly.
/**
 * IsValid报告v是否表示一个值。
 * 如果v为零值，则返回false。
 * 如果IsValid返回false，则其他所有方法（除外String方法外）引起恐慌。
 * 大多数函数和方法从不返回无效的值。
 * 如果是，则其文档会明确说明条件。
 * @param
 * @return
 **/
func (v Value) IsValid() bool {
	return v.flag != 0
}

// IsZero reports whether v is the zero value for its type.
// It panics if the argument is invalid.
/**
 * IsZero报告v是否为其类型的零值。
 * 如果参数无效，则会出现恐慌。
 * @param
 * @return
 **/
func (v Value) IsZero() bool {
	switch v.kind() {
	case Bool:
		return !v.Bool()
	case Int, Int8, Int16, Int32, Int64:
		return v.Int() == 0
	case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
		return v.Uint() == 0
	case Float32, Float64:
		return math.Float64bits(v.Float()) == 0
	case Complex64, Complex128:
		c := v.Complex()
		return math.Float64bits(real(c)) == 0 && math.Float64bits(imag(c)) == 0
	case Array:
		for i := 0; i < v.Len(); i++ {
			if !v.Index(i).IsZero() {
				return false
			}
		}
		return true
	case Chan, Func, Interface, Map, Ptr, Slice, UnsafePointer:
		return v.IsNil()
	case String:
		return v.Len() == 0
	case Struct:
		for i := 0; i < v.NumField(); i++ {
			if !v.Field(i).IsZero() {
				return false
			}
		}
		return true
	default:
		// This should never happens, but will act as a safeguard for
		// later, as a default value doesn't makes sense here.
		// 这永远都不会发生，但是以后会作为一种保护措施，因为默认值在这里没有意义。
		panic(&ValueError{"reflect.Value.IsZero", v.Kind()})
	}
}

// Kind returns v's Kind.
// If v is the zero Value (IsValid returns false), Kind returns Invalid.
/**
 * Kind返回v的值类型。 如果v为零值（IsValid返回false），则Kind返回Invalid。
 * @param
 * @return
 **/
func (v Value) Kind() Kind {
	return v.kind()
}

// Len returns v's length.
// It panics if v's Kind is not Array, Chan, Map, Slice, or String.
/**
 * Len返回v的长度。
 * 如果v的Kind不是Array，Chan，Map，Slice或String，它会引起恐慌。
 * @param
 * @return
 **/
func (v Value) Len() int {
	k := v.kind()
	switch k {
	case Array:
		tt := (*arrayType)(unsafe.Pointer(v.typ))
		return int(tt.len)
	case Chan:
		return chanlen(v.pointer())
	case Map:
		return maplen(v.pointer())
	case Slice:
		// Slice is bigger than a word; assume flagIndir.
	    // 切片大于一个字； 假设flagIndir已经被设置值。
		return (*sliceHeader)(v.ptr).Len
	case String:
		// String is bigger than a word; assume flagIndir.
		// 字符串大于一个字； 假设flagIndir已经被设置值。
		return (*stringHeader)(v.ptr).Len
	}
	panic(&ValueError{"reflect.Value.Len", v.kind()})
}

// MapIndex returns the value associated with key in the map v.
// It panics if v's Kind is not Map.
// It returns the zero Value if key is not found in the map or if v represents a nil map.
// As in Go, the key's value must be assignable to the map's key type.
/**
 * MapIndex返回与map v中的key关联的值。
 * 如果v的Kind不是Map，它会引起恐慌。
 * 如果在map中未找到键或v代表nil地图，则返回零值。
 * 和Go一样，键的值必须可分配给地图的键类型。
 * @param
 * @return
 **/
func (v Value) MapIndex(key Value) Value {
	v.mustBe(Map)
	tt := (*mapType)(unsafe.Pointer(v.typ))

	// Do not require key to be exported, so that DeepEqual
	// and other programs can use all the keys returned by
	// MapKeys as arguments to MapIndex. If either the map
	// or the key is unexported, though, the result will be
	// considered unexported. This is consistent with the
	// behavior for structs, which allow read but not write
	// of unexported fields.
	// 不需要key是可导出的，以便DeepEqual和其他程序可以将MapKeys返回的所有key用作MapIndex的参数。
	// 但是，如果map或key是不可导出的，则结果将被视为未导出。 这与结构的行为一致，
	// 该结构允许读取但不能写入未导出的字段。
	key = key.assignTo("reflect.Value.MapIndex", tt.key, nil)

	var k unsafe.Pointer // 是间接指针
	if key.flag&flagIndir != 0 { // 间接指针
		k = key.ptr
	} else {
		k = unsafe.Pointer(&key.ptr) // 构造间接指针
	}
	e := mapaccess(v.typ, v.pointer(), k)
	if e == nil {
		return Value{}
	}
	typ := tt.elem
	fl := (v.flag | key.flag).ro()
	fl |= flag(typ.Kind())
	return copyVal(typ, fl, e)
}

// MapKeys returns a slice containing all the keys present in the map,
// in unspecified order.
// It panics if v's Kind is not Map.
// It returns an empty slice if v represents a nil map.
/**
 * MapKeys返回一个切片，其中包含未指定顺序的map中存在的所有键。
 * 如果v的Kind不是Map，它会引起恐慌。
 * 如果v代表nil映射，则返回一个空切片。
 * @param
 * @return
 **/
func (v Value) MapKeys() []Value {
	v.mustBe(Map)
	tt := (*mapType)(unsafe.Pointer(v.typ))
	keyType := tt.key

	fl := v.flag.ro() | flag(keyType.Kind())

	m := v.pointer()
	mlen := int(0)
	if m != nil {
		mlen = maplen(m)
	}
	it := mapiterinit(v.typ, m)
	a := make([]Value, mlen)
	var i int
	for i = 0; i < len(a); i++ {
		key := mapiterkey(it)
		if key == nil {
			// Someone deleted an entry from the map since we
			// called maplen above. It's a data race, but nothing
			// we can do about it.
			// 自从我们在上面调用maplen以来，有人从map中删除了一个条目。
			// 这是一场数据竞赛，但是我们对此无能为力。
			break
		}
		a[i] = copyVal(keyType, fl, key)
		mapiternext(it)
	}
	return a[:i]
}

// A MapIter is an iterator for ranging over a map.
// See Value.MapRange.
/**
 * MapIter是用于遍历map的迭代器。请参见Value.MapRange。
 */
type MapIter struct {
	m  Value
	it unsafe.Pointer // map条件目的指针
}

// Key returns the key of the iterator's current map entry.
/**
 * Key返回迭代器的当前映射条目的key。
 */
func (it *MapIter) Key() Value {
	if it.it == nil {
		panic("MapIter.Key called before Next")
	}
	if mapiterkey(it.it) == nil {
		panic("MapIter.Key called on exhausted iterator")
	}

	t := (*mapType)(unsafe.Pointer(it.m.typ))
	ktype := t.key
	return copyVal(ktype, it.m.flag.ro()|flag(ktype.Kind()), mapiterkey(it.it))
}

// Value returns the value of the iterator's current map entry.
/**
 * Value返回迭代器的当前映射条目的value。
 */
func (it *MapIter) Value() Value {
	if it.it == nil {
		panic("MapIter.Value called before Next")
	}
	if mapiterkey(it.it) == nil {
		panic("MapIter.Value called on exhausted iterator")
	}

	t := (*mapType)(unsafe.Pointer(it.m.typ))
	vtype := t.elem
	return copyVal(vtype, it.m.flag.ro()|flag(vtype.Kind()), mapiterelem(it.it))
}

// Next advances the map iterator and reports whether there is another
// entry. It returns false when the iterator is exhausted; subsequent
// calls to Key, Value, or Next will panic.
func (it *MapIter) Next() bool {
	if it.it == nil {
		it.it = mapiterinit(it.m.typ, it.m.pointer())
	} else {
		if mapiterkey(it.it) == nil {
			panic("MapIter.Next called on exhausted iterator")
		}
		mapiternext(it.it)
	}
	return mapiterkey(it.it) != nil
}

// MapRange returns a range iterator for a map.
// It panics if v's Kind is not Map.
//
// Call Next to advance the iterator, and Key/Value to access each entry.
// Next returns false when the iterator is exhausted.
// MapRange follows the same iteration semantics as a range statement.
//
// Example:
//
//	iter := reflect.ValueOf(m).MapRange()
// 	for iter.Next() {
//		k := iter.Key()
//		v := iter.Value()
//		...
//	}
//
func (v Value) MapRange() *MapIter {
	v.mustBe(Map)
	return &MapIter{m: v}
}

// copyVal returns a Value containing the map key or value at ptr,
// allocating a new variable as needed.
func copyVal(typ *rtype, fl flag, ptr unsafe.Pointer) Value {
	if ifaceIndir(typ) {
		// Copy result so future changes to the map
		// won't change the underlying value.
		c := unsafe_New(typ)
		typedmemmove(typ, c, ptr)
		return Value{typ, c, fl | flagIndir}
	}
	return Value{typ, *(*unsafe.Pointer)(ptr), fl}
}

// Method returns a function value corresponding to v's i'th method.
// The arguments to a Call on the returned function should not include
// a receiver; the returned function will always use v as the receiver.
// Method panics if i is out of range or if v is a nil interface value.
func (v Value) Method(i int) Value {
	if v.typ == nil {
		panic(&ValueError{"reflect.Value.Method", Invalid})
	}
	if v.flag&flagMethod != 0 || uint(i) >= uint(v.typ.NumMethod()) {
		panic("reflect: Method index out of range")
	}
	if v.typ.Kind() == Interface && v.IsNil() {
		panic("reflect: Method on nil interface value")
	}
	fl := v.flag & (flagStickyRO | flagIndir) // Clear flagEmbedRO
	fl |= flag(Func)
	fl |= flag(i)<<flagMethodShift | flagMethod
	return Value{v.typ, v.ptr, fl}
}

// NumMethod returns the number of exported methods in the value's method set.
func (v Value) NumMethod() int {
	if v.typ == nil {
		panic(&ValueError{"reflect.Value.NumMethod", Invalid})
	}
	if v.flag&flagMethod != 0 {
		return 0
	}
	return v.typ.NumMethod()
}

// MethodByName returns a function value corresponding to the method
// of v with the given name.
// The arguments to a Call on the returned function should not include
// a receiver; the returned function will always use v as the receiver.
// It returns the zero Value if no method was found.
func (v Value) MethodByName(name string) Value {
	if v.typ == nil {
		panic(&ValueError{"reflect.Value.MethodByName", Invalid})
	}
	if v.flag&flagMethod != 0 {
		return Value{}
	}
	m, ok := v.typ.MethodByName(name)
	if !ok {
		return Value{}
	}
	return v.Method(m.Index)
}

// NumField returns the number of fields in the struct v.
// It panics if v's Kind is not Struct.
func (v Value) NumField() int {
	v.mustBe(Struct)
	tt := (*structType)(unsafe.Pointer(v.typ))
	return len(tt.fields)
}

// OverflowComplex reports whether the complex128 x cannot be represented by v's type.
// It panics if v's Kind is not Complex64 or Complex128.
func (v Value) OverflowComplex(x complex128) bool {
	k := v.kind()
	switch k {
	case Complex64:
		return overflowFloat32(real(x)) || overflowFloat32(imag(x))
	case Complex128:
		return false
	}
	panic(&ValueError{"reflect.Value.OverflowComplex", v.kind()})
}

// OverflowFloat reports whether the float64 x cannot be represented by v's type.
// It panics if v's Kind is not Float32 or Float64.
func (v Value) OverflowFloat(x float64) bool {
	k := v.kind()
	switch k {
	case Float32:
		return overflowFloat32(x)
	case Float64:
		return false
	}
	panic(&ValueError{"reflect.Value.OverflowFloat", v.kind()})
}

func overflowFloat32(x float64) bool {
	if x < 0 {
		x = -x
	}
	return math.MaxFloat32 < x && x <= math.MaxFloat64
}

// OverflowInt reports whether the int64 x cannot be represented by v's type.
// It panics if v's Kind is not Int, Int8, Int16, Int32, or Int64.
func (v Value) OverflowInt(x int64) bool {
	k := v.kind()
	switch k {
	case Int, Int8, Int16, Int32, Int64:
		bitSize := v.typ.size * 8
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic(&ValueError{"reflect.Value.OverflowInt", v.kind()})
}

// OverflowUint reports whether the uint64 x cannot be represented by v's type.
// It panics if v's Kind is not Uint, Uintptr, Uint8, Uint16, Uint32, or Uint64.
func (v Value) OverflowUint(x uint64) bool {
	k := v.kind()
	switch k {
	case Uint, Uintptr, Uint8, Uint16, Uint32, Uint64:
		bitSize := v.typ.size * 8
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic(&ValueError{"reflect.Value.OverflowUint", v.kind()})
}

//go:nocheckptr
// This prevents inlining Value.Pointer when -d=checkptr is enabled,
// which ensures cmd/compile can recognize unsafe.Pointer(v.Pointer())
// and make an exception.

// Pointer returns v's value as a uintptr.
// It returns uintptr instead of unsafe.Pointer so that
// code using reflect cannot obtain unsafe.Pointers
// without importing the unsafe package explicitly.
// It panics if v's Kind is not Chan, Func, Map, Ptr, Slice, or UnsafePointer.
//
// If v's Kind is Func, the returned pointer is an underlying
// code pointer, but not necessarily enough to identify a
// single function uniquely. The only guarantee is that the
// result is zero if and only if v is a nil func Value.
//
// If v's Kind is Slice, the returned pointer is to the first
// element of the slice. If the slice is nil the returned value
// is 0.  If the slice is empty but non-nil the return value is non-zero.
func (v Value) Pointer() uintptr {
	// TODO: deprecate
	k := v.kind()
	switch k {
	case Chan, Map, Ptr, UnsafePointer:
		return uintptr(v.pointer())
	case Func:
		if v.flag&flagMethod != 0 {
			// As the doc comment says, the returned pointer is an
			// underlying code pointer but not necessarily enough to
			// identify a single function uniquely. All method expressions
			// created via reflect have the same underlying code pointer,
			// so their Pointers are equal. The function used here must
			// match the one used in makeMethodValue.
			f := methodValueCall
			return **(**uintptr)(unsafe.Pointer(&f))
		}
		p := v.pointer()
		// Non-nil func value points at data block.
		// First word of data block is actual code.
		if p != nil {
			p = *(*unsafe.Pointer)(p)
		}
		return uintptr(p)

	case Slice:
		return (*SliceHeader)(v.ptr).Data
	}
	panic(&ValueError{"reflect.Value.Pointer", v.kind()})
}

// Recv receives and returns a value from the channel v.
// It panics if v's Kind is not Chan.
// The receive blocks until a value is ready.
// The boolean value ok is true if the value x corresponds to a send
// on the channel, false if it is a zero value received because the channel is closed.
func (v Value) Recv() (x Value, ok bool) {
	v.mustBe(Chan)
	v.mustBeExported()
	return v.recv(false)
}

// internal recv, possibly non-blocking (nb).
// v is known to be a channel.
func (v Value) recv(nb bool) (val Value, ok bool) {
	tt := (*chanType)(unsafe.Pointer(v.typ))
	if ChanDir(tt.dir)&RecvDir == 0 {
		panic("reflect: recv on send-only channel")
	}
	t := tt.elem
	val = Value{t, nil, flag(t.Kind())}
	var p unsafe.Pointer
	if ifaceIndir(t) {
		p = unsafe_New(t)
		val.ptr = p
		val.flag |= flagIndir
	} else {
		p = unsafe.Pointer(&val.ptr)
	}
	selected, ok := chanrecv(v.pointer(), nb, p)
	if !selected {
		val = Value{}
	}
	return
}

// Send sends x on the channel v.
// It panics if v's kind is not Chan or if x's type is not the same type as v's element type.
// As in Go, x's value must be assignable to the channel's element type.
func (v Value) Send(x Value) {
	v.mustBe(Chan)
	v.mustBeExported()
	v.send(x, false)
}

// internal send, possibly non-blocking.
// v is known to be a channel.
func (v Value) send(x Value, nb bool) (selected bool) {
	tt := (*chanType)(unsafe.Pointer(v.typ))
	if ChanDir(tt.dir)&SendDir == 0 {
		panic("reflect: send on recv-only channel")
	}
	x.mustBeExported()
	x = x.assignTo("reflect.Value.Send", tt.elem, nil)
	var p unsafe.Pointer
	if x.flag&flagIndir != 0 {
		p = x.ptr
	} else {
		p = unsafe.Pointer(&x.ptr)
	}
	return chansend(v.pointer(), p, nb)
}

// Set assigns x to the value v.
// It panics if CanSet returns false.
// As in Go, x's value must be assignable to v's type.
func (v Value) Set(x Value) {
	v.mustBeAssignable()
	x.mustBeExported() // do not let unexported x leak
	var target unsafe.Pointer
	if v.kind() == Interface {
		target = v.ptr
	}
	x = x.assignTo("reflect.Set", v.typ, target)
	if x.flag&flagIndir != 0 {
		typedmemmove(v.typ, v.ptr, x.ptr)
	} else {
		*(*unsafe.Pointer)(v.ptr) = x.ptr
	}
}

// SetBool sets v's underlying value.
// It panics if v's Kind is not Bool or if CanSet() is false.
func (v Value) SetBool(x bool) {
	v.mustBeAssignable()
	v.mustBe(Bool)
	*(*bool)(v.ptr) = x
}

// SetBytes sets v's underlying value.
// It panics if v's underlying value is not a slice of bytes.
func (v Value) SetBytes(x []byte) {
	v.mustBeAssignable()
	v.mustBe(Slice)
	if v.typ.Elem().Kind() != Uint8 {
		panic("reflect.Value.SetBytes of non-byte slice")
	}
	*(*[]byte)(v.ptr) = x
}

// setRunes sets v's underlying value.
// It panics if v's underlying value is not a slice of runes (int32s).
func (v Value) setRunes(x []rune) {
	v.mustBeAssignable()
	v.mustBe(Slice)
	if v.typ.Elem().Kind() != Int32 {
		panic("reflect.Value.setRunes of non-rune slice")
	}
	*(*[]rune)(v.ptr) = x
}

// SetComplex sets v's underlying value to x.
// It panics if v's Kind is not Complex64 or Complex128, or if CanSet() is false.
func (v Value) SetComplex(x complex128) {
	v.mustBeAssignable()
	switch k := v.kind(); k {
	default:
		panic(&ValueError{"reflect.Value.SetComplex", v.kind()})
	case Complex64:
		*(*complex64)(v.ptr) = complex64(x)
	case Complex128:
		*(*complex128)(v.ptr) = x
	}
}

// SetFloat sets v's underlying value to x.
// It panics if v's Kind is not Float32 or Float64, or if CanSet() is false.
func (v Value) SetFloat(x float64) {
	v.mustBeAssignable()
	switch k := v.kind(); k {
	default:
		panic(&ValueError{"reflect.Value.SetFloat", v.kind()})
	case Float32:
		*(*float32)(v.ptr) = float32(x)
	case Float64:
		*(*float64)(v.ptr) = x
	}
}

// SetInt sets v's underlying value to x.
// It panics if v's Kind is not Int, Int8, Int16, Int32, or Int64, or if CanSet() is false.
func (v Value) SetInt(x int64) {
	v.mustBeAssignable()
	switch k := v.kind(); k {
	default:
		panic(&ValueError{"reflect.Value.SetInt", v.kind()})
	case Int:
		*(*int)(v.ptr) = int(x)
	case Int8:
		*(*int8)(v.ptr) = int8(x)
	case Int16:
		*(*int16)(v.ptr) = int16(x)
	case Int32:
		*(*int32)(v.ptr) = int32(x)
	case Int64:
		*(*int64)(v.ptr) = x
	}
}

// SetLen sets v's length to n.
// It panics if v's Kind is not Slice or if n is negative or
// greater than the capacity of the slice.
func (v Value) SetLen(n int) {
	v.mustBeAssignable()
	v.mustBe(Slice)
	s := (*sliceHeader)(v.ptr)
	if uint(n) > uint(s.Cap) {
		panic("reflect: slice length out of range in SetLen")
	}
	s.Len = n
}

// SetCap sets v's capacity to n.
// It panics if v's Kind is not Slice or if n is smaller than the length or
// greater than the capacity of the slice.
func (v Value) SetCap(n int) {
	v.mustBeAssignable()
	v.mustBe(Slice)
	s := (*sliceHeader)(v.ptr)
	if n < s.Len || n > s.Cap {
		panic("reflect: slice capacity out of range in SetCap")
	}
	s.Cap = n
}

// SetMapIndex sets the element associated with key in the map v to elem.
// It panics if v's Kind is not Map.
// If elem is the zero Value, SetMapIndex deletes the key from the map.
// Otherwise if v holds a nil map, SetMapIndex will panic.
// As in Go, key's elem must be assignable to the map's key type,
// and elem's value must be assignable to the map's elem type.
func (v Value) SetMapIndex(key, elem Value) {
	v.mustBe(Map)
	v.mustBeExported()
	key.mustBeExported()
	tt := (*mapType)(unsafe.Pointer(v.typ))
	key = key.assignTo("reflect.Value.SetMapIndex", tt.key, nil)
	var k unsafe.Pointer
	if key.flag&flagIndir != 0 {
		k = key.ptr
	} else {
		k = unsafe.Pointer(&key.ptr)
	}
	if elem.typ == nil {
		mapdelete(v.typ, v.pointer(), k)
		return
	}
	elem.mustBeExported()
	elem = elem.assignTo("reflect.Value.SetMapIndex", tt.elem, nil)
	var e unsafe.Pointer
	if elem.flag&flagIndir != 0 {
		e = elem.ptr
	} else {
		e = unsafe.Pointer(&elem.ptr)
	}
	mapassign(v.typ, v.pointer(), k, e)
}

// SetUint sets v's underlying value to x.
// It panics if v's Kind is not Uint, Uintptr, Uint8, Uint16, Uint32, or Uint64, or if CanSet() is false.
func (v Value) SetUint(x uint64) {
	v.mustBeAssignable()
	switch k := v.kind(); k {
	default:
		panic(&ValueError{"reflect.Value.SetUint", v.kind()})
	case Uint:
		*(*uint)(v.ptr) = uint(x)
	case Uint8:
		*(*uint8)(v.ptr) = uint8(x)
	case Uint16:
		*(*uint16)(v.ptr) = uint16(x)
	case Uint32:
		*(*uint32)(v.ptr) = uint32(x)
	case Uint64:
		*(*uint64)(v.ptr) = x
	case Uintptr:
		*(*uintptr)(v.ptr) = uintptr(x)
	}
}

// SetPointer sets the unsafe.Pointer value v to x.
// It panics if v's Kind is not UnsafePointer.
func (v Value) SetPointer(x unsafe.Pointer) {
	v.mustBeAssignable()
	v.mustBe(UnsafePointer)
	*(*unsafe.Pointer)(v.ptr) = x
}

// SetString sets v's underlying value to x.
// It panics if v's Kind is not String or if CanSet() is false.
func (v Value) SetString(x string) {
	v.mustBeAssignable()
	v.mustBe(String)
	*(*string)(v.ptr) = x
}

// Slice returns v[i:j].
// It panics if v's Kind is not Array, Slice or String, or if v is an unaddressable array,
// or if the indexes are out of bounds.
func (v Value) Slice(i, j int) Value {
	var (
		cap  int
		typ  *sliceType
		base unsafe.Pointer
	)
	switch kind := v.kind(); kind {
	default:
		panic(&ValueError{"reflect.Value.Slice", v.kind()})

	case Array:
		if v.flag&flagAddr == 0 {
			panic("reflect.Value.Slice: slice of unaddressable array")
		}
		tt := (*arrayType)(unsafe.Pointer(v.typ))
		cap = int(tt.len)
		typ = (*sliceType)(unsafe.Pointer(tt.slice))
		base = v.ptr

	case Slice:
		typ = (*sliceType)(unsafe.Pointer(v.typ))
		s := (*sliceHeader)(v.ptr)
		base = s.Data
		cap = s.Cap

	case String:
		s := (*stringHeader)(v.ptr)
		if i < 0 || j < i || j > s.Len {
			panic("reflect.Value.Slice: string slice index out of bounds")
		}
		var t stringHeader
		if i < s.Len {
			t = stringHeader{arrayAt(s.Data, i, 1, "i < s.Len"), j - i}
		}
		return Value{v.typ, unsafe.Pointer(&t), v.flag}
	}

	if i < 0 || j < i || j > cap {
		panic("reflect.Value.Slice: slice index out of bounds")
	}

	// Declare slice so that gc can see the base pointer in it.
	var x []unsafe.Pointer

	// Reinterpret as *sliceHeader to edit.
	s := (*sliceHeader)(unsafe.Pointer(&x))
	s.Len = j - i
	s.Cap = cap - i
	if cap-i > 0 {
		s.Data = arrayAt(base, i, typ.elem.Size(), "i < cap")
	} else {
		// do not advance pointer, to avoid pointing beyond end of slice
		s.Data = base
	}

	fl := v.flag.ro() | flagIndir | flag(Slice)
	return Value{typ.common(), unsafe.Pointer(&x), fl}
}

// Slice3 is the 3-index form of the slice operation: it returns v[i:j:k].
// It panics if v's Kind is not Array or Slice, or if v is an unaddressable array,
// or if the indexes are out of bounds.
func (v Value) Slice3(i, j, k int) Value {
	var (
		cap  int
		typ  *sliceType
		base unsafe.Pointer
	)
	switch kind := v.kind(); kind {
	default:
		panic(&ValueError{"reflect.Value.Slice3", v.kind()})

	case Array:
		if v.flag&flagAddr == 0 {
			panic("reflect.Value.Slice3: slice of unaddressable array")
		}
		tt := (*arrayType)(unsafe.Pointer(v.typ))
		cap = int(tt.len)
		typ = (*sliceType)(unsafe.Pointer(tt.slice))
		base = v.ptr

	case Slice:
		typ = (*sliceType)(unsafe.Pointer(v.typ))
		s := (*sliceHeader)(v.ptr)
		base = s.Data
		cap = s.Cap
	}

	if i < 0 || j < i || k < j || k > cap {
		panic("reflect.Value.Slice3: slice index out of bounds")
	}

	// Declare slice so that the garbage collector
	// can see the base pointer in it.
	var x []unsafe.Pointer

	// Reinterpret as *sliceHeader to edit.
	s := (*sliceHeader)(unsafe.Pointer(&x))
	s.Len = j - i
	s.Cap = k - i
	if k-i > 0 {
		s.Data = arrayAt(base, i, typ.elem.Size(), "i < k <= cap")
	} else {
		// do not advance pointer, to avoid pointing beyond end of slice
		s.Data = base
	}

	fl := v.flag.ro() | flagIndir | flag(Slice)
	return Value{typ.common(), unsafe.Pointer(&x), fl}
}

// String returns the string v's underlying value, as a string.
// String is a special case because of Go's String method convention.
// Unlike the other getters, it does not panic if v's Kind is not String.
// Instead, it returns a string of the form "<T value>" where T is v's type.
// The fmt package treats Values specially. It does not call their String
// method implicitly but instead prints the concrete values they hold.
func (v Value) String() string {
	switch k := v.kind(); k {
	case Invalid:
		return "<invalid Value>"
	case String:
		return *(*string)(v.ptr)
	}
	// If you call String on a reflect.Value of other type, it's better to
	// print something than to panic. Useful in debugging.
	return "<" + v.Type().String() + " Value>"
}

// TryRecv attempts to receive a value from the channel v but will not block.
// It panics if v's Kind is not Chan.
// If the receive delivers a value, x is the transferred value and ok is true.
// If the receive cannot finish without blocking, x is the zero Value and ok is false.
// If the channel is closed, x is the zero value for the channel's element type and ok is false.
func (v Value) TryRecv() (x Value, ok bool) {
	v.mustBe(Chan)
	v.mustBeExported()
	return v.recv(true)
}

// TrySend attempts to send x on the channel v but will not block.
// It panics if v's Kind is not Chan.
// It reports whether the value was sent.
// As in Go, x's value must be assignable to the channel's element type.
func (v Value) TrySend(x Value) bool {
	v.mustBe(Chan)
	v.mustBeExported()
	return v.send(x, true)
}

// Type returns v's type.
func (v Value) Type() Type {
	f := v.flag
	if f == 0 {
		panic(&ValueError{"reflect.Value.Type", Invalid})
	}
	if f&flagMethod == 0 {
		// Easy case
		return v.typ
	}

	// Method value.
	// v.typ describes the receiver, not the method type.
	i := int(v.flag) >> flagMethodShift
	if v.typ.Kind() == Interface {
		// Method on interface.
		tt := (*interfaceType)(unsafe.Pointer(v.typ))
		if uint(i) >= uint(len(tt.methods)) {
			panic("reflect: internal error: invalid method index")
		}
		m := &tt.methods[i]
		return v.typ.typeOff(m.typ)
	}
	// Method on concrete type.
	ms := v.typ.exportedMethods()
	if uint(i) >= uint(len(ms)) {
		panic("reflect: internal error: invalid method index")
	}
	m := ms[i]
	return v.typ.typeOff(m.mtyp)
}

// Uint returns v's underlying value, as a uint64.
// It panics if v's Kind is not Uint, Uintptr, Uint8, Uint16, Uint32, or Uint64.
func (v Value) Uint() uint64 {
	k := v.kind()
	p := v.ptr
	switch k {
	case Uint:
		return uint64(*(*uint)(p))
	case Uint8:
		return uint64(*(*uint8)(p))
	case Uint16:
		return uint64(*(*uint16)(p))
	case Uint32:
		return uint64(*(*uint32)(p))
	case Uint64:
		return *(*uint64)(p)
	case Uintptr:
		return uint64(*(*uintptr)(p))
	}
	panic(&ValueError{"reflect.Value.Uint", v.kind()})
}

//go:nocheckptr
// This prevents inlining Value.UnsafeAddr when -d=checkptr is enabled,
// which ensures cmd/compile can recognize unsafe.Pointer(v.UnsafeAddr())
// and make an exception.

// UnsafeAddr returns a pointer to v's data.
// It is for advanced clients that also import the "unsafe" package.
// It panics if v is not addressable.
func (v Value) UnsafeAddr() uintptr {
	// TODO: deprecate
	if v.typ == nil {
		panic(&ValueError{"reflect.Value.UnsafeAddr", Invalid})
	}
	if v.flag&flagAddr == 0 {
		panic("reflect.Value.UnsafeAddr of unaddressable value")
	}
	return uintptr(v.ptr)
}

// StringHeader is the runtime representation of a string.
// It cannot be used safely or portably and its representation may
// change in a later release.
// Moreover, the Data field is not sufficient to guarantee the data
// it references will not be garbage collected, so programs must keep
// a separate, correctly typed pointer to the underlying data.
type StringHeader struct {
	Data uintptr
	Len  int
}

// stringHeader is a safe version of StringHeader used within this package.
type stringHeader struct {
	Data unsafe.Pointer
	Len  int
}

// SliceHeader is the runtime representation of a slice.
// It cannot be used safely or portably and its representation may
// change in a later release.
// Moreover, the Data field is not sufficient to guarantee the data
// it references will not be garbage collected, so programs must keep
// a separate, correctly typed pointer to the underlying data.
type SliceHeader struct {
	Data uintptr
	Len  int
	Cap  int
}

// sliceHeader is a safe version of SliceHeader used within this package.
type sliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

func typesMustMatch(what string, t1, t2 Type) {
	if t1 != t2 {
		panic(what + ": " + t1.String() + " != " + t2.String())
	}
}

// arrayAt returns the i-th element of p,
// an array whose elements are eltSize bytes wide.
// The array pointed at by p must have at least i+1 elements:
// it is invalid (but impossible to check here) to pass i >= len,
// because then the result will point outside the array.
// whySafe must explain why i < len. (Passing "i < len" is fine;
// the benefit is to surface this assumption at the call site.)
func arrayAt(p unsafe.Pointer, i int, eltSize uintptr, whySafe string) unsafe.Pointer {
	return add(p, uintptr(i)*eltSize, "i < len")
}

// grow grows the slice s so that it can hold extra more values, allocating
// more capacity if needed. It also returns the old and new slice lengths.
func grow(s Value, extra int) (Value, int, int) {
	i0 := s.Len()
	i1 := i0 + extra
	if i1 < i0 {
		panic("reflect.Append: slice overflow")
	}
	m := s.Cap()
	if i1 <= m {
		return s.Slice(0, i1), i0, i1
	}
	if m == 0 {
		m = extra
	} else {
		for m < i1 {
			if i0 < 1024 {
				m += m
			} else {
				m += m / 4
			}
		}
	}
	t := MakeSlice(s.Type(), i1, m)
	Copy(t, s)
	return t, i0, i1
}

// Append appends the values x to a slice s and returns the resulting slice.
// As in Go, each x's value must be assignable to the slice's element type.
func Append(s Value, x ...Value) Value {
	s.mustBe(Slice)
	s, i0, i1 := grow(s, len(x))
	for i, j := i0, 0; i < i1; i, j = i+1, j+1 {
		s.Index(i).Set(x[j])
	}
	return s
}

// AppendSlice appends a slice t to a slice s and returns the resulting slice.
// The slices s and t must have the same element type.
func AppendSlice(s, t Value) Value {
	s.mustBe(Slice)
	t.mustBe(Slice)
	typesMustMatch("reflect.AppendSlice", s.Type().Elem(), t.Type().Elem())
	s, i0, i1 := grow(s, t.Len())
	Copy(s.Slice(i0, i1), t)
	return s
}

// Copy copies the contents of src into dst until either
// dst has been filled or src has been exhausted.
// It returns the number of elements copied.
// Dst and src each must have kind Slice or Array, and
// dst and src must have the same element type.
//
// As a special case, src can have kind String if the element type of dst is kind Uint8.
func Copy(dst, src Value) int {
	dk := dst.kind()
	if dk != Array && dk != Slice {
		panic(&ValueError{"reflect.Copy", dk})
	}
	if dk == Array {
		dst.mustBeAssignable()
	}
	dst.mustBeExported()

	sk := src.kind()
	var stringCopy bool
	if sk != Array && sk != Slice {
		stringCopy = sk == String && dst.typ.Elem().Kind() == Uint8
		if !stringCopy {
			panic(&ValueError{"reflect.Copy", sk})
		}
	}
	src.mustBeExported()

	de := dst.typ.Elem()
	if !stringCopy {
		se := src.typ.Elem()
		typesMustMatch("reflect.Copy", de, se)
	}

	var ds, ss sliceHeader
	if dk == Array {
		ds.Data = dst.ptr
		ds.Len = dst.Len()
		ds.Cap = ds.Len
	} else {
		ds = *(*sliceHeader)(dst.ptr)
	}
	if sk == Array {
		ss.Data = src.ptr
		ss.Len = src.Len()
		ss.Cap = ss.Len
	} else if sk == Slice {
		ss = *(*sliceHeader)(src.ptr)
	} else {
		sh := *(*stringHeader)(src.ptr)
		ss.Data = sh.Data
		ss.Len = sh.Len
		ss.Cap = sh.Len
	}

	return typedslicecopy(de.common(), ds, ss)
}

// A runtimeSelect is a single case passed to rselect.
// This must match ../runtime/select.go:/runtimeSelect
type runtimeSelect struct {
	dir SelectDir    *  SelectSend, SelectRecv or SelectDefault
	typ *rtype       *  channel type
	ch  unsafe.Pointer // channel
	val unsafe.Pointer // ptr to data (SendDir) or ptr to receive buffer (RecvDir)
}

// rselect runs a select. It returns the index of the chosen case.
// If the case was a receive, val is filled in with the received value.
// The conventional OK bool indicates whether the receive corresponds
// to a sent value.
//go:noescape
func rselect([]runtimeSelect) (chosen int, recvOK bool)

// A SelectDir describes the communication direction of a select case.
type SelectDir int

// NOTE: These values must match ../runtime/select.go:/selectDir.

const (
	_             SelectDir = iota
	SelectSend              // case Chan <- Send
	SelectRecv              // case <-Chan:
	SelectDefault           // default
)

// A SelectCase describes a single case in a select operation.
// The kind of case depends on Dir, the communication direction.
//
// If Dir is SelectDefault, the case represents a default case.
// Chan and Send must be zero Values.
//
// If Dir is SelectSend, the case represents a send operation.
// Normally Chan's underlying value must be a channel, and Send's underlying value must be
// assignable to the channel's element type. As a special case, if Chan is a zero Value,
// then the case is ignored, and the field Send will also be ignored and may be either zero
// or non-zero.
//
// If Dir is SelectRecv, the case represents a receive operation.
// Normally Chan's underlying value must be a channel and Send must be a zero Value.
// If Chan is a zero Value, then the case is ignored, but Send must still be a zero Value.
// When a receive operation is selected, the received Value is returned by Select.
//
type SelectCase struct {
	Dir  SelectDir // direction of case
	Chan Value     // channel to use (for send or receive)
	Send Value     // value to send (for send)
}

// Select executes a select operation described by the list of cases.
// Like the Go select statement, it blocks until at least one of the cases
// can proceed, makes a uniform pseudo-random choice,
// and then executes that case. It returns the index of the chosen case
// and, if that case was a receive operation, the value received and a
// boolean indicating whether the value corresponds to a send on the channel
// (as opposed to a zero value received because the channel is closed).
func Select(cases []SelectCase) (chosen int, recv Value, recvOK bool) {
	// NOTE: Do not trust that caller is not modifying cases data underfoot.
	// The range is safe because the caller cannot modify our copy of the len
	// and each iteration makes its own copy of the value c.
	runcases := make([]runtimeSelect, len(cases))
	haveDefault := false
	for i, c := range cases {
		rc := &runcases[i]
		rc.dir = c.Dir
		switch c.Dir {
		default:
			panic("reflect.Select: invalid Dir")

		case SelectDefault: // default
			if haveDefault {
				panic("reflect.Select: multiple default cases")
			}
			haveDefault = true
			if c.Chan.IsValid() {
				panic("reflect.Select: default case has Chan value")
			}
			if c.Send.IsValid() {
				panic("reflect.Select: default case has Send value")
			}

		case SelectSend:
			ch := c.Chan
			if !ch.IsValid() {
				break
			}
			ch.mustBe(Chan)
			ch.mustBeExported()
			tt := (*chanType)(unsafe.Pointer(ch.typ))
			if ChanDir(tt.dir)&SendDir == 0 {
				panic("reflect.Select: SendDir case using recv-only channel")
			}
			rc.ch = ch.pointer()
			rc.typ = &tt.rtype
			v := c.Send
			if !v.IsValid() {
				panic("reflect.Select: SendDir case missing Send value")
			}
			v.mustBeExported()
			v = v.assignTo("reflect.Select", tt.elem, nil)
			if v.flag&flagIndir != 0 {
				rc.val = v.ptr
			} else {
				rc.val = unsafe.Pointer(&v.ptr)
			}

		case SelectRecv:
			if c.Send.IsValid() {
				panic("reflect.Select: RecvDir case has Send value")
			}
			ch := c.Chan
			if !ch.IsValid() {
				break
			}
			ch.mustBe(Chan)
			ch.mustBeExported()
			tt := (*chanType)(unsafe.Pointer(ch.typ))
			if ChanDir(tt.dir)&RecvDir == 0 {
				panic("reflect.Select: RecvDir case using send-only channel")
			}
			rc.ch = ch.pointer()
			rc.typ = &tt.rtype
			rc.val = unsafe_New(tt.elem)
		}
	}

	chosen, recvOK = rselect(runcases)
	if runcases[chosen].dir == SelectRecv {
		tt := (*chanType)(unsafe.Pointer(runcases[chosen].typ))
		t := tt.elem
		p := runcases[chosen].val
		fl := flag(t.Kind())
		if ifaceIndir(t) {
			recv = Value{t, p, fl | flagIndir}
		} else {
			recv = Value{t, *(*unsafe.Pointer)(p), fl}
		}
	}
	return chosen, recv, recvOK
}

/*
 * constructors
 */

// implemented in package runtime
func unsafe_New(*rtype) unsafe.Pointer
func unsafe_NewArray(*rtype, int) unsafe.Pointer

// MakeSlice creates a new zero-initialized slice value
// for the specified slice type, length, and capacity.
func MakeSlice(typ Type, len, cap int) Value {
	if typ.Kind() != Slice {
		panic("reflect.MakeSlice of non-slice type")
	}
	if len < 0 {
		panic("reflect.MakeSlice: negative len")
	}
	if cap < 0 {
		panic("reflect.MakeSlice: negative cap")
	}
	if len > cap {
		panic("reflect.MakeSlice: len > cap")
	}

	s := sliceHeader{unsafe_NewArray(typ.Elem().(*rtype), cap), len, cap}
	return Value{typ.(*rtype), unsafe.Pointer(&s), flagIndir | flag(Slice)}
}

// MakeChan creates a new channel with the specified type and buffer size.
func MakeChan(typ Type, buffer int) Value {
	if typ.Kind() != Chan {
		panic("reflect.MakeChan of non-chan type")
	}
	if buffer < 0 {
		panic("reflect.MakeChan: negative buffer size")
	}
	if typ.ChanDir() != BothDir {
		panic("reflect.MakeChan: unidirectional channel type")
	}
	t := typ.(*rtype)
	ch := makechan(t, buffer)
	return Value{t, ch, flag(Chan)}
}

// MakeMap creates a new map with the specified type.
func MakeMap(typ Type) Value {
	return MakeMapWithSize(typ, 0)
}

// MakeMapWithSize creates a new map with the specified type
// and initial space for approximately n elements.
func MakeMapWithSize(typ Type, n int) Value {
	if typ.Kind() != Map {
		panic("reflect.MakeMapWithSize of non-map type")
	}
	t := typ.(*rtype)
	m := makemap(t, n)
	return Value{t, m, flag(Map)}
}

// Indirect returns the value that v points to.
// If v is a nil pointer, Indirect returns a zero Value.
// If v is not a pointer, Indirect returns v.
func Indirect(v Value) Value {
	if v.Kind() != Ptr {
		return v
	}
	return v.Elem()
}

// ValueOf returns a new Value initialized to the concrete value
// stored in the interface i. ValueOf(nil) returns the zero Value.
func ValueOf(i interface{}) Value {
	if i == nil {
		return Value{}
	}

	// TODO: Maybe allow contents of a Value to live on the stack.
	// For now we make the contents always escape to the heap. It
	// makes life easier in a few places (see chanrecv/mapassign
	// comment below).
	escapes(i)

	return unpackEface(i)
}

// Zero returns a Value representing the zero value for the specified type.
// The result is different from the zero value of the Value struct,
// which represents no value at all.
// For example, Zero(TypeOf(42)) returns a Value with Kind Int and value 0.
// The returned value is neither addressable nor settable.
func Zero(typ Type) Value {
	if typ == nil {
		panic("reflect: Zero(nil)")
	}
	t := typ.(*rtype)
	fl := flag(t.Kind())
	if ifaceIndir(t) {
		return Value{t, unsafe_New(t), fl | flagIndir}
	}
	return Value{t, nil, fl}
}

// New returns a Value representing a pointer to a new zero value
// for the specified type. That is, the returned Value's Type is PtrTo(typ).
func New(typ Type) Value {
	if typ == nil {
		panic("reflect: New(nil)")
	}
	t := typ.(*rtype)
	ptr := unsafe_New(t)
	fl := flag(Ptr)
	return Value{t.ptrTo(), ptr, fl}
}

// NewAt returns a Value representing a pointer to a value of the
// specified type, using p as that pointer.
func NewAt(typ Type, p unsafe.Pointer) Value {
	fl := flag(Ptr)
	t := typ.(*rtype)
	return Value{t.ptrTo(), p, fl}
}

// assignTo returns a value v that can be assigned directly to typ.
// It panics if v is not assignable to typ.
// For a conversion to an interface type, target is a suggested scratch space to use.
func (v Value) assignTo(context string, dst *rtype, target unsafe.Pointer) Value {
	if v.flag&flagMethod != 0 {
		v = makeMethodValue(context, v)
	}

	switch {
	case directlyAssignable(dst, v.typ):
		// Overwrite type so that they match.
		// Same memory layout, so no harm done.
		fl := v.flag&(flagAddr|flagIndir) | v.flag.ro()
		fl |= flag(dst.Kind())
		return Value{dst, v.ptr, fl}

	case implements(dst, v.typ):
		if target == nil {
			target = unsafe_New(dst)
		}
		if v.Kind() == Interface && v.IsNil() {
			// A nil ReadWriter passed to nil Reader is OK,
			// but using ifaceE2I below will panic.
			// Avoid the panic by returning a nil dst (e.g., Reader) explicitly.
			return Value{dst, nil, flag(Interface)}
		}
		x := valueInterface(v, false)
		if dst.NumMethod() == 0 {
			*(*interface{})(target) = x
		} else {
			ifaceE2I(dst, x, target)
		}
		return Value{dst, target, flagIndir | flag(Interface)}
	}

	// Failed.
	panic(context + ": value of type " + v.typ.String() + " is not assignable to type " + dst.String())
}

// Convert returns the value v converted to type t.
// If the usual Go conversion rules do not allow conversion
// of the value v to type t, Convert panics.
func (v Value) Convert(t Type) Value {
	if v.flag&flagMethod != 0 {
		v = makeMethodValue("Convert", v)
	}
	op := convertOp(t.common(), v.typ)
	if op == nil {
		panic("reflect.Value.Convert: value of type " + v.typ.String() + " cannot be converted to type " + t.String())
	}
	return op(v, t)
}

// convertOp returns the function to convert a value of type src
// to a value of type dst. If the conversion is illegal, convertOp returns nil.
func convertOp(dst, src *rtype) func(Value, Type) Value {
	switch src.Kind() {
	case Int, Int8, Int16, Int32, Int64:
		switch dst.Kind() {
		case Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
			return cvtInt
		case Float32, Float64:
			return cvtIntFloat
		case String:
			return cvtIntString
		}

	case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
		switch dst.Kind() {
		case Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
			return cvtUint
		case Float32, Float64:
			return cvtUintFloat
		case String:
			return cvtUintString
		}

	case Float32, Float64:
		switch dst.Kind() {
		case Int, Int8, Int16, Int32, Int64:
			return cvtFloatInt
		case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
			return cvtFloatUint
		case Float32, Float64:
			return cvtFloat
		}

	case Complex64, Complex128:
		switch dst.Kind() {
		case Complex64, Complex128:
			return cvtComplex
		}

	case String:
		if dst.Kind() == Slice && dst.Elem().PkgPath() == "" {
			switch dst.Elem().Kind() {
			case Uint8:
				return cvtStringBytes
			case Int32:
				return cvtStringRunes
			}
		}

	case Slice:
		if dst.Kind() == String && src.Elem().PkgPath() == "" {
			switch src.Elem().Kind() {
			case Uint8:
				return cvtBytesString
			case Int32:
				return cvtRunesString
			}
		}

	case Chan:
		if dst.Kind() == Chan && specialChannelAssignability(dst, src) {
			return cvtDirect
		}
	}

	// dst and src have same underlying type.
	if haveIdenticalUnderlyingType(dst, src, false) {
		return cvtDirect
	}

	// dst and src are non-defined pointer types with same underlying base type.
	if dst.Kind() == Ptr && dst.Name() == "" &&
		src.Kind() == Ptr && src.Name() == "" &&
		haveIdenticalUnderlyingType(dst.Elem().common(), src.Elem().common(), false) {
		return cvtDirect
	}

	if implements(dst, src) {
		if src.Kind() == Interface {
			return cvtI2I
		}
		return cvtT2I
	}

	return nil
}

// makeInt returns a Value of type t equal to bits (possibly truncated),
// where t is a signed or unsigned int type.
func makeInt(f flag, bits uint64, t Type) Value {
	typ := t.common()
	ptr := unsafe_New(typ)
	switch typ.size {
	case 1:
		*(*uint8)(ptr) = uint8(bits)
	case 2:
		*(*uint16)(ptr) = uint16(bits)
	case 4:
		*(*uint32)(ptr) = uint32(bits)
	case 8:
		*(*uint64)(ptr) = bits
	}
	return Value{typ, ptr, f | flagIndir | flag(typ.Kind())}
}

// makeFloat returns a Value of type t equal to v (possibly truncated to float32),
// where t is a float32 or float64 type.
func makeFloat(f flag, v float64, t Type) Value {
	typ := t.common()
	ptr := unsafe_New(typ)
	switch typ.size {
	case 4:
		*(*float32)(ptr) = float32(v)
	case 8:
		*(*float64)(ptr) = v
	}
	return Value{typ, ptr, f | flagIndir | flag(typ.Kind())}
}

// makeComplex returns a Value of type t equal to v (possibly truncated to complex64),
// where t is a complex64 or complex128 type.
func makeComplex(f flag, v complex128, t Type) Value {
	typ := t.common()
	ptr := unsafe_New(typ)
	switch typ.size {
	case 8:
		*(*complex64)(ptr) = complex64(v)
	case 16:
		*(*complex128)(ptr) = v
	}
	return Value{typ, ptr, f | flagIndir | flag(typ.Kind())}
}

func makeString(f flag, v string, t Type) Value {
	ret := New(t).Elem()
	ret.SetString(v)
	ret.flag = ret.flag&^flagAddr | f
	return ret
}

func makeBytes(f flag, v []byte, t Type) Value {
	ret := New(t).Elem()
	ret.SetBytes(v)
	ret.flag = ret.flag&^flagAddr | f
	return ret
}

func makeRunes(f flag, v []rune, t Type) Value {
	ret := New(t).Elem()
	ret.setRunes(v)
	ret.flag = ret.flag&^flagAddr | f
	return ret
}

// These conversion functions are returned by convertOp
// for classes of conversions. For example, the first function, cvtInt,
// takes any value v of signed int type and returns the value converted
// to type t, where t is any signed or unsigned int type.

// convertOp: intXX -> [u]intXX
func cvtInt(v Value, t Type) Value {
	return makeInt(v.flag.ro(), uint64(v.Int()), t)
}

// convertOp: uintXX -> [u]intXX
func cvtUint(v Value, t Type) Value {
	return makeInt(v.flag.ro(), v.Uint(), t)
}

// convertOp: floatXX -> intXX
func cvtFloatInt(v Value, t Type) Value {
	return makeInt(v.flag.ro(), uint64(int64(v.Float())), t)
}

// convertOp: floatXX -> uintXX
func cvtFloatUint(v Value, t Type) Value {
	return makeInt(v.flag.ro(), uint64(v.Float()), t)
}

// convertOp: intXX -> floatXX
func cvtIntFloat(v Value, t Type) Value {
	return makeFloat(v.flag.ro(), float64(v.Int()), t)
}

// convertOp: uintXX -> floatXX
func cvtUintFloat(v Value, t Type) Value {
	return makeFloat(v.flag.ro(), float64(v.Uint()), t)
}

// convertOp: floatXX -> floatXX
func cvtFloat(v Value, t Type) Value {
	return makeFloat(v.flag.ro(), v.Float(), t)
}

// convertOp: complexXX -> complexXX
func cvtComplex(v Value, t Type) Value {
	return makeComplex(v.flag.ro(), v.Complex(), t)
}

// convertOp: intXX -> string
func cvtIntString(v Value, t Type) Value {
	return makeString(v.flag.ro(), string(v.Int()), t)
}

// convertOp: uintXX -> string
func cvtUintString(v Value, t Type) Value {
	return makeString(v.flag.ro(), string(v.Uint()), t)
}

// convertOp: []byte -> string
func cvtBytesString(v Value, t Type) Value {
	return makeString(v.flag.ro(), string(v.Bytes()), t)
}

// convertOp: string -> []byte
func cvtStringBytes(v Value, t Type) Value {
	return makeBytes(v.flag.ro(), []byte(v.String()), t)
}

// convertOp: []rune -> string
func cvtRunesString(v Value, t Type) Value {
	return makeString(v.flag.ro(), string(v.runes()), t)
}

// convertOp: string -> []rune
func cvtStringRunes(v Value, t Type) Value {
	return makeRunes(v.flag.ro(), []rune(v.String()), t)
}

// convertOp: direct copy
func cvtDirect(v Value, typ Type) Value {
	f := v.flag
	t := typ.common()
	ptr := v.ptr
	if f&flagAddr != 0 {
		// indirect, mutable word - make a copy
		c := unsafe_New(t)
		typedmemmove(t, c, ptr)
		ptr = c
		f &^= flagAddr
	}
	return Value{t, ptr, v.flag.ro() | f} // v.flag.ro()|f == f?
}

// convertOp: concrete -> interface
func cvtT2I(v Value, typ Type) Value {
	target := unsafe_New(typ.common())
	x := valueInterface(v, false)
	if typ.NumMethod() == 0 {
		*(*interface{})(target) = x
	} else {
		ifaceE2I(typ.(*rtype), x, target)
	}
	return Value{typ.common(), target, v.flag.ro() | flagIndir | flag(Interface)}
}

// convertOp: interface -> interface
func cvtI2I(v Value, typ Type) Value {
	if v.IsNil() {
		ret := Zero(typ)
		ret.flag |= v.flag.ro()
		return ret
	}
	return cvtT2I(v.Elem(), typ)
}

// implemented in ../runtime
func chancap(ch unsafe.Pointer) int
func chanclose(ch unsafe.Pointer)
func chanlen(ch unsafe.Pointer) int

// Note: some of the noescape annotations below are technically a lie,
// but safe in the context of this package. Functions like chansend
// and mapassign don't escape the referent, but may escape anything
// the referent points to (they do shallow copies of the referent).
// It is safe in this package because the referent may only point
// to something a Value may point to, and that is always in the heap
// (due to the escapes() call in ValueOf).

//go:noescape
func chanrecv(ch unsafe.Pointer, nb bool, val unsafe.Pointer) (selected, received bool)

//go:noescape
func chansend(ch unsafe.Pointer, val unsafe.Pointer, nb bool) bool

func makechan(typ *rtype, size int) (ch unsafe.Pointer)
func makemap(t *rtype, cap int) (m unsafe.Pointer)

//go:noescape
func mapaccess(t *rtype, m unsafe.Pointer, key unsafe.Pointer) (val unsafe.Pointer)

//go:noescape
func mapassign(t *rtype, m unsafe.Pointer, key, val unsafe.Pointer)

//go:noescape
func mapdelete(t *rtype, m unsafe.Pointer, key unsafe.Pointer)

// m escapes into the return value, but the caller of mapiterinit
// doesn't let the return value escape.
//go:noescape
func mapiterinit(t *rtype, m unsafe.Pointer) unsafe.Pointer

//go:noescape
func mapiterkey(it unsafe.Pointer) (key unsafe.Pointer)

//go:noescape
func mapiterelem(it unsafe.Pointer) (elem unsafe.Pointer)

//go:noescape
func mapiternext(it unsafe.Pointer)

//go:noescape
func maplen(m unsafe.Pointer) int

// call calls fn with a copy of the n argument bytes pointed at by arg.
// After fn returns, reflectcall copies n-retoffset result bytes
// back into arg+retoffset before returning. If copying result bytes back,
// the caller must pass the argument frame type as argtype, so that
// call can execute appropriate write barriers during the copy.
//
//go:linkname call runtime.reflectcall
func call(argtype *rtype, fn, arg unsafe.Pointer, n uint32, retoffset uint32)

func ifaceE2I(t *rtype, src interface{}, dst unsafe.Pointer)

// memmove copies size bytes to dst from src. No write barriers are used.
//go:noescape
func memmove(dst, src unsafe.Pointer, size uintptr)

// typedmemmove copies a value of type t to dst from src.
//go:noescape
func typedmemmove(t *rtype, dst, src unsafe.Pointer)

// typedmemmovepartial is like typedmemmove but assumes that
// dst and src point off bytes into the value and only copies size bytes.
//go:noescape
func typedmemmovepartial(t *rtype, dst, src unsafe.Pointer, off, size uintptr)

// typedmemclr zeros the value at ptr of type t.
//go:noescape
func typedmemclr(t *rtype, ptr unsafe.Pointer)

// typedmemclrpartial is like typedmemclr but assumes that
// dst points off bytes into the value and only clears size bytes.
//go:noescape
func typedmemclrpartial(t *rtype, ptr unsafe.Pointer, off, size uintptr)

// typedslicecopy copies a slice of elemType values from src to dst,
// returning the number of elements copied.
//go:noescape
func typedslicecopy(elemType *rtype, dst, src sliceHeader) int

//go:noescape
func typehash(t *rtype, p unsafe.Pointer, h uintptr) uintptr

// Dummy annotation marking that the value x escapes,
// for use in cases where the reflect code is so clever that
// the compiler cannot follow.
func escapes(x interface{}) {
	if dummy.b {
		dummy.x = x
	}
}

var dummy struct {
	b bool
	x interface{}
}
```go