reflect包基础类型是Type，其主要实现是rtype，在rtype下会有基于种类型的实现



```go

// 包reflect实现了运行时反射，从而允许程序处理任意类型的对象。典型的用法是使用静态类型
// interface{}获取一个值，并通过调用TypeOf来提取其动态类型信息，它返回一个Type类型。
// Package reflect implements run-time reflection, allowing a program to
// manipulate objects with arbitrary types. The typical use is to take a value
// with static type interface{} and extract its dynamic type information by
// calling TypeOf, which returns a Type.
//
// 调用ValueOf返回一个代表运行时数据的Value。
// Zero也是一种类型，并返回一个表示该类型的零值。
// A call to ValueOf returns a Value representing the run-time data.
// Zero takes a Type and returns a Value representing a zero value
// for that type.
//
// 有关Go语言中反射的介绍，请参见“反射规则”：https：//golang.org/doc/articles/laws_of_reflection.html
// See "The Laws of Reflection" for an introduction to reflection in Go:
// https://golang.org/doc/articles/laws_of_reflection.html
package reflect

import (
	"strconv"
	"sync"
	"unicode"
	"unicode/utf8"
	"unsafe"
)

// Type是Go类型的表示。
//
// 并非所有方法都适用于所有类型。在每种方法的限制都在文档中注明了（如果有）。
// 在调用特定于种类的方法之前，使用Kind方法找出类型。调用不适合该类型的方法会导致运行时恐慌。
//
// 类型值是可比较的，例如==运算符，因此它们可用作映射键。
// 如果两个Type值表示相同的类型，则它们相等
// Type is the representation of a Go type.
//
// Not all methods apply to all kinds of types. Restrictions,
// if any, are noted in the documentation for each method.
// Use the Kind method to find out the kind of type before
// calling kind-specific methods. Calling a method
// inappropriate to the kind of type causes a run-time panic.
//
// Type values are comparable, such as with the == operator,
// so they can be used as map keys.
// Two Type values are equal if they represent identical types.
type Type interface {
    // 适用于所有类型的方法。
	// Methods applicable to all types.

    // 当在内存中分配时，Align返回此类型的值的对齐方式（以字节为单位）。
	// Align returns the alignment in bytes of a value of
	// this type when allocated in memory.
	/**
	 * @return Question: 返回值代表什么?
	 * @date 2020-03-15 19:43:54
	 **/
	Align() int

    // 当用作结构体中的字段时，FieldAlign返回此类型值的对齐方式（以字节为单位）。
	// FieldAlign returns the alignment in bytes of a value of
	// this type when used as a field in a struct.
	/**
	 * @return
	 * @date 2020-03-15 19:45:52
	 **/
	FieldAlign() int

    // 方法返回类型的方法集中的第i个方法。如果i不在[0，NumMethod())范围内，引发恐慌。
    //
    // 对于非接口类型T或*T，返回的Method的Type和Func字段描述了一个函数，其第一个参数为接收者。
    //
    // 对于接口类型，返回的Method的Type字段给出方法签名，没有接收者，并且Func字段为nil。
    //
    // 仅可访问导出的方法，并且它们按字典顺序排序
	// Method returns the i'th method in the type's method set.
	// It panics if i is not in the range [0, NumMethod()).
	//
	// For a non-interface type T or *T, the returned Method's Type and Func
	// fields describe a function whose first argument is the receiver.
	//
	// For an interface type, the returned Method's Type field gives the
	// method signature, without a receiver, and the Func field is nil.
	//
	// Only exported methods are accessible and they are sorted in
	// lexicographic order.
	/**
	 * @param 第i个方法
	 * @return 方法描述信息
	 * @date 2020-03-15 19:49:07
	 **/
	Method(int) Method

    // MethodByName返回在类型的方法集中具有该名称的方法，以及一个布尔值，指示是否找到该方法。
    //
    // 对于非接口类型T或*T，返回的Method的Type和Func字段描述了一个函数，其第一个参数为接收者。
    //
    // 对于接口类型，返回的Method的Type字段给出了方法签名，没有接收者，而Func字段为nil
    //
	// MethodByName returns the method with that name in the type's
	// method set and a boolean indicating if the method was found.
	//
	// For a non-interface type T or *T, the returned Method's Type and Func
	// fields describe a function whose first argument is the receiver.
	//
	// For an interface type, the returned Method's Type field gives the
	// method signature, without a receiver, and the Func field is nil.
	/**
	 * @param string 方法名
	 * @return Method 方法结构体
	 * @return bool true: 表示找到
	 * @date 2020-03-16 14:04:12
	 **/
	MethodByName(string) (Method, bool)

	// NumMethod returns the number of exported methods in the type's method set.
	/**
	 * NumMethod返回类型的方法集中导出的方法的数量。
	 * @return int 方法集中导出的方法的数量
	 * @date 2020-03-16 17:02:19
	 **/
	NumMethod() int

	// Name returns the type's name within its package for a defined type.
	// For other (non-defined) types it returns the empty string.
	/**
	 * Name返回其包中已定义类型的类型名称。 对于其他（未定义）类型，它返回空字符串。
	 * @return string 包中已定义类型的类型名称
	 * @date 2020-03-16 17:04:06
	 **/
	Name() string

	// PkgPath returns a defined type's package path, that is, the import path
	// that uniquely identifies the package, such as "encoding/base64".
	// If the type was predeclared (string, error) or not defined (*T, struct{},
	// []int, or A where A is an alias for a non-defined type), the package path
	// will be the empty string.
	/**
	 * PkgPath返回定义的类型的包路径，即唯一标识包的导入路径，例如"encoding/base64"。
     * 如果类型是预先声明的（字符串，错误）或未定义（*T，struct{}，[]int或A，其中A是未定义类型的别名），则包路径将为空字符串。
	 * @return string 回定义的类型的包路径
	 * @date 2020-03-16 17:04:49
	 **/
	PkgPath() string

	// Size returns the number of bytes needed to store
	// a value of the given type; it is analogous to unsafe.Sizeof.
	/**
	 * Size返回存储给定类型的值所需的字节数；它类似于unsafe.Sizeof。
	 * @return uintptr 存储给定类型的值所需的字节数
	 * @date 2020-03-16 17:06:39
	 **/
	Size() uintptr

	// String returns a string representation of the type.
	// The string representation may use shortened package names
	// (e.g., base64 instead of "encoding/base64") and is not
	// guaranteed to be unique among types. To test for type identity,
	// compare the Types directly.
	/**
	 * String返回该类型的字符串表示形式。字符串表示形式可以使用缩短的包名称（例如，使用base64代替"encoding/base64"），
	 * 并且不能保证类型之间的唯一性。要测试类型标识，请直接比较类型。
	 * @return string 类型的字符串表示形式
	 * @date 2020-03-16 17:07:52
	 **/
	String() string

	// Kind returns the specific kind of this type.
	/**
	 * Kind返回此类型的特定类型。
	 * @return Kind 此类型的特定类型
	 * @date 2020-03-16 17:08:43
	 **/
	Kind() Kind

	// Implements reports whether the type implements the interface type u.
	/**
	 * 实现报告类型是否实现接口类型u。
	 * @param u 接口类型
	 * @return true: 实现了接口类型u
	 * @date 2020-03-16 17:09:43
	 **/
	Implements(u Type) bool

	// AssignableTo reports whether a value of the type is assignable to type u.
	/**
	 * AssignableTo报告类型的值是否可分配给类型u。
	 * @param u 任意类型
	 * @return true: 类型的值是可分配给类型u
	 * @date 2020-03-16 17:10:28
	 **/
	AssignableTo(u Type) bool

	// ConvertibleTo reports whether a value of the type is convertible to type u.
	/**
	 * ConvertibleTo报告该类型的值是否可转换为u类型。
	 * @param u 任意类型
	 * @return 类型的值是否可转换为u类型。
	 * @date 2020-03-16 17:11:44
	 **/
	ConvertibleTo(u Type) bool

	// Comparable reports whether values of this type are comparable.
	/**
	 * Comparable较报告此类型的值是否可比较。
	 * @return true 此类型的值是可比较
	 * @date 2020-03-16 17:12:22
	 **/
	Comparable() bool

    /**
     * 方法仅适用于某些类型，具体取决于种类。每种类型允许使用的方法是：
     * Int*, Uint*, Float*, Complex*: Bits
     * Array: Elem, Len
     * Chan: ChanDir, Elem
     * Func: In, NumIn, Out, NumOut, IsVariadic.
     * Map: Key, Elem
     * Ptr: Elem
     * Slice: Elem
     * Struct: Field, FieldByIndex, FieldByName, FieldByNameFunc, NumField
     **/
	// Methods applicable only to some types, depending on Kind.
	// The methods allowed for each kind are:
	//
	//	Int*, Uint*, Float*, Complex*: Bits
	//	Array: Elem, Len
	//	Chan: ChanDir, Elem
	//	Func: In, NumIn, Out, NumOut, IsVariadic.
	//	Map: Key, Elem
	//	Ptr: Elem
	//	Slice: Elem
	//	Struct: Field, FieldByIndex, FieldByName, FieldByNameFunc, NumField

	// Bits returns the size of the type in bits.
	// It panics if the type's Kind is not one of the
	// sized or unsized Int, Uint, Float, or Complex kinds.
	/**
	 * Bits返回以位为单位的类型的大小。 如果类型的Kind不是大小或大小不完整的Int，Uint，Float或Complex类型之一，它就会感到恐慌。
	 * @return 位为单位的类型的大小
	 * @date 2020-03-16 17:16:37
	 **/
	Bits() int

	// ChanDir returns a channel type's direction.
	// It panics if the type's Kind is not Chan.
	/**
	 * ChanDir返回通道类型的方向。如果类型的种类不是Chan，则会引起恐慌。
	 * @return 通道类型的方向
	 * @date 2020-03-17 08:05:51
	 **/
	ChanDir() ChanDir

	// IsVariadic reports whether a function type's final input parameter
	// is a "..." parameter. If so, t.In(t.NumIn() - 1) returns the parameter's
	// implicit actual type []T.
	//
	// For concreteness, if t represents func(x int, y ... float64), then
	//
	//	t.NumIn() == 2
	//	t.In(0) is the reflect.Type for "int"
	//	t.In(1) is the reflect.Type for "[]float64"
	//	t.IsVariadic() == true
	//
	// IsVariadic panics if the type's Kind is not Func.
	/**
	 * IsVariadic报告函数类型的最终输入参数是否为“...”参数。如果是这样，则t.In(t.NumIn()-1)返回参数的隐式实际类型[]T。
     *
     * 具体来说，如果t代表func(x int，y ... float64)，则
     *
     * t.NumIn()== 2
     * t.In(0)是“ int”的reflect.Type
     * t.In(1)是“ [] float64”的reflect.Type
     * t.IsVariadic()== true
     *
     * 如果类型的Kind不是Func，则为IsVariadic恐慌。
	 * @return 是否是可变参数类型，true：是
	 * @date 2020-03-17 08:09:56
	 **/
	IsVariadic() bool

	// Elem returns a type's element type.
	// It panics if the type's Kind is not Array, Chan, Map, Ptr, or Slice.
	/**
	 * Elem返回类型的元素类型。如果类型的Kind不是Array，Chan，Map，Ptr或Slice，它会感到恐慌。
	 * @return 元素类型
	 * @date 2020-03-17 08:12:23
	 **/
	Elem() Type

	// Field returns a struct type's i'th field.
	// It panics if the type's Kind is not Struct.
	// It panics if i is not in the range [0, NumField()).
	/**
	 * Field返回结构类型的第i个字段。如果类型的Kind不是Struct，它会感到恐慌。如果i不在[0，NumField())范围内，它引起恐慌。
	 * @param i 第i个字段
	 * @return 结体字段类型
	 * @date 2020-03-17 08:13:06
	 **/
	Field(i int) StructField

	// FieldByIndex returns the nested field corresponding
	// to the index sequence. It is equivalent to calling Field
	// successively for each index i.
	// It panics if the type's Kind is not Struct.
	/**
	 * FieldByIndex返回与索引序列相对应的嵌套字段。等效于为每个索引i依次调用Field。如果类型的Kind不是Struct，它会引起恐慌。
	 * @param index 字段索引数组
	 * @return 结体字段类型
	 * @date 2020-03-17 08:15:36
	 **/
	FieldByIndex(index []int) StructField

	// FieldByName returns the struct field with the given name
	// and a boolean indicating if the field was found.
	/**
	 * FieldByName返回具有给定名称的struct字段和一个布尔值，指示是否找到了该字段。
	 * @param name 字符段名称
	 * @return StructField 结体字段类型
	 * @return bool true: 找到了该字段。false：否
	 * @date 2020-03-17 08:16:43
	 **/
	FieldByName(name string) (StructField, bool)

	// FieldByNameFunc returns the struct field with a name
	// that satisfies the match function and a boolean indicating if
	// the field was found.
	//
	// FieldByNameFunc considers the fields in the struct itself
	// and then the fields in any embedded structs, in breadth first order,
	// stopping at the shallowest nesting depth containing one or more
	// fields satisfying the match function. If multiple fields at that depth
	// satisfy the match function, they cancel each other
	// and FieldByNameFunc returns no match.
	// This behavior mirrors Go's handling of name lookup in
	// structs containing embedded fields.
	/**
	 * FieldByNameFunc返回具有满足match函数名称的struct字段和一个布尔值，指示是否找到该字段。
     *
     * FieldByNameFunc会先考虑结构本身中的字段，然后再考虑所有嵌入式结构中的字段，并以广度优先
     * 的顺序停在最浅的嵌套深度，其中包含一个或多个满足match函数的字段。如果该深度处的多个字段满
     * 足匹配功能，则它们会相互取消，并且FieldByNameFunc不返回匹配项。此行为反映了Go在包含嵌入
     * 式字段的结构中对名称查找的处理。
	 * @param match 根据名称进行匹配的函数
	 * @return StructField 结体字段类型
	 * @return bool true: 找到了该字段。false：否
	 * @date 2020-03-17 08:21:04
	 **/
	FieldByNameFunc(match func(string) bool) (StructField, bool)

	// In returns the type of a function type's i'th input parameter.
	// It panics if the type's Kind is not Func.
	// It panics if i is not in the range [0, NumIn()).
	/**
	 * In返回函数类型的第i个输入参数的类型。如果类型的Kind不是Func，它会感到恐慌。如果i不在[0, NumIn())范围内，它将发生恐慌。
	 * @param i 第i个参数
	 * @return 参数的类型
	 * @date 2020-03-17 08:24:45
	 **/
	In(i int) Type

	// Key returns a map type's key type.
	// It panics if the type's Kind is not Map.
	/**
	 * Key返回Map类型的键类型。如果类型的Kind不是Map，则会发生恐慌。
	 * @return 键类型
	 * @date 2020-03-17 08:26:07
	 **/
	Key() Type

	// Len returns an array type's length.
	// It panics if the type's Kind is not Array.
	/**
	 * Len返回数组类型的长度。如果类型的Kind不是Array，它会惊慌。
	 * @return 数组类型的长度
	 * @date 2020-03-17 08:27:12
	 **/
	Len() int

	// NumField returns a struct type's field count.
	// It panics if the type's Kind is not Struct.
	/**
	 * NumField返回结构类型的字段数。如果类型的Kind不是Struct，它会引起恐慌。
	 * @return 类型的字段数
	 * @date 2020-03-17 08:29:50
	 **/
	NumField() int

	// NumIn returns a function type's input parameter count.
	// It panics if the type's Kind is not Func.
	/**
	 * NumIn返回函数类型的输入参数个数。如果类型的Kind不是Func，它会引起恐慌。
	 * @return 输入参数个数
	 * @date 2020-03-17 08:30:48
	 **/
	NumIn() int

	// NumOut returns a function type's output parameter count.
	// It panics if the type's Kind is not Func.
    /**
	 * NumIn返回函数类型的输出参数个数。如果类型的Kind不是Func，它会引起恐慌。
	 * @return 输出参数个数
	 * @date 2020-03-17 08:30:48
	 **/
	NumOut() int

	// Out returns the type of a function type's i'th output parameter.
	// It panics if the type's Kind is not Func.
	// It panics if i is not in the range [0, NumOut()).
	/**
	 * Out返回函数类型的第i个输出参数的类型。如果类型的Kind不是Func，它会引起恐慌。如果我不在[0, NumOut())范围内，它会引起恐慌。
	 * @param 第i个输出参数
	 * @return 第i个输出参数类型
	 * @date 2020-03-17 08:32:49
	 **/
	Out(i int) Type
    
    /**
     * 此方法获取大多数值的一些通用实现
     * @return *rtype rtype是大多数值的通用实现。它嵌入在其他结构类型中。
     * @date 2020-03-17 08:35:18 
     **/
	common() *rtype
	/**
	 * 此方法获取值的非通用实现
	 * @return *uncommonType uncommonType仅对于定义的类型或带有方法的类型存在
	 * @date 2020-03-17 08:36:42 
	 **/
	uncommon() *uncommonType
}

/**
 * BUG（rsc）：FieldByName和相关函数将结构字段名称视为相等，即使名称相同，即使它们是源自不同包的未导出名称也是如此。
 * 这样做的实际效果是，如果结构类型t包含多个名为x的字段（从不同的程序包中嵌入的），则t.FieldByName("x")的结果定义不明确。
 * FieldByName可能返回名为x的字段之一，或者可能报告没有字段。有关更多详细信息，请参见https://golang.org/issue/4876。
 * 示例：https://play.golang.org/p/WTj5d06CQ3
 **/
// BUG(rsc): FieldByName and related functions consider struct field names to be equal
// if the names are equal, even if they are unexported names originating
// in different packages. The practical effect of this is that the result of
// t.FieldByName("x") is not well defined if the struct type t contains
// multiple fields named x (embedded from different packages).
// FieldByName may return one of the fields named x or may report that there are none.
// See https://golang.org/issue/4876 for more details.

/**
 * 这些数据结构是编译器已知的（../../cmd/internal/gc/reflect.go）。
 * 少部分被../runtime/type.go已知并且传递给调试器。
 * 他们都被../runtime/type.go已知
 *
 */
/*
 * These data structures are known to the compiler (../../cmd/internal/gc/reflect.go).
 * A few are known to ../runtime/type.go to convey to debuggers.
 * They are also known to ../runtime/type.go.
 */

/**
 * Kind代表类型所代表的特定类型的种类。零种类不是有效种类。
 **/
// A Kind represents the specific kind of type that a Type represents.
// The zero Kind is not a valid kind.
type Kind uint

const (
	Invalid Kind = iota
	Bool
	Int
	Int8
	Int16
	Int32
	Int64
	Uint
	Uint8
	Uint16
	Uint32
	Uint64
	Uintptr
	Float32
	Float64
	Complex64
	Complex128
	Array
	Chan
	Func
	Interface
	Map
	Ptr
	Slice
	String
	Struct
	UnsafePointer
)

// tflag is used by an rtype to signal what extra type information is
// available in the memory directly following the rtype value.
//
// tflag values must be kept in sync with copies in:
//	cmd/compile/internal/gc/reflect.go
//	cmd/link/internal/ld/decodesym.go
//	runtime/type.go
/**
 * rtype使用tflag来指示紧随rtype值之后在内存中还有哪些额外的类型信息。
 *
 * tflag值必须与以下副本保持同步：
 * cmd/compile/internal/gc/reflect.go
 * cmd/link/internal/ld/decodesym.go
 * runtime/type.go
 **/
type tflag uint8

const (
	// tflagUncommon means that there is a pointer, *uncommonType,
	// just beyond the outer type structure.
	//
	// For example, if t.Kind() == Struct and t.tflag&tflagUncommon != 0,
	// then t has uncommonType data and it can be accessed as:
	//
	//	type tUncommon struct {
	//		structType
	//		u uncommonType
	//	}
	//	u := &(*tUncommon)(unsafe.Pointer(t)).u
	/**
	 * tflagUncommon意味着在外部类型结构之后，还有一个指针* uncommonType。
     *
     * 例如，如果t.Kind()== Struct且t.tflag&tflagUncommon != 0，则t具有uncommonType数据，可以按以下方式访问它：
     *  type tUncommon struct {
     *      structType
     *      u uncommonType
     *  }
     *  u := &(*tUncommon)(unsafe.Pointer(t)).u
	 **/
	tflagUncommon tflag = 1 << 0 // 0x00000001 = 1

	// tflagExtraStar means the name in the str field has an
	// extraneous '*' prefix. This is because for most types T in
	// a program, the type *T also exists and reusing the str data
	// saves binary size.
	/**
	 * tflagExtraStar表示str字段中的名称带有多余的“*”前缀。这是因为对于程序中的大多数T类型，
	 * T类型也存在，并且重新使用str数据可节省二进制大小。
	 **/
	tflagExtraStar tflag = 1 << 1 // 0x00000010 = 2

	// tflagNamed means the type has a name.
	/**
	 * tflagNamed表示类型具有名称。
	 **/
	tflagNamed tflag = 1 << 2 // 0x00000100 = 4

	// tflagRegularMemory means that equal and hash functions can treat
	// this type as a single region of t.size bytes.
	/**
	 * tflagRegularMemory意味着equal和hash函数可以将此类型视为t.size字节的单个区域。
	 **/
	tflagRegularMemory tflag = 1 << 3 // 0x00001000 = 8
)

// rtype is the common implementation of most values.
// It is embedded in other struct types.
//
// rtype must be kept in sync with ../runtime/type.go:/^type._type.
/**
 * rtype是大多数值的通用实现。
 * 它嵌入在其他结构类型中。
 *
 * rtype必须与../runtime/type.go:/^type._type保持同步。
 **/
type rtype struct {
	size       uintptr
	ptrdata    uintptr // number of bytes in the type that can contain pointers // 包含指针的类型需要的字节数
	hash       uint32  // hash of type; avoids computation in hash tables // 类型的哈希避免在哈希表中进行计算
	tflag      tflag   // extra type information flags // 额外类型信息标志
	align      uint8   // alignment of variable with this type // 变量与此类型的对齐 Question: 都有哪些值代表什么
	fieldAlign uint8   // alignment of struct field with this type // 结构体字段与此类型的对齐 Question: 都有哪些值代表什么
	kind       uint8   // enumeration for C // C的枚举 Question: 都有哪些值代表什么
	// 比较此类型的比较函数
	// function for comparing objects of this type
	// (ptr to object A, ptr to object B) -> ==?
	equal     func(unsafe.Pointer, unsafe.Pointer) bool
	gcdata    *byte   // garbage collection data // 垃圾收集数据 Question: 会有一些什么样子的数据
	str       nameOff // string form // 字符串形式 Question: 有哪些字符串形式
	ptrToThis typeOff // type for pointer to this type, may be zero // 指向此类型的指针的类型，可以为零
}

// Method on non-interface type
/**
 * 非接口类型的方法
 **/
type method struct {
	name nameOff // name of method // 方法名
	mtyp typeOff // method type (without receiver) // 方法类型（无接收者）
	ifn  textOff // fn used in interface call (one-word receiver) // 接口调用中使用的fn（单字接收器）
	tfn  textOff // fn used for normal method call // fn用于普通方法调用
}

// uncommonType is present only for defined types or types with methods
// (if T is a defined type, the uncommonTypes for T and *T have methods).
// Using a pointer to this struct reduces the overall size required
// to describe a non-defined type with no methods.
/**
 * uncommonType仅对定义的类型或带有方法的类型存在（如果T是定义的类型，则T和*T的uncommonTypes具有方法）。
 * 使用指向此结构的指针可减少描述没有方法的未定义类型所需的总体大小。
 **/
type uncommonType struct {
	pkgPath nameOff // import path; empty for built-in types like int, string // 导入路径；对于内置类型（如int，string）为空
	mcount  uint16  // number of methods // 方法数量
	xcount  uint16  // number of exported methods // 导出的方法数量
	moff    uint32  // offset from this uncommontype to [mcount]method // 从uncommontype到[mcount]method偏移量
	_       uint32  // unused // 未使用
}

// ChanDir represents a channel type's direction.
/**
 * ChanDir表示通道类型的方向。
 * 1: 发送通道
 * 2: 接收通道
 * 3: 双向通道
 **/
type ChanDir int

const (
	RecvDir ChanDir             = 1 << iota // <-chan // 发送通道
	SendDir                                 // chan<- // 接收通道
	BothDir = RecvDir | SendDir             // chan   // 双向通道
)

// arrayType represents a fixed array type.
/**
 * arrayType表示固定大小的数组类型。
 */
type arrayType struct {
	rtype // 通用数据类型
	elem  *rtype // array element type // 数组元素类型
	slice *rtype // slice type         // 切片类型
	len   uintptr                      // 数组类型的长度
}

// chanType represents a channel type.
/**
 * chanType表示通道类型。
 */
type chanType struct {
	rtype // 通用数据类型
	elem *rtype  // channel element type            // 通道元素类型
	dir  uintptr // channel direction (ChanDir)     // 通道方向（ChanDir）
}

// funcType represents a function type.
//
// A *rtype for each in and out parameter is stored in an array that
// directly follows the funcType (and possibly its uncommonType). So
// a function type with one method, one input, and one output is:
//
//	struct {
//		funcType
//		uncommonType
//		[2]*rtype    // [0] is in, [1] is out
//	}
/**
 * funcType表示函数类型。
 *
 * 每个in和out参数的*rtype存储在一个数组中，该数组紧随funcType（可能还有其uncommonType）。
 * 因此，具有一个方法，一个输入和一个输出的函数类型为：
 *  struct {
 *      funcType
 *      uncommonType
 *      [2]*rtype    // [0]是输入参数, [1]输出结果
 *  }
 */
type funcType struct {
	rtype // 通用数据类型
	inCount  uint16 // 输入参数的个数
	outCount uint16 // top bit is set if last input parameter is ... // 输出参数的个数，如果最后一个输入参数为...，则设置最高位
}

// imethod represents a method on an interface type
/**
 * imethod表示接口类型上的方法
 */
type imethod struct {
	name nameOff // name of method // 方法名
	typ  typeOff // .(*FuncType) underneath // Question: 这是什么
}

// interfaceType represents an interface type.
/**
 * interfaceType代表接口类型
 */
type interfaceType struct {
	rtype // 通用数据类型
	pkgPath name      // import path        // 导入路径
	methods []imethod // sorted by hash     // 接口方法，根据hash排序
}

// mapType represents a map type.
/**
 * mapType表示Map类型。
 */
type mapType struct {
	rtype // 通用数据类型
	key    *rtype // map key type                                   // map key的类型
	elem   *rtype // map element (value) type                       // map元素的类型
	bucket *rtype // internal bucket structure                      // hash桶结构
	// function for hashing keys (ptr to key, seed) -> hash         // hash函数
	hasher     func(unsafe.Pointer, uintptr) uintptr
	keysize    uint8  // size of key slot                           // key槽数
	valuesize  uint8  // size of value slot                         // value槽数
	bucketsize uint16 // size of bucket                             // 桶数
	flags      uint32                                               // Question: 用来做什么
}

// ptrType represents a pointer type.
/**
 * ptrType表示指针类型。
 */
type ptrType struct {
	rtype // 通用数据类型
	elem *rtype // pointer element (pointed at) type // 指针指向的元素类型
}

// sliceType represents a slice type.
/**
 * sliceType表示切片类型。
 */
type sliceType struct {
	rtype // 通用数据类型
	elem *rtype // slice element type // 切片元素类型
}

// Struct field
/**
 * 结构体字段类型
 */
type structField struct {
	name        name    // name is always non-empty                 // 名称始终为非空
	typ         *rtype  // type of field                            // 字段类型
	offsetEmbed uintptr // byte offset of field<<1 | isEmbedded     // 偏移量指针与嵌入类型共用字段
}
/**
 * 求偏移量
 * @return uintptr 偏移量指针
 * @date 2020-03-18 12:49:38
 **/
func (f *structField) offset() uintptr {
	return f.offsetEmbed >> 1
}

/**
 * 是否是嵌入类型
 * @return true: 是嵌入类型
 * @date 2020-03-18 12:50:47
 **/
func (f *structField) embedded() bool {
	return f.offsetEmbed&1 != 0
}

// structType represents a struct type.
/**
 * structType表示结构类型。
 **/
type structType struct {
	rtype // 通用数据类型
	pkgPath name // 包名
	fields  []structField // sorted by offset // 结构体字段，根据偏量排序
}

// name is an encoded type name with optional extra data.
//
// The first byte is a bit field containing:
//
//	1<<0 the name is exported
//	1<<1 tag data follows the name
//	1<<2 pkgPath nameOff follows the name and tag
//
// The next two bytes are the data length:
//
//	 l := uint16(data[1])<<8 | uint16(data[2])
//
// Bytes [3:3+l] are the string data.
//
// If tag data follows then bytes 3+l and 3+l+1 are the tag length,
// with the data following.
//
// If the import path follows, then 4 bytes at the end of
// the data form a nameOff. The import path is only set for concrete
// methods that are defined in a different package than their type.
//
// If a name starts with "*", then the exported bit represents
// whether the pointed to type is exported.
/**
 * name是带有可选附加数据的编码类型名称。
 *
 * 第一个字节是一个位字段，其中包含
 *  1 << 0 名称是可导出的
 *  1 << 1 标签数据跟随名称之后
 *  1 << 2 pkgPath nameOff跟随名称和标签数据之后
 *
 * 接下来的两个字节是数据长度：
 *  l := uint16(data[1])<<8 | uint16(data[2])
 * 字节[3:3+1]是字符串数据。
 * 数据结构示意思图，下划线表示一个位，+和|号表示分割符，...表示省略多个字节
 * +--------+--------+--------+--------...--------+--------+--------+--------...--------+--------+--------+--------+--------+
 * |     ???|    name len     |     name data     |     tag len     |      tag data     |          pkgPath nameOff          |
 * +--------+--------+--------+--------...--------+--------+--------+--------...--------+--------+--------+--------+--------+
 *
 * 如果名称后跟随标签数据，则字节3+1和3+1+1是标签长度，数据跟随在后面
 * 如果有导入路径，则数据末尾的4个字节形成nameOff
 * 仅为在与包类型不同的包中定义的具体方法设置导入路径。
 *
 * 如果名称以“*”开头，则导出的位表示是否导出了所指向的类型。
 */
type name struct {
	bytes *byte
}
/**
 * 添加字符串
 * @param 偏移量
 * @param 字符串，实际未使用到
 * @return 返回新的引用地址
 * @date 2020-03-19 08:28:49
 **/
func (n name) data(off int, whySafe string) *byte {
	return (*byte)(add(unsafe.Pointer(n.bytes), uintptr(off), whySafe))
}

/**
 * 是否是可导出类型
 * @return true: 是
 * @date 2020-03-19 08:31:13
 **/
func (n name) isExported() bool {
	return (*n.bytes)&(1<<0) != 0 // 最低位为0
}

/**
 * 名称长度
 * @return
 * @date 2020-03-19 08:33:18
 **/
func (n name) nameLen() int {
    // 0b表示前二进制前缀，a,b,c,d表示0或者1
    // bytes = [0baaaaaaaa, 0bbbbbbbbb, 0bcccccccc, 0bdddddddd]
    // A: uint16(*n.data(1, "name len field"))<<8 ==> 0bbbbbbbbb_?????????
    // B: uint16(*n.data(2, "name len field")) ==> 0b????????_cccccccc
    // A|B ==> 00bbbbbbbbb_cccccccc
	return int(uint16(*n.data(1, "name len field"))<<8 | uint16(*n.data(2, "name len field")))
}

/**
 * 获取标签的长度
 * @return
 * @date 2020-03-19 08:48:05
 **/
func (n name) tagLen() int {
    // 第一个字节的第二位是1说明有标签
	if *n.data(0, "name flag field")&(1<<1) == 0 {
		return 0
	}

	// 标签长度使用两个字节表示，第一个字节所在位置
	off := 3 + n.nameLen()
	return int(uint16(*n.data(off, "name taglen field"))<<8 | uint16(*n.data(off+1, "name taglen field")))
}

/**
 * 获取名称字符串
 * @return 名称字符串
 * @date 2020-03-19 09:01:31
 **/
func (n name) name() (s string) {
	if n.bytes == nil {
		return
	}
	b := (*[4]byte)(unsafe.Pointer(n.bytes))    // 创建一个地址

	hdr := (*stringHeader)(unsafe.Pointer(&s))  // 创建stringHeader对象
	hdr.Data = unsafe.Pointer(&b[3])            // 设置数据开始位置
	hdr.Len = int(b[1])<<8 | int(b[2])          // 设置数据长度
	return s
}

/**
 * 获取标签字符串
 * @return 标签字符串
 * @date 2020-03-19 09:39:21
 **/
func (n name) tag() (s string) {
	tl := n.tagLen() // 取标签长度
	if tl == 0 { // 说明没有签
		return ""
	}
	nl := n.nameLen() // 取名称长度
	hdr := (*stringHeader)(unsafe.Pointer(&s)) // 创建stringHeader对象
	hdr.Data = unsafe.Pointer(n.data(3+nl+2, "non-empty string")) // 标签字符串地址
	hdr.Len = tl // 设置长度
	return s
}

/**
 * 获取包路径信息
 * @return 包路径信息
 * @date 2020-03-19 09:41:30
 **/
func (n name) pkgPath() string {
    // 第一个字节第三个位是0，说明没有包路径
	if n.bytes == nil || *n.data(0, "name flag field")&(1<<2) == 0 {
		return ""
	}
	// 求包路径的偏移量
	off := 3 + n.nameLen()
	if tl := n.tagLen(); tl > 0 {
		off += 2 + tl
	}
	var nameOff int32 // 用于保存偏移量地址

	// 请注意，该字段在内存中可能未对齐，因此在此我们不能使用直接的int32分配。
	// Note that this field may not be aligned in memory,
	// so we cannot use a direct int32 assignment here.
	// 如果包路路径存在，则n.data最后四个字节表示包路径的偏移量地址，将最后四个字节取出，创建结构体，并且复制结构体
	// Question: 原理是什么？
	copy((*[4]byte)(unsafe.Pointer(&nameOff))[:], (*[4]byte)(unsafe.Pointer(n.data(off, "name offset field")))[:])
	pkgPathName := name{(*byte)(resolveTypeOff(unsafe.Pointer(n.bytes), nameOff))} // 创建name结构体
	return pkgPathName.name() // 最终获取包路径信息
}

/**
 * 创建name结构体，并未设置包路径偏移量
 * @param n 名称字
 * @param taq 标签信息
 * @param exported 是否可导出
 * @return name结构体
 * @date 2020-03-19 09:50:38
 **/
func newName(n, tag string, exported bool) name {
    // 长度不能大于65535
	if len(n) > 1<<16-1 {
		panic("reflect.nameFrom: name too long: " + n)
	}
    // 长度不能大于65535
	if len(tag) > 1<<16-1 {
		panic("reflect.nameFrom: tag too long: " + tag)
	}

	var bits byte // 标记字段
	l := 1 + 2 + len(n) // l用于记录数据长度
	if exported { // 标记是否可导出
		bits |= 1 << 0
	}
	if len(tag) > 0 { // 有标签数据
		l += 2 + len(tag)
		bits |= 1 << 1 // 标记有标签
	}

	b := make([]byte, l)
	b[0] = bits // 设置位标记
	b[1] = uint8(len(n) >> 8) // 设置名称长度
	b[2] = uint8(len(n))
	copy(b[3:], n) // 拷贝名称数据
	if len(tag) > 0 { // 有标签数据
		tb := b[3+len(n):]
		tb[0] = uint8(len(tag) >> 8) // 设置标签长度
		tb[1] = uint8(len(tag))
		copy(tb[2:], tag) // 拷贝标签数据
	}

	return name{bytes: &b[0]}
}

/**
 * The compiler knows the exact layout of all the data structures above.
 * The compiler does not know about the data structures and methods below.
 * 编译器知道上面所有数据结构的确切布局。
 * 编译器不了解以下数据结构和方法。
 */

// Method represents a single method.
/**
 * Method表示一个方法类型
 */
type Method struct {
	// Name is the method name.
	// PkgPath is the package path that qualifies a lower case (unexported)
	// method name. It is empty for upper case (exported) method names.
	// The combination of PkgPath and Name uniquely identifies a method
	// in a method set.
	// See https://golang.org/ref/spec#Uniqueness_of_identifiers
	/**
	 * Name是方法名称。
     * PkgPath是包含小写（未导出）方法名称的程序包路径。大写（导出）的方法名称PkgPath为空。
     * PkgPath和Name的组合唯一标识方法集中的方法。
     * 参见https://golang.org/ref/spec#Uniqueness_of_identifiers
	 */
	Name    string
	PkgPath string

	Type  Type  // method type // 方法类型
	Func  Value // func with receiver as first argument // 以接收者为第一个参数的func
	Index int   // index for Type.Method // Type.Method的索引
}

const (
	kindDirectIface = 1 << 5  // 0b00100000
	kindGCProg      = 1 << 6 // Type.gc points to GC program // Type.gc指向GC程序 // 0b01000000
	kindMask        = (1 << 5) - 1 // 0b00011111
)

// String returns the name of k.
/**
 * 字符串返回k的名称。
 @return string k的名称字符串
 */
func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "kind" + strconv.Itoa(int(k))
}

/**
 * 类型和名称映射
 */
var kindNames = []string{
	Invalid:       "invalid",
	Bool:          "bool",
	Int:           "int",
	Int8:          "int8",
	Int16:         "int16",
	Int32:         "int32",
	Int64:         "int64",
	Uint:          "uint",
	Uint8:         "uint8",
	Uint16:        "uint16",
	Uint32:        "uint32",
	Uint64:        "uint64",
	Uintptr:       "uintptr",
	Float32:       "float32",
	Float64:       "float64",
	Complex64:     "complex64",
	Complex128:    "complex128",
	Array:         "array",
	Chan:          "chan",
	Func:          "func",
	Interface:     "interface",
	Map:           "map",
	Ptr:           "ptr",
	Slice:         "slice",
	String:        "string",
	Struct:        "struct",
	UnsafePointer: "unsafe.Pointer",
}
/**
 * 获取类型上定义的所有方法，包括未导出的方法
 * @return 类型上定义的所有方法
 * @date 2020-03-20 08:37:57
 **/
func (t *uncommonType) methods() []method {
	if t.mcount == 0 {
		return nil
	}
	// 一个长度是65536个的数组，从中取指定位置的值
	// 切片从0到第t.mcount-1位，长度为t.mcount，最大扩充项cap设置为t.mcount
	return (*[1 << 16]method)(add(unsafe.Pointer(t), uintptr(t.moff), "t.mcount > 0"))[:t.mcount:t.mcount]
}

/**
 * 获取类型上定义的所有导出的方法
 * @return 类型上定义的所有导出的方法
 * @date 2020-03-20 08:37:57
 **/
func (t *uncommonType) exportedMethods() []method {
	if t.xcount == 0 {
		return nil
	}
	return (*[1 << 16]method)(add(unsafe.Pointer(t), uintptr(t.moff), "t.xcount > 0"))[:t.xcount:t.xcount]
}

// resolveNameOff resolves a name offset from a base pointer.
// The (*rtype).nameOff method is a convenience wrapper for this function.
// Implemented in the runtime package.
/**
 * resolveNameOff解析与名称基本指针的偏移量。 (*rtype).nameOff方法是此功能的便捷包装。在runtime包中实现。
 * @param ptrInModule 模块中的指针对象
 * @param off 偏移量
 * @return 计算后的打针对象
 * @date 2020-03-20 08:46:29
 **/
func resolveNameOff(ptrInModule unsafe.Pointer, off int32) unsafe.Pointer

// resolveTypeOff resolves an *rtype offset from a base type.
// The (*rtype).typeOff method is a convenience wrapper for this function.
// Implemented in the runtime package.
/**
 * resolveTypeOff解析*rtype与基本类型的偏移量。(*rtype).typeOff方法是此函数的便捷包装。在runtime包中实现。
 * @param 类型的指针
 * @param 偏移量
 * @return 计算后的类型指针
 * @date 2020-03-20 08:49:12
 **/
func resolveTypeOff(rtype unsafe.Pointer, off int32) unsafe.Pointer

// resolveTextOff resolves a function pointer offset from a base type.
// The (*rtype).textOff method is a convenience wrapper for this function.
// Implemented in the runtime package.
/**
 * resolveTextOff解析函数指针与基本类型的偏移量。(*rtype).textOff方法是此函数的便捷包装。在runtime包中实现。
 * @param 类型的指针
 * @param 偏移量
 * @return 计算后的类型指针
 * @date 2020-03-20 08:49:12
 **/
func resolveTextOff(rtype unsafe.Pointer, off int32) unsafe.Pointer

// addReflectOff adds a pointer to the reflection lookup map in the runtime.
// It returns a new ID that can be used as a typeOff or textOff, and will
// be resolved correctly. Implemented in the runtime package.
/**
 * addReflectOff在运行时中将一个指针添加到反射查找map。返回一个新的ID，该ID可以用作typeOff或textOff，并且可以正确解析。在运行时包中实现。
 * @param 指针
 * @param 偏移量
 * @return 新的ID
 * @date 2020-03-20 08:49:12
 **/
func addReflectOff(ptr unsafe.Pointer) int32

// resolveReflectType adds a name to the reflection lookup map in the runtime.
// It returns a new nameOff that can be used to refer to the pointer.
/**
 * resolveReflectType在运行时为反射查找map添加名称。它返回一个新的nameOff，可以用来引用该指针。
 * @param n 名称
 * @return 新的nameOff
 * @date 2020-03-20 08:54:31
 **/
func resolveReflectName(n name) nameOff {
	return nameOff(addReflectOff(unsafe.Pointer(n.bytes)))
}

// resolveReflectType adds a *rtype to the reflection lookup map in the runtime.
// It returns a new typeOff that can be used to refer to the pointer.
/**
 * resolveReflectType在运行时将*rtype添加到反射查找map中。它返回一个新的typeOff，可以用来引用该指针。
 * @param t 类型
 * @return 新的typeOff
 * @date 2020-03-20 08:55:32
 **/
func resolveReflectType(t *rtype) typeOff {
	return typeOff(addReflectOff(unsafe.Pointer(t)))
}

// resolveReflectText adds a function pointer to the reflection lookup map in
// the runtime. It returns a new textOff that can be used to refer to the
// pointer.
/**
 * resolveReflectText将函数指针添加到反射查找map中，运行时。它返回一个新的textOff，可以用来引用该指针。
 * @param ptr 指针
 * @return 新的textOff
 * @return
 * @date 2020-03-20 08:57:01
 **/
func resolveReflectText(ptr unsafe.Pointer) textOff {
	return textOff(addReflectOff(ptr))
}

type nameOff int32 // offset to a name      // 到一个name的偏移量
type typeOff int32 // offset to an *rtype   // 到*rtype的偏移量
type textOff int32 // offset from top of text section // 与文字部分顶部的偏移量

/**
 * 根据nameOff创建name实体
 * @param off 到一个name的偏移量
 * @return name对象
 * @date 2020-03-20 08:59:24
 **/
func (t *rtype) nameOff(off nameOff) name {
	return name{(*byte)(resolveNameOff(unsafe.Pointer(t), int32(off)))}
}
/**
 * 根据typeOff创建rtype结构体指针
 * @param 到*rtype的偏移量
 * @return rtype的指针
 * @date 2020-03-20 09:00:10
 **/
func (t *rtype) typeOff(off typeOff) *rtype {
	return (*rtype)(resolveTypeOff(unsafe.Pointer(t), int32(off)))
}

/**
 * 根据textOff指针结构体实例
 * @param 与文字部分顶部的偏移量
 * @return 指针结构体实例
 * @date 2020-03-20 09:02:56
 **/
func (t *rtype) textOff(off textOff) unsafe.Pointer {
	return resolveTextOff(unsafe.Pointer(t), int32(off))
}
/**
 * 求rtype的uncommonType，返回是uncommonType指针
 * @return uncommonType指针
 * @date 2020-03-20 09:33:28
 **/
func (t *rtype) uncommon() *uncommonType {
	if t.tflag&tflagUncommon == 0 { // uncommonType不存在
		return nil
	}

	// 根据不同的类型，创建对应的xxxTypeUncommon，返回其uncommonType类型指针
	switch t.Kind() {
	case Struct:
		return &(*structTypeUncommon)(unsafe.Pointer(t)).u
	case Ptr:
		type u struct {
			ptrType
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	case Func:
		type u struct {
			funcType
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	case Slice:
		type u struct {
			sliceType
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	case Array:
		type u struct {
			arrayType
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	case Chan:
		type u struct {
			chanType
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	case Map:
		type u struct {
			mapType
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	case Interface:
		type u struct {
			interfaceType
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	default:
		type u struct {
			rtype
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	}
}

/**
 * 获取rtype类型的字符表示
 * @return type类型的字符表示
 * @date 2020-03-20 09:39:55
 **/
func (t *rtype) String() string {
	s := t.nameOff(t.str).name()
	if t.tflag&tflagExtraStar != 0 { // 有额外的*号前缀
		return s[1:]
	}
	return s
}

/**
 * 获取rtype类型的size属性值
 * @return size属性值
 * @date 2020-03-20 09:41:46
 **/
func (t *rtype) Size() uintptr { return t.size }
/**
 * rtype所代表的数据值，在内存中表示需要的位数，只能求数值类型的，非数值类型会引起panic
 * @param
 * @param
 * @return
 * @return
 * @date 2020-03-20 09:43:37
 **/
func (t *rtype) Bits() int {
	if t == nil { // 类型不能为空
		panic("reflect: Bits of nil Type")
	}
	// 不能是非数值类型
	k := t.Kind()
	if k < Int || k > Complex128 {
		panic("reflect: Bits of non-arithmetic Type " + t.String())
	}

    // 求位数
	return int(t.size) * 8
}

func (t *rtype) Align() int { return int(t.align) }

func (t *rtype) FieldAlign() int { return int(t.fieldAlign) }

func (t *rtype) Kind() Kind { return Kind(t.kind & kindMask) }

func (t *rtype) pointers() bool { return t.ptrdata != 0 }

func (t *rtype) common() *rtype { return t }

/**
 * 求rtype的代表的值的导出方法，有uncommonType才可能有导出类型
 * @return 导出方法
 * @date 2020-03-20 09:50:20
 **/
func (t *rtype) exportedMethods() []method {
	ut := t.uncommon()
	if ut == nil {
		return nil
	}
	return ut.exportedMethods()
}

/**
 * 求rtype的代表的值的导出方法数
 * @return 导出方法数
 * @date 2020-03-20 09:58:24
 **/
func (t *rtype) NumMethod() int {
	if t.Kind() == Interface {
		tt := (*interfaceType)(unsafe.Pointer(t))
		return tt.NumMethod()
	}
	return len(t.exportedMethods())
}
/**
 * 求导出方法中的第i个方法
 * @param i 第i个方法，0开始计数
 * @return 方法结构体实例
 * @date 2020-03-20 10:14:22
 **/
func (t *rtype) Method(i int) (m Method) {
    // 接口类型
	if t.Kind() == Interface {
		tt := (*interfaceType)(unsafe.Pointer(t))
		return tt.Method(i)
	}

	// 非接口类型
	methods := t.exportedMethods()
	if i < 0 || i >= len(methods) {
		panic("reflect: Method index out of range")
	}
	p := methods[i]
	pname := t.nameOff(p.name) // 根据nameOff创建name实例
	m.Name = pname.name()

	fl := flag(Func)
	mtyp := t.typeOff(p.mtyp) // 根据typeOff创建rtype结构体，并反回指针
	ft := (*funcType)(unsafe.Pointer(mtyp)) // 获取方法类型

	in := make([]Type, 0, 1+len(ft.in()))
	in = append(in, t) // 方法的第一个入参表示方法的接收者
	for _, arg := range ft.in() { // 添加入参
		in = append(in, arg)
	}
	out := make([]Type, 0, len(ft.out()))
	for _, ret := range ft.out() { // 添加出参
		out = append(out, ret)
	}

	// FuncOf返回具有给定参数和结果类型的函数类型。
	mt := FuncOf(in, out, ft.IsVariadic())
	m.Type = mt // 设置方法类型
	tfn := t.textOff(p.tfn) // 求函数的textOff
	fn := unsafe.Pointer(&tfn)
	m.Func = Value{mt.(*rtype), fn, fl} // 设置方法值，可使用此值调用方法

	m.Index = i // 设置是第几个方法
	return m
}

/**
 * 根据名称查找导出的方法
 * @param name 方法名
 * @return m 找到的方法
 * @return ok true: 找到方法
 * @date 2020-03-21 11:21:50
 **/
func (t *rtype) MethodByName(name string) (m Method, ok bool) {
    // 接口类型
	if t.Kind() == Interface {
		tt := (*interfaceType)(unsafe.Pointer(t))
		return tt.MethodByName(name)
	}

	// 非接口类型
	ut := t.uncommon()
	if ut == nil {
		return Method{}, false
	}
	// TODO(mdempsky): Binary search.
	// 根据名称找到对应的方法索引，再根据索引找到对应的方法
	for i, p := range ut.exportedMethods() {
		if t.nameOff(p.name).name() == name {
			return t.Method(i), true
		}
	}
	return Method{}, false
}

/**
 * 获取包路径信息，没有返回""
 * @return 包路径信息，没有返回""
 * @date 2020-03-21 11:25:34
 **/
func (t *rtype) PkgPath() string {
	if t.tflag&tflagNamed == 0 {
		return ""
	}
	ut := t.uncommon()
	if ut == nil {
		return ""
	}
	return t.nameOff(ut.pkgPath).name()
}

/**
 * 类型是否有名称
 * @return 类型是否有名称。true: 是
 * @date 2020-03-21 11:26:48
 **/
func (t *rtype) hasName() bool {
	return t.tflag&tflagNamed != 0
}

/**
 * 获取类型的名称
 * @return 类型的名称
 * @date 2020-03-21 11:28:20 
 **/
func (t *rtype) Name() string {
	if !t.hasName() {
		return ""
	}
	s := t.String()
	i := len(s) - 1
	for i >= 0 && s[i] != '.' { // 找最后的一个.号的位置
		i--
	}
	return s[i+1:]
}

/**
 * 获取类型的通道类型，如果不是通道类型，引起panic
 * @return 通道类型
 * @date 2020-03-21 11:29:24 
 **/
func (t *rtype) ChanDir() ChanDir {
	if t.Kind() != Chan {
		panic("reflect: ChanDir of non-chan type " + t.String())
	}
	tt := (*chanType)(unsafe.Pointer(t))
	return ChanDir(tt.dir)
}

/**
 * 是否有可变参数，如要不是Func类型，引起panic
 * @return true: 有可变参数类型
 * @date 2020-03-21 11:31:13 
 **/
func (t *rtype) IsVariadic() bool {
	if t.Kind() != Func {
		panic("reflect: IsVariadic of non-func type " + t.String())
	}
	tt := (*funcType)(unsafe.Pointer(t))
	return tt.outCount&(1<<15) != 0 // outCount最高位为1说明有可变参数
}

/**
 * 获取元素类型，此方法只对Array, Chan, Map, Ptr, Slice类型有效，其他类型返回Panic
 * @return 元素类型
 * @date 2020-03-21 11:33:37 
 **/
func (t *rtype) Elem() Type {
	switch t.Kind() {
	case Array:
		tt := (*arrayType)(unsafe.Pointer(t))
		return toType(tt.elem)
	case Chan:
		tt := (*chanType)(unsafe.Pointer(t))
		return toType(tt.elem)
	case Map:
		tt := (*mapType)(unsafe.Pointer(t))
		return toType(tt.elem)
	case Ptr:
		tt := (*ptrType)(unsafe.Pointer(t))
		return toType(tt.elem)
	case Slice:
		tt := (*sliceType)(unsafe.Pointer(t))
		return toType(tt.elem)
	}
	panic("reflect: Elem of invalid type " + t.String())
}

/**
 * 获取第i个属性，非结构体类型会引起panic
 * @param 第i个属性
 * @return 结构体属性实例
 * @date 2020-03-21 11:35:14 
 **/
func (t *rtype) Field(i int) StructField {
	if t.Kind() != Struct {
		panic("reflect: Field of non-struct type " + t.String())
	}
	tt := (*structType)(unsafe.Pointer(t))
	return tt.Field(i)
}

/**
 * 获取属性，参数表示层次关系，非结构体类型会引起panic
 * @param 属性位置切片
 * @return 结构体属性实例
 * @date 2020-03-21 11:35:14 
 **/
func (t *rtype) FieldByIndex(index []int) StructField {
	if t.Kind() != Struct {
		panic("reflect: FieldByIndex of non-struct type " + t.String())
	}
	tt := (*structType)(unsafe.Pointer(t))
	return tt.FieldByIndex(index)
}
/**
 * 根据名称找到对应的属性
 * @param 属性名
 * @return 结构体属性实例
 * @return true: 找到
 * @date 2020-03-21 11:39:00 
 **/
func (t *rtype) FieldByName(name string) (StructField, bool) {
	if t.Kind() != Struct {
		panic("reflect: FieldByName of non-struct type " + t.String())
	}
	tt := (*structType)(unsafe.Pointer(t))
	return tt.FieldByName(name)
}

/**
 * 根据匹配函数找到对应的属性
 * @param 匹配函数
 * @return 结构体属性实例
 * @return true: 找到
 * @date 2020-03-21 11:39:00 
 **/
func (t *rtype) FieldByNameFunc(match func(string) bool) (StructField, bool) {
	if t.Kind() != Struct {
		panic("reflect: FieldByNameFunc of non-struct type " + t.String())
	}
	tt := (*structType)(unsafe.Pointer(t))
	return tt.FieldByNameFunc(match)
}
/**
 * 方法的第i个入参的类型
 * @param 第i个入参
 * @return 第i个入参的类型
 * @date 2020-03-21 11:43:56 
 **/
func (t *rtype) In(i int) Type {
	if t.Kind() != Func {
		panic("reflect: In of non-func type " + t.String())
	}
	tt := (*funcType)(unsafe.Pointer(t))
	return toType(tt.in()[i])
}

/**
 * Map的key类型
 * @return Map的key类型
 * @date 2020-03-21 11:44:36 
 **/
func (t *rtype) Key() Type {
	if t.Kind() != Map {
		panic("reflect: Key of non-map type " + t.String())
	}
	tt := (*mapType)(unsafe.Pointer(t))
	return toType(tt.key)
}

/**
 * 获取数组长度
 * @return 数组长度
 * @date 2020-03-21 11:46:22
 **/
func (t *rtype) Len() int {
	if t.Kind() != Array {
		panic("reflect: Len of non-array type " + t.String())
	}
	tt := (*arrayType)(unsafe.Pointer(t))
	return int(tt.len)
}

/**
 * 结构的字段数目
 * @return 结构的字段数目
 * @date 2020-03-21 11:47:34
 **/
func (t *rtype) NumField() int {
	if t.Kind() != Struct {
		panic("reflect: NumField of non-struct type " + t.String())
	}
	tt := (*structType)(unsafe.Pointer(t))
	return len(tt.fields)
}

/**
 * 方法的入参个数
 * @return
 * @date 2020-03-21 11:48:04
 **/
func (t *rtype) NumIn() int {
	if t.Kind() != Func {
		panic("reflect: NumIn of non-func type " + t.String())
	}
	tt := (*funcType)(unsafe.Pointer(t))
	return int(tt.inCount)
}

/**
 * 方法的出参个数
 * @return 方法的出参个数
 * @date 2020-03-21 11:48:21
 **/
func (t *rtype) NumOut() int {
	if t.Kind() != Func {
		panic("reflect: NumOut of non-func type " + t.String())
	}
	tt := (*funcType)(unsafe.Pointer(t))
	return len(tt.out())
}

/**
 * 方法的第i个出参
 * @param 第i个出参
 * @return 第i个出参类型
 * @date 2020-03-21 11:48:43
 **/
func (t *rtype) Out(i int) Type {
	if t.Kind() != Func {
		panic("reflect: Out of non-func type " + t.String())
	}
	tt := (*funcType)(unsafe.Pointer(t))
	return toType(tt.out()[i])
}
/**
 * 所有的入参
 * @return 所有的入参
 * @date 2020-03-21 11:49:56
 **/
func (t *funcType) in() []*rtype {
	uadd := unsafe.Sizeof(*t)
	if t.tflag&tflagUncommon != 0 {
		uadd += unsafe.Sizeof(uncommonType{})
	}
	// 这里可以提到前面
	if t.inCount == 0 {
		return nil
	}
	return (*[1 << 20]*rtype)(add(unsafe.Pointer(t), uadd, "t.inCount > 0"))[:t.inCount:t.inCount]
}

/**
 * 所有的出参
 * @return
 * @date 2020-03-21 11:50:19
 **/
func (t *funcType) out() []*rtype {
	uadd := unsafe.Sizeof(*t)
	if t.tflag&tflagUncommon != 0 {
		uadd += unsafe.Sizeof(uncommonType{})
	}
	// 判断可以提前
	outCount := t.outCount & (1<<15 - 1)
	if outCount == 0 {
		return nil
	}
	return (*[1 << 20]*rtype)(add(unsafe.Pointer(t), uadd, "outCount > 0"))[t.inCount : t.inCount+outCount : t.inCount+outCount]
}

// add returns p+x.
//
// The whySafe string is ignored, so that the function still inlines
// as efficiently as p+x, but all call sites should use the string to
// record why the addition is safe, which is to say why the addition
// does not cause x to advance to the very end of p's allocation
// and therefore point incorrectly at the next block in memory.
/**
 * add返回p + x。
 *
 * whySafe字符串将被忽略，因此该函数仍然可以像p+x一样有效地内联，
 * 但是所有调用站点都应使用该字符串来记录为什么加法是安全的，
 * 也就是说加法为何不会导致x提前到p分配的末尾，因此而错误地指向了内存中的下一个块。
 * @param p 指针类型
 * @param x 偏移量
 * @param whySafe 提示字符串
 * @return 新的指针类型
 * @return
 * @date 2020-03-21 14:40:58
 **/
func add(p unsafe.Pointer, x uintptr, whySafe string) unsafe.Pointer {
	return unsafe.Pointer(uintptr(p) + x)
}

/**
 * 通道方向字符串描述
 * @return 通道方向字符串描述
 * @date 2020-03-21 14:42:43
 **/
func (d ChanDir) String() string {
	switch d {
	case SendDir:
		return "chan<-"
	case RecvDir:
		return "<-chan"
	case BothDir:
		return "chan"
	}
	return "ChanDir" + strconv.Itoa(int(d))
}

// Method returns the i'th method in the type's method set.
/**
 * 方法返回类型的方法集中的第i个方法。
 * @return 方法集中的第i个方法
 * @date 2020-03-21 14:43:31
 **/
func (t *interfaceType) Method(i int) (m Method) {
	if i < 0 || i >= len(t.methods) { //
		return
	}
	p := &t.methods[i]
	pname := t.nameOff(p.name)
	m.Name = pname.name() // 设置方法名
	if !pname.isExported() { // 如果方法是可导出的，则设置方法名
		m.PkgPath = pname.pkgPath()
		if m.PkgPath == "" { // pname上的名称可能为空，则取接口类型上的包名
			m.PkgPath = t.pkgPath.name()
		}
	}
	m.Type = toType(t.typeOff(p.typ)) // 设置方法的type类类型
	m.Index = i // 设置是接口的第i个方法
	return
}

// NumMethod returns the number of interface methods in the type's method set.
/**
 * NumMethod返回类型的方法集中的接口方法的数量。
 * @return 方法集中的接口方法的数量
 * @date 2020-03-21 14:52:34
 **/
func (t *interfaceType) NumMethod() int { return len(t.methods) }

// MethodByName method with the given name in the type's method set.
func (t *interfaceType) MethodByName(name string) (m Method, ok bool) {
	if t == nil {
		return
	}
	var p *imethod
	for i := range t.methods {
		p = &t.methods[i]
		if t.nameOff(p.name).name() == name {
			return t.Method(i), true
		}
	}
	return
}

// A StructField describes a single field in a struct.
/**
 * StructField描述结构中的单个字段。
 */
type StructField struct {
	// Name is the field name.
	// 字段名称
	Name string
	// PkgPath is the package path that qualifies a lower case (unexported)
	// field name. It is empty for upper case (exported) field names.
	// See https://golang.org/ref/spec#Uniqueness_of_identifiers
	/**
	 * PkgPath是限定小写（未导出）字段名称的程序包路径。大写（导出）字段名称为空。
     * 参见https://golang.org/ref/spec#Uniqueness_of_identifiers
	 */
	PkgPath string

    /**
     * 字段类型
     */
	Type      Type      // field type
	/**
	 * 标签信息
	 */
	Tag       StructTag // field tag string
	/**
	 * 结构体中的偏移量（以字节为单位）
	 */
	Offset    uintptr   // offset within struct, in bytes
	/**
	 * Type.FieldByIndex的索引序列
	 */
	Index     []int     // index sequence for Type.FieldByIndex
	/**
	 * 是否是内嵌字段
	 */
	Anonymous bool      // is an embedded field
}

// A StructTag is the tag string in a struct field.
//
// By convention, tag strings are a concatenation of
// optionally space-separated key:"value" pairs.
// Each key is a non-empty string consisting of non-control
// characters other than space (U+0020 ' '), quote (U+0022 '"'),
// and colon (U+003A ':').  Each value is quoted using U+0022 '"'
// characters and Go string literal syntax.
/**
 * StructTag是struct字段中的标签字符串。
 *
 * 按照惯例，标签字符串是由空格分隔的key:"value"对组成的串联。
 * 每个key都是非空字符串，由非控制字符组成，除了空格（U+0020 ' '），引号（U+0022 '"'）
 * 和冒号（U+003A ':'）。每个值使用U+0022 '"字符和Go字符串文字语法引用。
 */
type StructTag string

// Get returns the value associated with key in the tag string.
// If there is no such key in the tag, Get returns the empty string.
// If the tag does not have the conventional format, the value
// returned by Get is unspecified. To determine whether a tag is
// explicitly set to the empty string, use Lookup.
/**
 * Get返回与标签字符串中的key关联的值。
 * 如果标签中没有这样的键，则Get返回空字符串。
 * 如果标记不具有常规格式，则未指定Get返回的值。
 * 若要确定是否将标记明确设置为空字符串，请使用Lookup。
 */
func (tag StructTag) Get(key string) string {
	v, _ := tag.Lookup(key)
	return v
}

// Lookup returns the value associated with key in the tag string.
// If the key is present in the tag the value (which may be empty)
// is returned. Otherwise the returned value will be the empty string.
// The ok return value reports whether the value was explicitly set in
// the tag string. If the tag does not have the conventional format,
// the value returned by Lookup is unspecified.
/**
 * 标签格式 ： `a:"b",c:"d",e:"f"`
 * 查找返回与标签字符串中的键关联的值。
 * 如果关键字存在于标签中，则返回值（可能为空）。否则，返回值将为空字符串。
 * ok返回值报告该值是否在标记字符串中显式设置。如果标记不具有常规格式，则未指定Lookup返回的值。
 */
func (tag StructTag) Lookup(key string) (value string, ok bool) {
	// When modifying this code, also update the validateStructTag code
	// in cmd/vet/structtag.go.
	/**
	 * 修改此代码时，还请更新cmd/vet/structtag.go中的validateStructTag代码。
	 */

	for tag != "" {
		// Skip leading space.
		// 忽略前导空格
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}

		// Scan to colon. A space, a quote or a control character is a syntax error.
		// Strictly speaking, control chars include the range [0x7f, 0x9f], not just
		// [0x00, 0x1f], but in practice, we ignore the multi-byte control characters
		// as it is simpler to inspect the tag's bytes than the tag's runes.
		// 扫描到冒号。空格，引号或控制字符是语法错误。
        // 严格来说，控制字符包括范围[0x7f，0x9f]，而不仅仅是
        // [0x00，0x1f]，但实际上，我们忽略了多字节控制字符
        // 因为检查标记的字节比标记的符文更简单。
		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
			i++
		}

		// 没找到或者格式不合法的情况
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			break
		}
		name := string(tag[:i]) // 标签名
		tag = tag[i+1:] // 标签值开始的切片

		// Scan quoted string to find value.
		// Scan quoted string to find value.
		i = 1
		for i < len(tag) && tag[i] != '"' { // 在不越界的情况下，直到第一个非转义"为止
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			break
		}
		qvalue := string(tag[:i+1]) // 双引号引起的字符串
		tag = tag[i+1:]

		if key == name { // 名称相等的情况下
			value, err := strconv.Unquote(qvalue) // 去掉引号后的字符串
			if err != nil {
				break
			}
			return value, true
		}
	}
	return "", false
}

// Field returns the i'th struct field.
/**
 * Field返回第i个struct字段。
 */
func (t *structType) Field(i int) (f StructField) {
	if i < 0 || i >= len(t.fields) { // 不存在第i个字段
		panic("reflect: Field index out of bounds")
	}

	p := &t.fields[i]
	f.Type = toType(p.typ) // 设置字段类型
	f.Name = p.name.name() // 设置字段名
	f.Anonymous = p.embedded() // 设置是否内嵌字段
	if !p.name.isExported() { // 非导出字段要设置包名
		f.PkgPath = t.pkgPath.name()
	}
	if tag := p.name.tag(); tag != "" { // 存在标签就设置标签
		f.Tag = StructTag(tag)
	}
	f.Offset = p.offset() // 设置偏移量

	// NOTE(rsc): This is the only allocation in the interface
	// presented by a reflect.Type. It would be nice to avoid,
	// at least in the common cases, but we need to make sure
	// that misbehaving clients of reflect cannot affect other
	// uses of reflect. One possibility is CL 5371098, but we
	// postponed that ugliness until there is a demonstrated
	// need for the performance. This is issue 2320.
	// NOTE（rsc）：这是reflect.Type在接口中提供的唯一分配。
	// 至少在通常情况下，最好避免这样做，但是我们需要确保行为不当的反射
	// 客户不会影响反射的其他用途。 一种可能是CL 5371098，但我们推迟了
	// 这种丑陋，直到表现出对性能的需求为止。 issue 2320。
	f.Index = []int{i} // 设置索引
	return
}

// TODO(gri): Should there be an error/bool indicator if the index
//            is wrong for FieldByIndex?
// TODO(gri): 如果FieldByIndex的索引错误，是否应该有一个错误/布尔指示器？

// FieldByIndex returns the nested field corresponding to index.
/**
 * FieldByIndex返回与索引对应的嵌套字段。
 * @param index 索引数组
 * @return 与索引对应的嵌套字段
 */
func (t *structType) FieldByIndex(index []int) (f StructField) {
	f.Type = toType(&t.rtype)
	for i, x := range index { // 遍历索引
		if i > 0 {
			ft := f.Type
			if ft.Kind() == Ptr && ft.Elem().Kind() == Struct { // 结构体指针类型特殊处理
				ft = ft.Elem()
			}
			f.Type = ft
		}
		f = f.Type.Field(x)
	}
	return
}

// A fieldScan represents an item on the fieldByNameFunc scan work list.
/**
 * fieldScan代表fieldByNameFunc扫描工作列表上的条目。
 */
type fieldScan struct {
	typ   *structType // 结构体类型
	index []int // 索引数组
}

// FieldByNameFunc returns the struct field with a name that satisfies the
// match function and a boolean to indicate if the field was found.
/**
 * FieldByNameFunc返回具有满足match函数名称的struct字段和一个布尔值，以指示是否找到了该字段。
 * @param match 匹配函数
 * @return result 结构体字段
 * @return 是否找到，true: 是
 * @date 2020-03-21 16:22:31 
 **/
func (t *structType) FieldByNameFunc(match func(string) bool) (result StructField, ok bool) {
	// This uses the same condition that the Go language does: there must be a unique instance
	// of the match at a given depth level. If there are multiple instances of a match at the
	// same depth, they annihilate each other and inhibit any possible match at a lower level.
	// The algorithm is breadth first search, one depth level at a time.
    /**
     * 这使用了与Go语言相同的条件：在给定的深度级别，必须有唯一的匹配实例。
     * 如果在同一深度有多个匹配的实例，它们将相消灭制并在较低级别禁止任何可能的匹配。
     * 该算法是广度优先搜索，一次一个深度级别。
     */

	// The current and next slices are work queues:
	// current lists the fields to visit on this depth level,
	// and next lists the fields on the next lower level.
	/**
	 * 当前和下一个切片是工作队列：
     * 当前列出了在此深度级别上要访问的字段，下一个列出了下一个较低级别的字段。
	 */
	current := []fieldScan{}
	next := []fieldScan{{typ: t}}

	// nextCount records the number of times an embedded type has been
	// encountered and considered for queueing in the 'next' slice.
	// We only queue the first one, but we increment the count on each.
	// If a struct type T can be reached more than once at a given depth level,
	// then it annihilates itself and need not be considered at all when we
	// process that next depth level.
	/**
	 * nextCount记录遇到嵌入式类型并考虑在“下一个”切片中进行排队的次数。
	 * 我们只将第一个入队列，但是我们增加每个的计数。 如果在给定的深度级别可以多次到达结构类型T，
	 * 那么它将消灭自身，并且在我们处理下一个深度级别时完全不需要考虑。
	 */
	var nextCount map[*structType]int

	// visited records the structs that have been considered already.
	// Embedded pointer fields can create cycles in the graph of
	// reachable embedded types; visited avoids following those cycles.
	// It also avoids duplicated effort: if we didn't find the field in an
	// embedded type T at level 2, we won't find it in one at level 4 either.
	/**
	 * 访问记录了已经考虑过的结构。
     * 嵌入式指针字段可以在可到达的嵌入式类型图中创建循环； 来访者避免陷入这些循环。
     * 这也避免了重复的工作：如果我们在第2级没有找到嵌入类型T中的字段，
     * 那么在第4级也不会在一个类型中找到该字段。
	 */
	visited := map[*structType]bool{}

	for len(next) > 0 {
		current, next = next, current[:0] // current变成现在要处理的层，next再次记录下一层
		count := nextCount
		nextCount = nil

		// Process all the fields at this depth, now listed in 'current'.
		// The loop queues embedded fields found in 'next', for processing during the next
		// iteration. The multiplicity of the 'current' field counts is recorded
		// in 'count'; the multiplicity of the 'next' field counts is recorded in 'nextCount'.
        /**
		* 处理此深度的所有字段，现在列在“current”中。
        * 循环将'next'中找到的嵌入字段入队，以便在下一次迭代中进行处理。
        * “current”字段计数的多重性记录在“count”中； “next”字段计数的多重性记录在“nextCount”中。
		*/
		for _, scan := range current {
			t := scan.typ
			if visited[t] {
				// We've looked through this type before, at a higher level.
				// That higher level would shadow the lower level we're now at,
				// so this one can't be useful to us. Ignore it.
                /**
                 * 之前，我们已经在更高层次上处理了这种类型。
                 * 较高的级别将掩盖我们现在所处的较低级别，因此这一级别对我们没有用。忽略它。
                 */
				continue
			}
			visited[t] = true
			for i := range t.fields {
				f := &t.fields[i]
				// Find name and (for embedded field) type for field f.
				// 查找字段f的名称和（对于嵌入式字段）类型。
				fname := f.name.name()
				var ntyp *rtype
				if f.embedded() {
					// Embedded field of type T or *T.
					// 类型T或* T的嵌入式字段。
					ntyp = f.typ
					if ntyp.Kind() == Ptr {
						ntyp = ntyp.Elem().common()
					}
				}

				// Does it match?
				// 是否匹配
				if match(fname) {
					// Potential match
					// 潜在匹配
					if count[t] > 1 || ok {
						// Name appeared multiple times at this level: annihilate.
						// 在此级别上，名字多次出现：歼灭。
						return StructField{}, false
					}
					result = t.Field(i) // 取属性，并且设置索引列表
					result.Index = nil
					result.Index = append(result.Index, scan.index...)
					result.Index = append(result.Index, i)
					ok = true
					continue
				}

				// Queue embedded struct fields for processing with next level,
				// but only if we haven't seen a match yet at this level and only
				// if the embedded types haven't already been queued.
				/**
				 * 将嵌入的struct字段入队列，以进行下一级别的处理，但前提是我们尚未在此级别看到匹配项，
				 * 并且仅当尚未对嵌入的类型进行入队列时。
				 */
				if ok || ntyp == nil || ntyp.Kind() != Struct {
					continue
				}
				styp := (*structType)(unsafe.Pointer(ntyp))
				if nextCount[styp] > 0 {
				    // 已经现匹配过两次了，那就不需要再处理了
					nextCount[styp] = 2 // exact multiple doesn't matter
					continue
				}

				if nextCount == nil {
					nextCount = map[*structType]int{}
				}

				nextCount[styp] = 1 // 本层已经见到过，下一层也要标记起来
				if count[t] > 1 { // 本层见过多次，下层也要标记见过多次
					nextCount[styp] = 2 // exact multiple doesn't matter  // 2就表示多次
				}
				var index []int
				index = append(index, scan.index...)
				index = append(index, i)
				next = append(next, fieldScan{styp, index})
			}
		}
		if ok {
			break
		}
	}
	return
}

// FieldByName returns the struct field with the given name
// and a boolean to indicate if the field was found.
/**
 * FieldByName返回具有给定名称的struct字段和一个布尔值，以指示是否找到了该字段。
 */
func (t *structType) FieldByName(name string) (f StructField, present bool) {
	// Quick check for top-level name, or struct without embedded fields.
	// 快速检查顶级名称或没有嵌入字段的结构。
	hasEmbeds := false
	if name != "" {
		for i := range t.fields {
			tf := &t.fields[i]
			if tf.name.name() == name {
				return t.Field(i), true
			}
			if tf.embedded() {
				hasEmbeds = true
			}
		}
	}
	if !hasEmbeds {
		return
	}

	// 有嵌入字段在这里进行处理
	return t.FieldByNameFunc(func(s string) bool { return s == name })
}

// TypeOf returns the reflection Type that represents the dynamic type of i.
// If i is a nil interface value, TypeOf returns nil.
/**
 * TypeOf返回表示i的动态类型的反射类型。如果i是nil接口值，则TypeOf返回nil。
 **/
func TypeOf(i interface{}) Type {
	eface := *(*emptyInterface)(unsafe.Pointer(&i))
	return toType(eface.typ)
}

// ptrMap is the cache for PtrTo.
/**
 * ptrMap是PtrTo的缓存。
 */
var ptrMap sync.Map // map[*rtype]*ptrType

// PtrTo returns the pointer type with element t.
// For example, if t represents type Foo, PtrTo(t) represents *Foo.
/**
 * PtrTo返回带有元素t的指针类型。例如，如果t表示类型Foo，则PtrTo(t)表示* Foo。
 */
func PtrTo(t Type) Type {
	return t.(*rtype).ptrTo()
}

/**
 * ptrTo返回带有元素t的指针类型。
 */
func (t *rtype) ptrTo() *rtype {
    // 有ptrToThis，直接使用
	if t.ptrToThis != 0 {
		return t.typeOff(t.ptrToThis)
	}

	// Check the cache.
	// 从缓存中取
	if pi, ok := ptrMap.Load(t); ok {
		return &pi.(*ptrType).rtype
	}

	// Look in known types.
	// 查找已知类型。
	s := "*" + t.String()
	// typesByString返回typelinks()的子片段，其元素具有给定的字符串表示形式。
	for _, tt := range typesByString(s) {
		p := (*ptrType)(unsafe.Pointer(tt))
		if p.elem != t {
			continue
		}
		pi, _ := ptrMap.LoadOrStore(t, p)
		return &pi.(*ptrType).rtype
	}

	// Create a new ptrType starting with the description
	// of an *unsafe.Pointer.
	// 从*unsafe.Pointer的描述开始创建一个新的ptrType。
	var iptr interface{} = (*unsafe.Pointer)(nil)
	prototype := *(**ptrType)(unsafe.Pointer(&iptr))
	pp := *prototype

	pp.str = resolveReflectName(newName(s, "", false))
	pp.ptrToThis = 0

	// For the type structures linked into the binary, the
	// compiler provides a good hash of the string.
	// Create a good hash for the new string by using
	// the FNV-1 hash's mixing function to combine the
	// old hash and the new "*".
	// 对于链接到二进制文件中的类型结构，编译器提供了字符串的良好哈希。
    // 通过使用FNV-1哈希的混合函数为旧字符串和新的“ *”组合，为新字符串创建良好的哈希。
	pp.hash = fnv1(t.hash, '*')

	pp.elem = t

	pi, _ := ptrMap.LoadOrStore(t, &pp)
	return &pi.(*ptrType).rtype
}

// fnv1 incorporates the list of bytes into the hash x using the FNV-1 hash function.
/**
 * fnv1使用FNV-1哈希函数将字节列表合并到哈希x中。
 */
func fnv1(x uint32, list ...byte) uint32 {
	for _, b := range list {
		x = x*16777619 ^ uint32(b)
	}
	return x
}

/**
 * 当前类型是否实现接口u
 */
func (t *rtype) Implements(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.Implements")
	}
	if u.Kind() != Interface {
		panic("reflect: non-interface type passed to Type.Implements")
	}
	return implements(u.(*rtype), t)
}

/**
 * 当前类型是否可赋值给类型u
 */
func (t *rtype) AssignableTo(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.AssignableTo")
	}
	uu := u.(*rtype)
	return directlyAssignable(uu, t) || implements(uu, t)
}

/**
 * 当前类型是否可转换成类型u
 */
func (t *rtype) ConvertibleTo(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.ConvertibleTo")
	}
	uu := u.(*rtype)
	return convertOp(uu, t) != nil
}

/**
 * 当前类型是否可比较
 */
func (t *rtype) Comparable() bool {
	return t.equal != nil
}

// implements reports whether the type V implements the interface type T.
/**
 * 实现报告类型V是否实现接口类型T。
 */
func implements(T, V *rtype) bool {
	if T.Kind() != Interface {
		return false
	}
	t := (*interfaceType)(unsafe.Pointer(T))
	if len(t.methods) == 0 {
		return true
	}

	// The same algorithm applies in both cases, but the
	// method tables for an interface type and a concrete type
	// are different, so the code is duplicated.
	// In both cases the algorithm is a linear scan over the two
	// lists - T's methods and V's methods - simultaneously.
	// Since method tables are stored in a unique sorted order
	// (alphabetical, with no duplicate method names), the scan
	// through V's methods must hit a match for each of T's
	// methods along the way, or else V does not implement T.
	// This lets us run the scan in overall linear time instead of
	// the quadratic time  a naive search would require.
	// See also ../runtime/iface.go.
	// 两种情况下都应用相同的算法，但是接口类型和具体类型的方法表不同，因此代码重复。
	// 在这两种情况下，算法都是同时对两个列表（T方法和V方法）进行线性扫描。
    // 由于方法表是以唯一的排序顺序存储的（字母顺序，没有重复的方法名称），
    // 因此对V方法的扫描必须沿途对每个T方法进行匹配，否则V不会实现T。
    // 这样，我们就可以在整个线性时间内（而不是天真的搜索所需的二次时间）运行扫描。
    // 另请参阅../runtime/iface.go。
	if V.Kind() == Interface { // V是接口类型
		v := (*interfaceType)(unsafe.Pointer(V))
		i := 0
		for j := 0; j < len(v.methods); j++ {
			tm := &t.methods[i]
			tmName := t.nameOff(tm.name)
			vm := &v.methods[j]
			vmName := V.nameOff(vm.name)
			// 名称必须相同，typeOff是同样的
			if vmName.name() == tmName.name() && V.typeOff(vm.typ) == t.typeOff(tm.typ) {
				if !tmName.isExported() { // 非导出类型还要比较包名
					tmPkgPath := tmName.pkgPath()
					if tmPkgPath == "" {
						tmPkgPath = t.pkgPath.name()
					}
					vmPkgPath := vmName.pkgPath()
					if vmPkgPath == "" {
						vmPkgPath = v.pkgPath.name()
					}
					if tmPkgPath != vmPkgPath {
						continue
					}
				}
				if i++; i >= len(t.methods) {
				    // 已经到了末尾
					return true
				}
			}
		}
		return false
	}

    // V是非接口类型
	v := V.uncommon()
	if v == nil {
		return false
	}
	i := 0
	vmethods := v.methods()
	for j := 0; j < int(v.mcount); j++ {
		tm := &t.methods[i]
		tmName := t.nameOff(tm.name)
		vm := vmethods[j]
		vmName := V.nameOff(vm.name)
		if vmName.name() == tmName.name() && V.typeOff(vm.mtyp) == t.typeOff(tm.typ) {
			if !tmName.isExported() {
				tmPkgPath := tmName.pkgPath()
				if tmPkgPath == "" {
					tmPkgPath = t.pkgPath.name()
				}
				vmPkgPath := vmName.pkgPath()
				if vmPkgPath == "" {
					vmPkgPath = V.nameOff(v.pkgPath).name()
				}
				if tmPkgPath != vmPkgPath {
					continue
				}
			}
			if i++; i >= len(t.methods) {
				return true
			}
		}
	}
	return false
}

// specialChannelAssignability reports whether a value x of channel type V
// can be directly assigned (using memmove) to another channel type T.
// https://golang.org/doc/go_spec.html#Assignability
// T and V must be both of Chan kind.
/**
 * specialChannelAssignability报告是否可以将通道类型V的值x直接（使用内存移动）分配给另一个通道类型T。
 * https://golang.org/doc/go_spec.html#Assignability
 * T和V必须都是Chan类型。
 */
func specialChannelAssignability(T, V *rtype) bool {
	// Special case:
	// x is a bidirectional channel value, T is a channel type,
	// x's type V and T have identical element types,
	// and at least one of V or T is not a defined type.
	// 特殊情况：
    // x是双向通道值，T是通道类型，
    // x的类型V和T具有相同的元素类型，
    // 并且V或T中的至少一个不是定义的类型。
	return V.ChanDir() == BothDir && (T.Name() == "" || V.Name() == "") && haveIdenticalType(T.Elem(), V.Elem(), true)
}

// directlyAssignable reports whether a value x of type V can be directly
// assigned (using memmove) to a value of type T.
// https://golang.org/doc/go_spec.html#Assignability
// Ignoring the interface rules (implemented elsewhere)
// and the ideal constant rules (no ideal constants at run time).
// directAssignable报告是否可以将V类型的值x直接（使用内存移动）分配给T类型的值。
// https://golang.org/doc/go_spec.html#Assignability
// 忽略接口规则（在其他地方实现）和理想常量规则（运行时没有理想常量）。
func directlyAssignable(T, V *rtype) bool {
	// x's type V is identical to T?
	// x的V型与T相同
	if T == V {
		return true
	}

	// Otherwise at least one of T and V must not be defined
	// and they must have the same kind.
	// 都有名称或者类型不同
	if T.hasName() && V.hasName() || T.Kind() != V.Kind() {
		return false
	}

    // 通道类型，并且通道可以赋值
	if T.Kind() == Chan && specialChannelAssignability(T, V) {
		return true
	}

	// x's type T and V must have identical underlying types.
	// x的类型T和V必须具有相同的底层类型。
	return haveIdenticalUnderlyingType(T, V, true)
}

/**
 * 判断是否类型一致
 */
func haveIdenticalType(T, V Type, cmpTags bool) bool {
	if cmpTags {
		return T == V
	}

    // 名称不同或者类型不同
	if T.Name() != V.Name() || T.Kind() != V.Kind() {
		return false
	}

    // 判断是否有相同的底层类型
	return haveIdenticalUnderlyingType(T.common(), V.common(), false)
}

/**
 * 判断是否有相同的底层类型
 */
func haveIdenticalUnderlyingType(T, V *rtype, cmpTags bool) bool {
	if T == V {
		return true
	}

	kind := T.Kind()
	if kind != V.Kind() {
		return false
	}

	// Non-composite types of equal kind have same underlying type
	// (the predefined instance of the type).
	// 相同种类的非复合类型具有相同的基础类型（该类型的预定义实例）。
	if Bool <= kind && kind <= Complex128 || kind == String || kind == UnsafePointer {
		return true
	}

	// Composite types.
	// 组合类型
	switch kind {
	case Array: // 对于数组类型，长度要一致，元素类型要相同
		return T.Len() == V.Len() && haveIdenticalType(T.Elem(), V.Elem(), cmpTags)

	case Chan: // 通道类型一致，通道元素一致
		return V.ChanDir() == T.ChanDir() && haveIdenticalType(T.Elem(), V.Elem(), cmpTags)

	case Func: // 函数类型，出参入参要一致，并且对应参数类型要相同
		t := (*funcType)(unsafe.Pointer(T))
		v := (*funcType)(unsafe.Pointer(V))
		if t.outCount != v.outCount || t.inCount != v.inCount {
			return false
		}
		for i := 0; i < t.NumIn(); i++ {
			if !haveIdenticalType(t.In(i), v.In(i), cmpTags) {
				return false
			}
		}
		for i := 0; i < t.NumOut(); i++ {
			if !haveIdenticalType(t.Out(i), v.Out(i), cmpTags) {
				return false
			}
		}
		return true

	case Interface: // 接口类型当没有方法时，才类型一致
		t := (*interfaceType)(unsafe.Pointer(T))
		v := (*interfaceType)(unsafe.Pointer(V))
		if len(t.methods) == 0 && len(v.methods) == 0 {
			return true
		}
		// Might have the same methods but still
		// need a run time conversion.
		// 可能具有相同的方法，但仍需要运行时转换。
		return false

	case Map: // 映射类型key和value类型都要致
		return haveIdenticalType(T.Key(), V.Key(), cmpTags) && haveIdenticalType(T.Elem(), V.Elem(), cmpTags)

	case Ptr, Slice: // 切片和指针类型，对应的元素类型必须一致
		return haveIdenticalType(T.Elem(), V.Elem(), cmpTags)

	case Struct: // 结构体类型，字段数，对应字段名称，类型，标签（如果需要），嵌入类型都要一致
		t := (*structType)(unsafe.Pointer(T))
		v := (*structType)(unsafe.Pointer(V))
		if len(t.fields) != len(v.fields) {
			return false
		}
		if t.pkgPath.name() != v.pkgPath.name() {
			return false
		}
		for i := range t.fields {
			tf := &t.fields[i]
			vf := &v.fields[i]
			if tf.name.name() != vf.name.name() {
				return false
			}
			if !haveIdenticalType(tf.typ, vf.typ, cmpTags) {
				return false
			}
			if cmpTags && tf.name.tag() != vf.name.tag() {
				return false
			}
			if tf.offsetEmbed != vf.offsetEmbed {
				return false
			}
		}
		return true
	}

	return false
}

// typelinks is implemented in package runtime.
// It returns a slice of the sections in each module,
// and a slice of *rtype offsets in each module.
//
// The types in each module are sorted by string. That is, the first
// two linked types of the first module are:
//
//	d0 := sections[0]
//	t1 := (*rtype)(add(d0, offset[0][0]))
//	t2 := (*rtype)(add(d0, offset[0][1]))
//
// and
//
//	t1.String() < t2.String()
//
// Note that strings are not unique identifiers for types:
// there can be more than one with a given string.
// Only types we might want to look up are included:
// pointers, channels, maps, slices, and arrays.
/**
 * typelinks在runtime包实现。在runtime/symtab.go文件中的
 * moduledata中的typelinks，代表types的偏移量
 * 它在每个模块中返回一部分的切片，并在每个模块中返回*rtype偏移量的切片。
 *
 * 每个模块中的类型均按字符串排序。 即，第一个模块的前两个链接类型是：
 *
 *  d0 := sections[0]
 *  t1 := (*rtype)(add(d0, offset[0][0]))
 *  t2 := (*rtype)(add(d0, offset[0][1]))
 *
 * 和
 *
 *  t1.String() <t2.String()
 *
 * 注意，字符串不是类型的唯一标识符：给定的字符串可以有多个。
 * 仅包括我们可能要查找的类型：指针，通道，映射，切片和数组。
 */
func typelinks() (sections []unsafe.Pointer, offset [][]int32)

/**
 * 求指标的偏移量，并且返回rtype类型的打针
 */
func rtypeOff(section unsafe.Pointer, off int32) *rtype {
	return (*rtype)(add(section, uintptr(off), "sizeof(rtype) > 0"))
}

// typesByString returns the subslice of typelinks() whose elements have
// the given string representation.
// It may be empty (no known types with that string) or may have
// multiple elements (multiple types with that string).
/**
 * typesByString返回typelinks()的子片段，其元素具有给定的字符串表示形式。
 * 它可能为空（该字符串没有已知类型）或可能具有多个元素（该字符串为多个类型）。
 * Question:
 */
func typesByString(s string) []*rtype {
	sections, offset := typelinks()
	var ret []*rtype

	for offsI, offs := range offset {
		section := sections[offsI]

		// We are looking for the first index i where the string becomes >= s.
		// This is a copy of sort.Search, with f(h) replaced by (*typ[h].String() >= s).
		// 我们正在寻找第一个索引i，其中字符串变为 >= s。
        // 这是sort.Search的副本，其中f(h)替换为(* typ [h] .String()> = s)。
        // 二分查找，找满足条件的索引最小的值
		i, j := 0, len(offs)
		for i < j {
			h := i + (j-i)/2 // avoid overflow when computing h
			// i ≤ h < j
			if !(rtypeOff(section, offs[h]).String() >= s) {
				i = h + 1 // preserves f(i-1) == false
			} else {
				j = h // preserves f(j) == true
			}
		}
		// i == j, f(i-1) == false, and f(j) (= f(i)) == true  =>  answer is i.
		// 当i == j, f(i-1) == false, 且 f(j) (= f(i)) == true  ==> i是如找的值

		// Having found the first, linear scan forward to find the last.
		// We could do a second binary search, but the caller is going
		// to do a linear scan anyway.
		// 找到第一个线性向前扫描以找到最后一个。
        // 我们可以进行第二次二进制搜索，但是调用者还是要进行线性扫描。
		for j := i; j < len(offs); j++ {
			typ := rtypeOff(section, offs[j])
			if typ.String() != s { // 从i位置开始匹配，直到第一个类型不是为止
				break
			}
			ret = append(ret, typ)
		}
	}
	return ret
}

// The lookupCache caches ArrayOf, ChanOf, MapOf and SliceOf lookups.
/**
 * lookupCache缓存ArrayOf，ChanOf，MapOf和SliceOf查找结果。
 */
var lookupCache sync.Map // map[cacheKey]*rtype

// A cacheKey is the key for use in the lookupCache.
// Four values describe any of the types we are looking for:
// type kind, one or two subtypes, and an extra integer.
/**
 * cacheKey是在lookupCache中使用的键。
 * 四个值描述了我们正在寻找的任何类型：类型kind，一个或两个子类型以及一个额外的整数。
 */
type cacheKey struct {
	kind  Kind
	t1    *rtype
	t2    *rtype
	extra uintptr
}

// The funcLookupCache caches FuncOf lookups.
// FuncOf does not share the common lookupCache since cacheKey is not
// sufficient to represent functions unambiguously.
/**
 * funcLookupCache缓存FuncOf查找结果。
 * FuncOf不共享公共lookupCache，因为cacheKey不足以明确表示函数。
 */
var funcLookupCache struct {
    // 用于守位m
	sync.Mutex // Guards stores (but not loads) on m.

	// m is a map[uint32][]*rtype keyed by the hash calculated in FuncOf.
	// Elements of m are append-only and thus safe for concurrent reading.
	// m是一个map[uint32][]*rtype，由FuncOf中计算出的哈希值作为键。
    // m的元素是仅追加元素，因此对于并行读取是安全的。
	m sync.Map
}

// ChanOf returns the channel type with the given direction and element type.
// For example, if t represents int, ChanOf(RecvDir, t) represents <-chan int.
//
// The gc runtime imposes a limit of 64 kB on channel element types.
// If t's size is equal to or exceeds this limit, ChanOf panics.
// ChanOf返回具有给定方向和元素类型的通道类型。
// 例如，如果t表示int，则ChanOf(RecvDir，t)表示<-chan int。
//
// gc运行时对通道元素类型施加64kB的限制。
// 如果t的大小等于或超过此限制，ChanOf会恐慌。
func ChanOf(dir ChanDir, t Type) Type {
	typ := t.(*rtype)

	// Look in cache.
	// 在缓存中查找
	ckey := cacheKey{Chan, typ, nil, uintptr(dir)}
	if ch, ok := lookupCache.Load(ckey); ok {
		return ch.(*rtype)
	}

	// This restriction is imposed by the gc compiler and the runtime.
	// 此限制由gc编译器和运行时施加。
	if typ.size >= 1<<16 { // 数据不能超过64kB
		panic("reflect.ChanOf: element size too large")
	}

	// Look in known types.
	// TODO: Precedence when constructing string.
	// 查找已知类型。
    // TODO：构造字符串时的优先级。
	var s string
	switch dir {
	default:
		panic("reflect.ChanOf: invalid dir")
	case SendDir:
		s = "chan<- " + typ.String()
	case RecvDir:
		s = "<-chan " + typ.String()
	case BothDir:
		s = "chan " + typ.String()
	}

	// 根据字符串描述获取类型，并返回第一个在缓存中的值
	for _, tt := range typesByString(s) {
		ch := (*chanType)(unsafe.Pointer(tt))
		if ch.elem == typ && ch.dir == uintptr(dir) {
			ti, _ := lookupCache.LoadOrStore(ckey, tt)
			return ti.(Type)
		}
	}

	// Make a channel type.
	// 创建一种通道类型
	var ichan interface{} = (chan unsafe.Pointer)(nil)
	prototype := *(**chanType)(unsafe.Pointer(&ichan))
	ch := *prototype
	ch.tflag = tflagRegularMemory
	ch.dir = uintptr(dir)
	ch.str = resolveReflectName(newName(s, "", false))
	ch.hash = fnv1(typ.hash, 'c', byte(dir))
	ch.elem = typ

	ti, _ := lookupCache.LoadOrStore(ckey, &ch.rtype)
	return ti.(Type)
}

// MapOf returns the map type with the given key and element types.
// For example, if k represents int and e represents string,
// MapOf(k, e) represents map[int]string.
//
// If the key type is not a valid map key type (that is, if it does
// not implement Go's == operator), MapOf panics.
/**
 * MapOf返回具有给定键和元素类型的Map类型。
 * 例如，如果k表示int而e表示字符串，则MapOfk, e)表示map[int]string。
 *
 * 如果键类型不是有效的Map键类型（即，如果它不实现Go的==运算符），则MapOf会发生恐慌。
 */
func MapOf(key, elem Type) Type {
	ktyp := key.(*rtype)
	etyp := elem.(*rtype)

	if ktyp.equal == nil {
		panic("reflect.MapOf: invalid key type " + ktyp.String())
	}

	// Look in cache.
	// 在缓存中查找
	ckey := cacheKey{Map, ktyp, etyp, 0}
	if mt, ok := lookupCache.Load(ckey); ok {
		return mt.(Type)
	}

	// Look in known types.
	// 查找已知类型
	s := "map[" + ktyp.String() + "]" + etyp.String()
	for _, tt := range typesByString(s) {
		mt := (*mapType)(unsafe.Pointer(tt))
		if mt.key == ktyp && mt.elem == etyp {
			ti, _ := lookupCache.LoadOrStore(ckey, tt)
			return ti.(Type)
		}
	}

	// Make a map type.
	// Note: flag values must match those used in the TMAP case
	// in ../cmd/compile/internal/gc/reflect.go:dtypesym.
	// 设定Map类型。
    // 注意：标志值必须与../cmd/compile/internal/gc/reflect.go:dtypesym中TMAP情况下使用的标志值匹配。
	var imap interface{} = (map[unsafe.Pointer]unsafe.Pointer)(nil)
	mt := **(**mapType)(unsafe.Pointer(&imap))
	mt.str = resolveReflectName(newName(s, "", false))
	mt.tflag = 0
	mt.hash = fnv1(etyp.hash, 'm', byte(ktyp.hash>>24), byte(ktyp.hash>>16), byte(ktyp.hash>>8), byte(ktyp.hash))
	mt.key = ktyp
	mt.elem = etyp
	mt.bucket = bucketOf(ktyp, etyp)
	mt.hasher = func(p unsafe.Pointer, seed uintptr) uintptr {
		return typehash(ktyp, p, seed)
	}
	mt.flags = 0
	if ktyp.size > maxKeySize { // 超出了就是非直接key
		mt.keysize = uint8(ptrSize)
		mt.flags |= 1 // indirect key
	} else {
		mt.keysize = uint8(ktyp.size)
	}
	if etyp.size > maxValSize { // 超出了就是非直接value
		mt.valuesize = uint8(ptrSize)
		mt.flags |= 2 // indirect value
	} else {
		mt.valuesize = uint8(etyp.size)
	}
	mt.bucketsize = uint16(mt.bucket.size)
	// isReflexive报告类型上的==操作是否是自反的。也就是说，对于类型t的所有值x，x == x。
	if isReflexive(ktyp) {
		mt.flags |= 4
	}
	// needKeyUpdate报告是否Map覆盖要求复制key。
	if needKeyUpdate(ktyp) {
		mt.flags |= 8
	}
	// hashMightPanic报告类型为t的映射键的哈希是否可能出现panic情况。
	if hashMightPanic(ktyp) {
		mt.flags |= 16
	}
    // 指向此类型的指针的类型，设置为零
	mt.ptrToThis = 0

	ti, _ := lookupCache.LoadOrStore(ckey, &mt.rtype)
	return ti.(Type)
}

// TODO(crawshaw): as these funcTypeFixedN structs have no methods,
// they could be defined at runtime using the StructOf function.
// TODO(crawshaw):：由于这些funcTypeFixedN结构没有方法，因此可以在运行时使用StructOf函数定义它们。
// 一些固定参数的结构体类型
type funcTypeFixed4 struct {
	funcType
	args [4]*rtype
}
type funcTypeFixed8 struct {
	funcType
	args [8]*rtype
}
type funcTypeFixed16 struct {
	funcType
	args [16]*rtype
}
type funcTypeFixed32 struct {
	funcType
	args [32]*rtype
}
type funcTypeFixed64 struct {
	funcType
	args [64]*rtype
}
type funcTypeFixed128 struct {
	funcType
	args [128]*rtype
}

// FuncOf returns the function type with the given argument and result types.
// For example if k represents int and e represents string,
// FuncOf([]Type{k}, []Type{e}, false) represents func(int) string.
//
// The variadic argument controls whether the function is variadic. FuncOf
// panics if the in[len(in)-1] does not represent a slice and variadic is
// true.
/**
 * FuncOf返回具有给定参数和结果类型的函数类型。
 * 例如，如果k表示int，e表示字符串，则FuncOf([]Type{k}, []Type{e},false）表示func(int) string。
 *
 * variadic参数控制函数是否有可变参数。 如果in[len(in)-1]不代表切片并且可变参数为true，则FuncOf会发生恐慌。
 * @param in 入参
 * @param out 出参
 * @param variadic 否有可变参数
 */
func FuncOf(in, out []Type, variadic bool) Type {
    // 有可变参数类型的情况下
	if variadic && (len(in) == 0 || in[len(in)-1].Kind() != Slice) {
		panic("reflect.FuncOf: last arg of variadic func must be slice")
	}

	// Make a func type.
	// 创建函数类型
	var ifunc interface{} = (func())(nil)
	prototype := *(**funcType)(unsafe.Pointer(&ifunc))
	n := len(in) + len(out) // 总参数个数

	var ft *funcType
	var args []*rtype
	switch { // 根据总有参数个数创建对应类型
	case n <= 4:
		fixed := new(funcTypeFixed4)
		args = fixed.args[:0:len(fixed.args)]
		ft = &fixed.funcType
	case n <= 8:
		fixed := new(funcTypeFixed8)
		args = fixed.args[:0:len(fixed.args)]
		ft = &fixed.funcType
	case n <= 16:
		fixed := new(funcTypeFixed16)
		args = fixed.args[:0:len(fixed.args)]
		ft = &fixed.funcType
	case n <= 32:
		fixed := new(funcTypeFixed32)
		args = fixed.args[:0:len(fixed.args)]
		ft = &fixed.funcType
	case n <= 64:
		fixed := new(funcTypeFixed64)
		args = fixed.args[:0:len(fixed.args)]
		ft = &fixed.funcType
	case n <= 128:
		fixed := new(funcTypeFixed128)
		args = fixed.args[:0:len(fixed.args)]
		ft = &fixed.funcType
	default:
		panic("reflect.FuncOf: too many arguments")
	}
	*ft = *prototype

	// Build a hash and minimally populate ft.
	// 建立一个散列并最少填充ft。
	var hash uint32
	for _, in := range in {
		t := in.(*rtype)
		args = append(args, t)
		hash = fnv1(hash, byte(t.hash>>24), byte(t.hash>>16), byte(t.hash>>8), byte(t.hash))
	}
	if variadic {
		hash = fnv1(hash, 'v')
	}
	hash = fnv1(hash, '.')
	for _, out := range out {
		t := out.(*rtype)
		args = append(args, t)
		hash = fnv1(hash, byte(t.hash>>24), byte(t.hash>>16), byte(t.hash>>8), byte(t.hash))
	}
	// 参数过多
	if len(args) > 50 {
		panic("reflect.FuncOf does not support more than 50 arguments")
	}
	ft.tflag = 0 // 无额外类型信息
	ft.hash = hash
	ft.inCount = uint16(len(in))
	ft.outCount = uint16(len(out))
	if variadic { // 有可变参数，设置最高位
		ft.outCount |= 1 << 15
	}

	// Look in cache.
	// 在缓存中查找
	if ts, ok := funcLookupCache.m.Load(hash); ok {
		for _, t := range ts.([]*rtype) {
			if haveIdenticalUnderlyingType(&ft.rtype, t, true) {
				return t
			}
		}
	}

	// Not in cache, lock and retry.
	// 如果缓存中没有，就锁住缓存，再次尝试
	funcLookupCache.Lock()
	defer funcLookupCache.Unlock()
	if ts, ok := funcLookupCache.m.Load(hash); ok {
		for _, t := range ts.([]*rtype) {
			if haveIdenticalUnderlyingType(&ft.rtype, t, true) {
				return t
			}
		}
	}

    // 向缓存中添加函数的方法
	addToCache := func(tt *rtype) Type {
		var rts []*rtype
		if rti, ok := funcLookupCache.m.Load(hash); ok {
			rts = rti.([]*rtype)
		}
		funcLookupCache.m.Store(hash, append(rts, tt))
		return tt
	}

	// Look in known types for the same string representation.
	// 在已知类型中查找相同的字符串表示形式。
	str := funcStr(ft)
	for _, tt := range typesByString(str) {
		if haveIdenticalUnderlyingType(&ft.rtype, tt, true) {
			return addToCache(tt)
		}
	}

	// Populate the remaining fields of ft and store in cache.
	// 填充ft的其余字段并存储在缓存中。
	ft.str = resolveReflectName(newName(str, "", false))
	ft.ptrToThis = 0
	// 向缓存中添加方法类型并且返回添加值
	return addToCache(&ft.rtype)
}

// funcStr builds a string representation of a funcType.
/**
 * funcStr构建funcType的字符串表示形式。输出类似：func(int, string, ...bool) (int, string, ...bool)
 *
 * @param ft 函数类型
 * @return funcType的字符串表示形式
 **/
func funcStr(ft *funcType) string {
	repr := make([]byte, 0, 64)
	repr = append(repr, "func("...)
	for i, t := range ft.in() { // 处理入参
		if i > 0 {
			repr = append(repr, ", "...)
		}
		if ft.IsVariadic() && i == int(ft.inCount)-1 { // 可变参数
			repr = append(repr, "..."...)
			repr = append(repr, (*sliceType)(unsafe.Pointer(t)).elem.String()...)
		} else {
			repr = append(repr, t.String()...)
		}
	}
	repr = append(repr, ')')
	// 处理出参
	out := ft.out()
	if len(out) == 1 {
		repr = append(repr, ' ')
	} else if len(out) > 1 {
		repr = append(repr, " ("...)
	}
	for i, t := range out {
		if i > 0 {
			repr = append(repr, ", "...)
		}
		repr = append(repr, t.String()...)
	}
	if len(out) > 1 {
		repr = append(repr, ')')
	}
	return string(repr)
}

// isReflexive reports whether the == operation on the type is reflexive.
// That is, x == x for all values x of type t.
/**
 * isReflexive报告类型上的==操作是否自反。 也就是说，对于类型t的所有值x，x == x。
 * @param t 类型
 * @return true: ==操作是自反
 * @date 2020-03-23 08:57:05
 **/
func isReflexive(t *rtype) bool {
	switch t.Kind() {
	case Bool, Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Uintptr, Chan, Ptr, String, UnsafePointer:
		return true
	case Float32, Float64, Complex64, Complex128, Interface:
		return false
	case Array: // 数组类型，判定子类型
		tt := (*arrayType)(unsafe.Pointer(t))
		return isReflexive(tt.elem)
	case Struct: // 结构体类型，判断所有的的字段类型
		tt := (*structType)(unsafe.Pointer(t))
		for _, f := range tt.fields {
			if !isReflexive(f.typ) {
				return false
			}
		}
		return true
	default:
		// Func, Map, Slice, Invalid
		panic("isReflexive called on non-key type " + t.String())
	}
}

// needKeyUpdate reports whether map overwrites require the key to be copied.
/**
 * needKeyUpdate报告Map覆盖是否需要复制key。
 * @param 类型
 * @return true: Map覆盖需要复制key
 **/
func needKeyUpdate(t *rtype) bool {
	switch t.Kind() {
	case Bool, Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Uintptr, Chan, Ptr, UnsafePointer:
		return false
	case Float32, Float64, Complex64, Complex128, Interface, String:
		// Float keys can be updated from +0 to -0.
		// String keys can be updated to use a smaller backing store.
		// Interfaces might have floats of strings in them.
		// float key可以从+0更新到-0。
        // 可以将字符串键更新为使用较小的后备存储。
        // 接口中可能包含字符串浮点数。
		return true
	case Array: // 数组类型，判定子类型
		tt := (*arrayType)(unsafe.Pointer(t))
		return needKeyUpdate(tt.elem)
	case Struct: // 结构体类型，判断所有的的字段类型
		tt := (*structType)(unsafe.Pointer(t))
		for _, f := range tt.fields {
			if needKeyUpdate(f.typ) {
				return true
			}
		}
		return false
	default:
		// Func, Map, Slice, Invalid
		panic("needKeyUpdate called on non-key type " + t.String())
	}
}

// hashMightPanic reports whether the hash of a map key of type t might panic.
/**
 * hashMightPanic报告类型为t的Map key的哈希是否可能出现panic情况。
 * @param
 * @return
 **/
func hashMightPanic(t *rtype) bool {
	switch t.Kind() {
	case Interface: // 接口类型会出现
		return true
	case Array: // 数组类型，判定子类型
		tt := (*arrayType)(unsafe.Pointer(t))
		return hashMightPanic(tt.elem)
	case Struct: // 结构体类型，判断所有的的字段类型
		tt := (*structType)(unsafe.Pointer(t))
		for _, f := range tt.fields {
			if hashMightPanic(f.typ) {
				return true
			}
		}
		return false
	default: // 其他必定不会出现
		return false
	}
}

// Make sure these routines stay in sync with ../../runtime/map.go!
// These types exist only for GC, so we only fill out GC relevant info.
// Currently, that's just size and the GC program. We also fill in string
// for possible debugging use.
/**
 * 确保这些例程与../../runtime/map.go保持同步！
 * 这些类型仅适用于GC，因此我们仅填写与GC相关的信息。
 * 当前，这只是大小和GC程序。 我们还填写字符串以供可能的调试使用。
 */
const (
	bucketSize uintptr = 8
	maxKeySize uintptr = 128
	maxValSize uintptr = 128
)

/**
 * 创建桶类型
 */
func bucketOf(ktyp, etyp *rtype) *rtype {
	if ktyp.size > maxKeySize {
		ktyp = PtrTo(ktyp).(*rtype)
	}
	if etyp.size > maxValSize {
		etyp = PtrTo(etyp).(*rtype)
	}

	// Prepare GC data if any.
	// A bucket is at most bucketSize*(1+maxKeySize+maxValSize)+2*ptrSize bytes,
	// or 2072 bytes, or 259 pointer-size words, or 33 bytes of pointer bitmap.
	// Note that since the key and value are known to be <= 128 bytes,
	// they're guaranteed to have bitmaps instead of GC programs.
	/**
	 * 准备GC数据（如果有）。
     * 一个存储桶最多为bucketSize*(1 + maxKeySize + maxValSize)+ 2 * ptrSize字节，
     * 即2072字节，或259个指针大小的字，或33字节的指针bitmap。
     * 请注意，由于已知键和值<= 128字节，因此可以确保它们具有bitmap而不是GC程序。
	 */
	var gcdata *byte
	var ptrdata uintptr
	var overflowPad uintptr

	size := bucketSize*(1+ktyp.size+etyp.size) + overflowPad + ptrSize
	if size&uintptr(ktyp.align-1) != 0 || size&uintptr(etyp.align-1) != 0 {
		panic("reflect: bad size computation in MapOf")
	}

	if ktyp.ptrdata != 0 || etyp.ptrdata != 0 {
		nptr := (bucketSize*(1+ktyp.size+etyp.size) + ptrSize) / ptrSize
		mask := make([]byte, (nptr+7)/8)
		base := bucketSize / ptrSize

		if ktyp.ptrdata != 0 {
			emitGCMask(mask, base, ktyp, bucketSize)
		}
		base += bucketSize * ktyp.size / ptrSize

		if etyp.ptrdata != 0 {
			emitGCMask(mask, base, etyp, bucketSize)
		}
		base += bucketSize * etyp.size / ptrSize
		base += overflowPad / ptrSize

		word := base
		mask[word/8] |= 1 << (word % 8)
		gcdata = &mask[0]
		ptrdata = (word + 1) * ptrSize

		// overflow word must be last
		// 溢出字必须是最后一个
		if ptrdata != size {
			panic("reflect: bad layout computation in MapOf")
		}
	}

	b := &rtype{
		align:   ptrSize,
		size:    size,
		kind:    uint8(Struct),
		ptrdata: ptrdata,
		gcdata:  gcdata,
	}
	if overflowPad > 0 {
		b.align = 8
	}
	s := "bucket(" + ktyp.String() + "," + etyp.String() + ")"
	b.str = resolveReflectName(newName(s, "", false))
	return b
}

/**
 * 获取gc字节切片
 */
func (t *rtype) gcSlice(begin, end uintptr) []byte {
	return (*[1 << 30]byte)(unsafe.Pointer(t.gcdata))[begin:end:end]
}

// emitGCMask writes the GC mask for [n]typ into out, starting at bit
// offset base.
/**
 * emitGCMask将[n] typ的GC掩码从位偏移量的基数开始写出到out中。
 */
func emitGCMask(out []byte, base uintptr, typ *rtype, n uintptr) {
	if typ.kind&kindGCProg != 0 {
		panic("reflect: unexpected GC program")
	}
	ptrs := typ.ptrdata / ptrSize
	words := typ.size / ptrSize
	mask := typ.gcSlice(0, (ptrs+7)/8)
	for j := uintptr(0); j < ptrs; j++ {
		if (mask[j/8]>>(j%8))&1 != 0 {
			for i := uintptr(0); i < n; i++ {
				k := base + i*words + j
				out[k/8] |= 1 << (k % 8)
			}
		}
	}
}

// appendGCProg appends the GC program for the first ptrdata bytes of
// typ to dst and returns the extended slice.
/**
 * appendGCProg将typ的第一个ptrdata字节的GC程序追加到dst，并返回扩展的切片。
 */
func appendGCProg(dst []byte, typ *rtype) []byte {
	if typ.kind&kindGCProg != 0 {
		// Element has GC program; emit one element.
		// 元素具有GC程序； 发出一个元素。
		n := uintptr(*(*uint32)(unsafe.Pointer(typ.gcdata)))
		prog := typ.gcSlice(4, 4+n-1)
		return append(dst, prog...)
	}

	// Element is small with pointer mask; use as literal bits.
	// 元素很小，带有指针mask； 用作literal bits。
	ptrs := typ.ptrdata / ptrSize
	mask := typ.gcSlice(0, (ptrs+7)/8)

	// Emit 120-bit chunks of full bytes (max is 127 but we avoid using partial bytes).
	// 发射完整字节的120位块（最大为127，但我们避免使用部分字节）。
	for ; ptrs > 120; ptrs -= 120 {
		dst = append(dst, 120)
		dst = append(dst, mask[:15]...)
		mask = mask[15:]
	}

	dst = append(dst, byte(ptrs))
	dst = append(dst, mask...)
	return dst
}

// SliceOf returns the slice type with element type t.
// For example, if t represents int, SliceOf(t) represents []int.
/**
 * SliceOf返回元素类型为t的切片类型。
 * 例如，如果t表示int，则SliceOf(t)表示[]int。
 */
func SliceOf(t Type) Type {
	typ := t.(*rtype)

	// Look in cache.
	// 在缓存中查找。
	ckey := cacheKey{Slice, typ, nil, 0}
	if slice, ok := lookupCache.Load(ckey); ok {
		return slice.(Type)
	}

	// Look in known types.
	// 查找已知类型。
	s := "[]" + typ.String()
	for _, tt := range typesByString(s) {
		slice := (*sliceType)(unsafe.Pointer(tt))
		if slice.elem == typ {
			ti, _ := lookupCache.LoadOrStore(ckey, tt)
			return ti.(Type)
		}
	}

	// Make a slice type.
	// 创建一种切片类型
	var islice interface{} = ([]unsafe.Pointer)(nil)
	prototype := *(**sliceType)(unsafe.Pointer(&islice))
	slice := *prototype
	slice.tflag = 0
	slice.str = resolveReflectName(newName(s, "", false))
	slice.hash = fnv1(typ.hash, '[')
	slice.elem = typ
	slice.ptrToThis = 0

	ti, _ := lookupCache.LoadOrStore(ckey, &slice.rtype)
	return ti.(Type)
}

// The structLookupCache caches StructOf lookups.
// StructOf does not share the common lookupCache since we need to pin
// the memory associated with *structTypeFixedN.
/**
 * structLookupCache缓存StructOf查找结果。
 * StructOf不共享公共lookupCache，因为我们需要固定与*structTypeFixedN关联的内存。
 */
var structLookupCache struct {
	sync.Mutex // Guards stores (but not loads) on m. // 用于守位m，Question: 具体怎么守位

	// m is a map[uint32][]Type keyed by the hash calculated in StructOf.
	// Elements in m are append-only and thus safe for concurrent reading.
	// m是一个Map [uint32] []类型，由在StructOf中计算出的哈希值作为键。
    // m中的元素是仅追加元素，因此可以安全地进行并行读取。
	m sync.Map
}

/**
 * 结构体非平凡类型
 */
type structTypeUncommon struct {
	structType
	u uncommonType
}

// isLetter reports whether a given 'rune' is classified as a Letter.
/**
 * isLetter报告给定的“符文”是否归类为字母。
 */
func isLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' || ch >= utf8.RuneSelf && unicode.IsLetter(ch)
}

// isValidFieldName checks if a string is a valid (struct) field name or not.
//
// According to the language spec, a field name should be an identifier.
//
// identifier = letter { letter | unicode_digit } .
// letter = unicode_letter | "_" .
/**
 * isValidFieldName检查字符串是否为有效的（结构）字段名称。
 *
 * 根据语言规范，字段名称应为标识符。
 *
 * 标识符 = 字母{字母|unicode数字}。
 * 字母 = unicode字母|"_"。
 */
func isValidFieldName(fieldName string) bool {
	for i, c := range fieldName {
		if i == 0 && !isLetter(c) {
			return false
		}

		if !(isLetter(c) || unicode.IsDigit(c)) {
			return false
		}
	}

	return len(fieldName) > 0
}

// StructOf returns the struct type containing fields.
// The Offset and Index fields are ignored and computed as they would be
// by the compiler.
//
// StructOf currently does not generate wrapper methods for embedded
// fields and panics if passed unexported StructFields.
// These limitations may be lifted in a future version.
/**
 * StructOf返回包含字段的结构类型。
 * 偏移量和索引字段将被忽略并按照编译器的方式进行计算。
 *
 * StructOf当前不为嵌入式字段生成包装方法，并且如果传递未导出的StructFields，则会发生恐慌。
 * 这些限制可能会在将来的版本中取消。
 * TODO 需要进一步研究
 */
func StructOf(fields []StructField) Type {
	var (
		hash       = fnv1(0, []byte("struct {")...)
		size       uintptr
		typalign   uint8 // 对齐类型
		comparable = true
		methods    []method

		fs   = make([]structField, len(fields))
		repr = make([]byte, 0, 64)
		fset = map[string]struct{}{} // fields' names

		hasGCProg = false // records whether a struct-field type has a GCProg // 记录结构字段类型是否具有GCProg
	)

	lastzero := uintptr(0)
	repr = append(repr, "struct {"...)
	pkgpath := ""
	for i, field := range fields { // 处理每一个字段
		if field.Name == "" { // 字段名为空
			panic("reflect.StructOf: field " + strconv.Itoa(i) + " has no name")
		}
		if !isValidFieldName(field.Name) { // 字段名不合法
			panic("reflect.StructOf: field " + strconv.Itoa(i) + " has invalid name")
		}
		if field.Type == nil { // 字段是nil类型
			panic("reflect.StructOf: field " + strconv.Itoa(i) + " has no type")
		}
		f, fpkgpath := runtimeStructField(field)
		ft := f.typ
		if ft.kind&kindGCProg != 0 {
			hasGCProg = true
		}
		if fpkgpath != "" { // 获取包路径
			if pkgpath == "" {
				pkgpath = fpkgpath
			} else if pkgpath != fpkgpath {
				panic("reflect.Struct: fields with different PkgPath " + pkgpath + " and " + fpkgpath)
			}
		}

		// Update string and hash
		// 更新字符串和哈希
		name := f.name.name()
		hash = fnv1(hash, []byte(name)...)
		repr = append(repr, (" " + name)...)
		if f.embedded() {  // 如果是嵌入类型
			// Embedded field
			// 嵌入字段
			if f.typ.Kind() == Ptr { // 是指针类型
				// Embedded ** and *interface{} are illegal
				// 嵌入式**和*interface{}是非法的
				elem := ft.Elem()
				// 元素类型不能是指针和接口
				if k := elem.Kind(); k == Ptr || k == Interface {
					panic("reflect.StructOf: illegal embedded field type " + ft.String())
				}
			}

			switch f.typ.Kind() {
			case Interface: // 接口类型
				ift := (*interfaceType)(unsafe.Pointer(ft))
				for im, m := range ift.methods { // 处理接口的每个方法
					if ift.nameOff(m.name).pkgPath() != "" { // 包名为空
						// TODO(sbinet).  Issue 15924.
						panic("reflect: embedded interface with unexported method(s) not implemented")
					}

					var (
						mtyp    = ift.typeOff(m.typ)
						ifield  = i
						imethod = im
						ifn     Value
						tfn     Value
					)

					if ft.kind&kindDirectIface != 0 {
						tfn = MakeFunc(mtyp, func(in []Value) []Value {
							var args []Value
							var recv = in[0]
							if len(in) > 1 {
								args = in[1:]
							}
							return recv.Field(ifield).Method(imethod).Call(args)
						})
						ifn = MakeFunc(mtyp, func(in []Value) []Value {
							var args []Value
							var recv = in[0]
							if len(in) > 1 {
								args = in[1:]
							}
							return recv.Field(ifield).Method(imethod).Call(args)
						})
					} else {
						tfn = MakeFunc(mtyp, func(in []Value) []Value {
							var args []Value
							var recv = in[0]
							if len(in) > 1 {
								args = in[1:]
							}
							return recv.Field(ifield).Method(imethod).Call(args)
						})
						ifn = MakeFunc(mtyp, func(in []Value) []Value {
							var args []Value
							var recv = Indirect(in[0])
							if len(in) > 1 {
								args = in[1:]
							}
							return recv.Field(ifield).Method(imethod).Call(args)
						})
					}

					methods = append(methods, method{
						name: resolveReflectName(ift.nameOff(m.name)),
						mtyp: resolveReflectType(mtyp),
						ifn:  resolveReflectText(unsafe.Pointer(&ifn)),
						tfn:  resolveReflectText(unsafe.Pointer(&tfn)),
					})
				}
			case Ptr: // 指针类型
				ptr := (*ptrType)(unsafe.Pointer(ft))
				if unt := ptr.uncommon(); unt != nil {
					if i > 0 && unt.mcount > 0 {
						// Issue 15924.
						panic("reflect: embedded type with methods not implemented if type is not first field")
					}
					if len(fields) > 1 {
						panic("reflect: embedded type with methods not implemented if there is more than one field")
					}
					for _, m := range unt.methods() {
						mname := ptr.nameOff(m.name)
						if mname.pkgPath() != "" {
							// TODO(sbinet).
							// Issue 15924.
							panic("reflect: embedded interface with unexported method(s) not implemented")
						}
						methods = append(methods, method{
							name: resolveReflectName(mname),
							mtyp: resolveReflectType(ptr.typeOff(m.mtyp)),
							ifn:  resolveReflectText(ptr.textOff(m.ifn)),
							tfn:  resolveReflectText(ptr.textOff(m.tfn)),
						})
					}
				}
				if unt := ptr.elem.uncommon(); unt != nil {
					for _, m := range unt.methods() {
						mname := ptr.nameOff(m.name)
						if mname.pkgPath() != "" {
							// TODO(sbinet)
							// Issue 15924.
							panic("reflect: embedded interface with unexported method(s) not implemented")
						}
						methods = append(methods, method{
							name: resolveReflectName(mname),
							mtyp: resolveReflectType(ptr.elem.typeOff(m.mtyp)),
							ifn:  resolveReflectText(ptr.elem.textOff(m.ifn)),
							tfn:  resolveReflectText(ptr.elem.textOff(m.tfn)),
						})
					}
				}
			default:
				if unt := ft.uncommon(); unt != nil {
					if i > 0 && unt.mcount > 0 {
						// Issue 15924.
						panic("reflect: embedded type with methods not implemented if type is not first field")
					}
					if len(fields) > 1 && ft.kind&kindDirectIface != 0 {
						panic("reflect: embedded type with methods not implemented for non-pointer type")
					}
					for _, m := range unt.methods() {
						mname := ft.nameOff(m.name)
						if mname.pkgPath() != "" {
							// TODO(sbinet)
							// Issue 15924.
							panic("reflect: embedded interface with unexported method(s) not implemented")
						}
						methods = append(methods, method{
							name: resolveReflectName(mname),
							mtyp: resolveReflectType(ft.typeOff(m.mtyp)),
							ifn:  resolveReflectText(ft.textOff(m.ifn)),
							tfn:  resolveReflectText(ft.textOff(m.tfn)),
						})

					}
				}
			}
		}
		if _, dup := fset[name]; dup {
			panic("reflect.StructOf: duplicate field " + name)
		}
		fset[name] = struct{}{}

		hash = fnv1(hash, byte(ft.hash>>24), byte(ft.hash>>16), byte(ft.hash>>8), byte(ft.hash))

		repr = append(repr, (" " + ft.String())...)
		if f.name.tagLen() > 0 {
			hash = fnv1(hash, []byte(f.name.tag())...)
			repr = append(repr, (" " + strconv.Quote(f.name.tag()))...)
		}
		if i < len(fields)-1 {
			repr = append(repr, ';')
		}

		comparable = comparable && (ft.equal != nil)

		offset := align(size, uintptr(ft.align))
		if ft.align > typalign {
			typalign = ft.align
		}
		size = offset + ft.size
		f.offsetEmbed |= offset << 1

		if ft.size == 0 {
			lastzero = size
		}

		fs[i] = f
	}

	if size > 0 && lastzero == size {
		// This is a non-zero sized struct that ends in a
		// zero-sized field. We add an extra byte of padding,
		// to ensure that taking the address of the final
		// zero-sized field can't manufacture a pointer to the
		// next object in the heap. See issue 9401.
		/**
		 * 这是一个非零大小的结构，以零大小的字段结尾。 我们添加了一个额外的填充字节，
		 * 以确保获取最后一个零大小字段的地址不会产生指向堆中下一个对象的指针。 
		 * 请参阅 issue 9401。
		 */
		size++
	}

	var typ *structType
	var ut *uncommonType

	if len(methods) == 0 {
		t := new(structTypeUncommon)
		typ = &t.structType
		ut = &t.u
	} else {
		// A *rtype representing a struct is followed directly in memory by an
		// array of method objects representing the methods attached to the
		// struct. To get the same layout for a run time generated type, we
		// need an array directly following the uncommonType memory.
		// A similar strategy is used for funcTypeFixed4, ...funcTypeFixedN.
        /**
         * 代表结构的* rtype在内存中直接跟随着表示附加到该结构的方法的方法对象数组。
         * 为了使运行时生成的类型具有相同的布局，我们需要紧跟在uncommonType内存之后的数组。
         * 类似的策略用于funcTypeFixed4，... funcTypeFixedN。
         */
		tt := New(StructOf([]StructField{
			{Name: "S", Type: TypeOf(structType{})},
			{Name: "U", Type: TypeOf(uncommonType{})},
			{Name: "M", Type: ArrayOf(len(methods), TypeOf(methods[0]))},
		}))

		typ = (*structType)(unsafe.Pointer(tt.Elem().Field(0).UnsafeAddr()))
		ut = (*uncommonType)(unsafe.Pointer(tt.Elem().Field(1).UnsafeAddr()))

		copy(tt.Elem().Field(2).Slice(0, len(methods)).Interface().([]method), methods)
	}
	// TODO(sbinet): Once we allow embedding multiple types,
	// methods will need to be sorted like the compiler does.
	// TODO(sbinet): Once we allow non-exported methods, we will
	// need to compute xcount as the number of exported methods.
	// TODO（sbinet）: 一旦我们允许嵌入多个类型，就需要像编译器一样对方法进行排序。
    // TODO（sbinet）: 一旦允许非导出方法，我们将需要计算xcount作为导出方法的数量。
	ut.mcount = uint16(len(methods))
	ut.xcount = ut.mcount
	ut.moff = uint32(unsafe.Sizeof(uncommonType{}))

	if len(fs) > 0 {
		repr = append(repr, ' ')
	}
	repr = append(repr, '}')
	hash = fnv1(hash, '}')
	str := string(repr)

	// Round the size up to be a multiple of the alignment.
	// 将大小舍入为对齐的倍数。
	size = align(size, uintptr(typalign))

	// Make the struct type.
	var istruct interface{} = struct{}{}
	prototype := *(**structType)(unsafe.Pointer(&istruct))
	*typ = *prototype
	typ.fields = fs
	if pkgpath != "" {
		typ.pkgPath = newName(pkgpath, "", false)
	}

	// Look in cache.
	if ts, ok := structLookupCache.m.Load(hash); ok {
		for _, st := range ts.([]Type) {
			t := st.common()
			if haveIdenticalUnderlyingType(&typ.rtype, t, true) {
				return t
			}
		}
	}

	// Not in cache, lock and retry.
	structLookupCache.Lock()
	defer structLookupCache.Unlock()
	if ts, ok := structLookupCache.m.Load(hash); ok {
		for _, st := range ts.([]Type) {
			t := st.common()
			if haveIdenticalUnderlyingType(&typ.rtype, t, true) {
				return t
			}
		}
	}

	addToCache := func(t Type) Type {
		var ts []Type
		if ti, ok := structLookupCache.m.Load(hash); ok {
			ts = ti.([]Type)
		}
		structLookupCache.m.Store(hash, append(ts, t))
		return t
	}

	// Look in known types.
	for _, t := range typesByString(str) {
		if haveIdenticalUnderlyingType(&typ.rtype, t, true) {
			// even if 't' wasn't a structType with methods, we should be ok
			// as the 'u uncommonType' field won't be accessed except when
			// tflag&tflagUncommon is set.
			// 即使't'不是带有方法的structType，也应该没问题，因为除非设置了
			// tflag＆tflagUncommon，否则不会访问'u uncommonType'字段。
			return addToCache(t)
		}
	}

	typ.str = resolveReflectName(newName(str, "", false))
	typ.tflag = 0 // TODO: set tflagRegularMemory
	typ.hash = hash
	typ.size = size
	typ.ptrdata = typeptrdata(typ.common())
	typ.align = typalign
	typ.fieldAlign = typalign
	typ.ptrToThis = 0
	if len(methods) > 0 {
		typ.tflag |= tflagUncommon
	}

	if hasGCProg {
		lastPtrField := 0
		for i, ft := range fs {
			if ft.typ.pointers() {
				lastPtrField = i
			}
		}
		prog := []byte{0, 0, 0, 0} // will be length of prog // 将是prog的长度
		var off uintptr
		for i, ft := range fs {
			if i > lastPtrField {
				// gcprog should not include anything for any field after
				// the last field that contains pointer data
				// gcprog不应在包含指针数据的最后一个字段之后的任何字段中包含任何内容
				break
			}
			if !ft.typ.pointers() {
				// Ignore pointerless fields.
				// 忽略无指针字段。
				continue
			}
			// Pad to start of this field with zeros.
			// 用零填充该字段的开头。
			if ft.offset() > off {
				n := (ft.offset() - off) / ptrSize
				prog = append(prog, 0x01, 0x00) // emit a 0 bit // 发出0位
				if n > 1 {
					prog = append(prog, 0x81)      // repeat previous bit // 重复上一位
					prog = appendVarint(prog, n-1) // n-1 times
				}
				off = ft.offset()
			}

			prog = appendGCProg(prog, ft.typ)
			off += ft.typ.ptrdata
		}
		prog = append(prog, 0)
		*(*uint32)(unsafe.Pointer(&prog[0])) = uint32(len(prog) - 4)
		typ.kind |= kindGCProg
		typ.gcdata = &prog[0]
	} else {
		typ.kind &^= kindGCProg
		bv := new(bitVector)
		addTypeBits(bv, 0, typ.common())
		if len(bv.data) > 0 {
			typ.gcdata = &bv.data[0]
		}
	}
	typ.equal = nil
	if comparable {
		typ.equal = func(p, q unsafe.Pointer) bool {
			for _, ft := range typ.fields {
				pi := add(p, ft.offset(), "&x.field safe")
				qi := add(q, ft.offset(), "&x.field safe")
				if !ft.typ.equal(pi, qi) {
					return false
				}
			}
			return true
		}
	}

	switch {
	case len(fs) == 1 && !ifaceIndir(fs[0].typ):
		// structs of 1 direct iface type can be direct
		typ.kind |= kindDirectIface
	default:
		typ.kind &^= kindDirectIface
	}

	return addToCache(&typ.rtype)
}

// runtimeStructField takes a StructField value passed to StructOf and
// returns both the corresponding internal representation, of type
// structField, and the pkgpath value to use for this field.
/**
 * runtimeStructField接受传递给StructOf的StructField值，
 * 并返回相应的内部表示形式structField和用于该字段的pkgpath值。
 * @param
 * @return
 **/
func runtimeStructField(field StructField) (structField, string) {
    // 无包名的匿名字段
	if field.Anonymous && field.PkgPath != "" {
		panic("reflect.StructOf: field \"" + field.Name + "\" is anonymous but has PkgPath set")
	}

	exported := field.PkgPath == ""
	if exported {
		// Best-effort check for misuse.
		// Since this field will be treated as exported, not much harm done if Unicode lowercase slips through.
		// 尽最大努力检查滥用情况。 由于此字段将被视为导出字段，因此如果Unicode小写漏掉，不会造成太大危害。
		c := field.Name[0]
		if 'a' <= c && c <= 'z' || c == '_' {
			panic("reflect.StructOf: field \"" + field.Name + "\" is unexported but missing PkgPath")
		}
	}

	offsetEmbed := uintptr(0)
	if field.Anonymous {
		offsetEmbed |= 1
	}

	resolveReflectType(field.Type.common()) // install in runtime
	f := structField{
		name:        newName(field.Name, string(field.Tag), exported),
		typ:         field.Type.common(),
		offsetEmbed: offsetEmbed,
	}
	return f, field.PkgPath
}

// typeptrdata returns the length in bytes of the prefix of t
// containing pointer data. Anything after this offset is scalar data.
// keep in sync with ../cmd/compile/internal/gc/reflect.go
/**
 * typeptrdata返回包含指针数据的t前缀的长度（以字节为单位）。
 * 此偏移量之后的所有内容均为标量数据。 与../cmd/compile/internal/gc/reflect.go保持同步
 * @return 包含指针数据的t前缀的长度，此偏移量之后的所有内容均为标量数据。
 */
func typeptrdata(t *rtype) uintptr {
	switch t.Kind() {
	case Struct:
		st := (*structType)(unsafe.Pointer(t))
		// find the last field that has pointers.
		// 查找具有指针的最后一个字段。
		field := -1
		for i := range st.fields {
			ft := st.fields[i].typ
			if ft.pointers() {
				field = i
			}
		}
		if field == -1 {
			return 0
		}
		f := st.fields[field]
		return f.offset() + f.typ.ptrdata

	default:
		panic("reflect.typeptrdata: unexpected type, " + t.String())
	}
}

// See cmd/compile/internal/gc/reflect.go for derivation of constant.
// 有关常量的派生，请参见cmd/compile/internal/gc/reflect.go。
const maxPtrmaskBytes = 2048

// ArrayOf returns the array type with the given count and element type.
// For example, if t represents int, ArrayOf(5, t) represents [5]int.
//
// If the resulting type would be larger than the available address space,
// ArrayOf panics.
/**
 * ArrayOf返回具有给定计数和元素类型的数组类型。
 * 例如，如果t表示int，则ArrayOf（5，t）表示[5] int。
 *
 * 如果结果类型大于可用的地址空间，
 *  ArrayOf恐慌。
 */
func ArrayOf(count int, elem Type) Type {
	typ := elem.(*rtype)

	// Look in cache.
	// 在缓存中查找
	ckey := cacheKey{Array, typ, nil, uintptr(count)}
	if array, ok := lookupCache.Load(ckey); ok {
		return array.(Type)
	}

	// Look in known types.
	// 在已知类型中查找相同的字符串表示形式
	s := "[" + strconv.Itoa(count) + "]" + typ.String()
	for _, tt := range typesByString(s) {
		array := (*arrayType)(unsafe.Pointer(tt))
		if array.elem == typ {
			ti, _ := lookupCache.LoadOrStore(ckey, tt)
			return ti.(Type)
		}
	}

	// Make an array type.
	// 创建数组类型
	var iarray interface{} = [1]unsafe.Pointer{}
	prototype := *(**arrayType)(unsafe.Pointer(&iarray))
	array := *prototype
	array.tflag = typ.tflag & tflagRegularMemory
	array.str = resolveReflectName(newName(s, "", false))
	array.hash = fnv1(typ.hash, '[')
	for n := uint32(count); n > 0; n >>= 8 {
		array.hash = fnv1(array.hash, byte(n))
	}
	array.hash = fnv1(array.hash, ']')
	array.elem = typ
	array.ptrToThis = 0
	if typ.size > 0 {
		max := ^uintptr(0) / typ.size
		if uintptr(count) > max { // 不能大于整个虚拟地址空间可表示的数组大小，虚拟地址空间根据机器的不同不一样
			panic("reflect.ArrayOf: array size would exceed virtual address space")
		}
	}
	array.size = typ.size * uintptr(count) // 数组的大小
	if count > 0 && typ.ptrdata != 0 { // 元素大于0，并且指针数据不为0
	    // Question: 这个原理是什么？
		array.ptrdata = typ.size*uintptr(count-1) + typ.ptrdata
	}
	array.align = typ.align
	array.fieldAlign = typ.fieldAlign
	array.len = uintptr(count)
	array.slice = SliceOf(elem).(*rtype)

	switch {
	case typ.ptrdata == 0 || array.size == 0: // 无指针或者数组长度为0
		// No pointers.
		array.gcdata = nil
		array.ptrdata = 0

	case count == 1:
		// In memory, 1-element array looks just like the element.
		// 在内存中，1元素数组看起来就像元素。
		array.kind |= typ.kind & kindGCProg
		array.gcdata = typ.gcdata
		array.ptrdata = typ.ptrdata

	case typ.kind&kindGCProg == 0 && array.size <= maxPtrmaskBytes*8*ptrSize:
		// Element is small with pointer mask; array is still small.
		// Create direct pointer mask by turning each 1 bit in elem
		// into count 1 bits in larger mask.
		// 元素很小，带有指针遮罩； 数组仍然很小。
        // 通过将elem中的每个1位转换为较大掩码中的1个位来创建直接指针掩码。
		mask := make([]byte, (array.ptrdata/ptrSize+7)/8)
		emitGCMask(mask, 0, typ, array.len)
		array.gcdata = &mask[0]

	default:
		// Create program that emits one element
		// and then repeats to make the array.
		// 创建发出一个元素然后重复进行以生成数组的程序。
		prog := []byte{0, 0, 0, 0} // will be length of prog
		prog = appendGCProg(prog, typ)
		// Pad from ptrdata to size.
		elemPtrs := typ.ptrdata / ptrSize
		elemWords := typ.size / ptrSize
		if elemPtrs < elemWords {
			// Emit literal 0 bit, then repeat as needed.
			prog = append(prog, 0x01, 0x00)
			if elemPtrs+1 < elemWords {
				prog = append(prog, 0x81)
				prog = appendVarint(prog, elemWords-elemPtrs-1)
			}
		}
		// Repeat count-1 times.
		// 重复count-1次
		if elemWords < 0x80 {
			prog = append(prog, byte(elemWords|0x80))
		} else {
			prog = append(prog, 0x80)
			prog = appendVarint(prog, elemWords)
		}
		prog = appendVarint(prog, uintptr(count)-1)
		prog = append(prog, 0)
		*(*uint32)(unsafe.Pointer(&prog[0])) = uint32(len(prog) - 4)
		array.kind |= kindGCProg
		array.gcdata = &prog[0]
		// 高估但还可以 必须匹配程序
		array.ptrdata = array.size // overestimate but ok; must match program
	}

	etyp := typ.common()
	esize := etyp.Size()

	array.equal = nil
	if eequal := etyp.equal; eequal != nil {
		array.equal = func(p, q unsafe.Pointer) bool {
			for i := 0; i < count; i++ {
				pi := arrayAt(p, i, esize, "i < count")
				qi := arrayAt(q, i, esize, "i < count")
				if !eequal(pi, qi) {
					return false
				}

			}
			return true
		}
	}

	switch {
	case count == 1 && !ifaceIndir(typ):
		// array of 1 direct iface type can be direct
		// 1个直接iface类型的数组可以是直接的
		array.kind |= kindDirectIface
	default:
		array.kind &^= kindDirectIface
	}

	ti, _ := lookupCache.LoadOrStore(ckey, &array.rtype)
	return ti.(Type)
}

func appendVarint(x []byte, v uintptr) []byte {
	for ; v >= 0x80; v >>= 7 {
		x = append(x, byte(v|0x80))
	}
	x = append(x, byte(v))
	return x
}

// toType converts from a *rtype to a Type that can be returned
// to the client of package reflect. In gc, the only concern is that
// a nil *rtype must be replaced by a nil Type, but in gccgo this
// function takes care of ensuring that multiple *rtype for the same
// type are coalesced into a single Type.
/**
 * toType从*rtype转换为可以返回给package反射客户端的Type。
 * 在gc中，唯一需要注意的是必须将nil *rtype替换为nil Type，但是在gccgo中，
 * 此函数将确保将同一类型的多个*rtype合并为单个Type。
 */
func toType(t *rtype) Type {
	if t == nil {
		return nil
	}
    // Question: 没有类型转换，直接返回
	return t
}

/**
 * 函数类型的底层key
 */
type layoutKey struct {
	ftyp *funcType // function signature // 函数签名
	rcvr *rtype    // receiver type, or nil if none // 接收者类型
}

/**
 * 函数类型的底层value
 */
type layoutType struct {
	t         *rtype
	argSize   uintptr // size of arguments // 参数大小，
	retOffset uintptr // offset of return values. // 返回值的偏移量
	stack     *bitVector
	framePool *sync.Pool
}

// 用于函数底层的缓存
var layoutCache sync.Map // map[layoutKey]layoutType

// funcLayout computes a struct type representing the layout of the
// function arguments and return values for the function type t.
// If rcvr != nil, rcvr specifies the type of the receiver.
// The returned type exists only for GC, so we only fill out GC relevant info.
// Currently, that's just size and the GC program. We also fill in
// the name for possible debugging use.
/**
 * funcLayout计算一个表示函数参数布局和函数类型t的返回值的结构类型。
 * 如果rcvr != nil，则rcvr指定接收方的类型。
 * 返回的类型仅适用于GC，因此我们仅填写与GC相关的信息。
 * 当前，这只是大小和GC程序。 我们还填写该名称，以供可能的调试使用。
 * @param t 函数类型
 * @param rcvr 接收者类型
 * @return frametype 帧类型
 * @return argSize 参数大小
 * @return retOffset 返回值的偏移量
 * @return stk 位向量
 * @return framePool 帧的缓存池
 */
func funcLayout(t *funcType, rcvr *rtype) (frametype *rtype, argSize, retOffset uintptr, stk *bitVector, framePool *sync.Pool) {
	if t.Kind() != Func { // 不是方法
		panic("reflect: funcLayout of non-func type " + t.String())
	}
	if rcvr != nil && rcvr.Kind() == Interface { // 接收者不为空，并且接收者是接口
		panic("reflect: funcLayout with interface receiver " + rcvr.String())
	}

	k := layoutKey{t, rcvr}
	if lti, ok := layoutCache.Load(k); ok {
		lt := lti.(layoutType)
		return lt.t, lt.argSize, lt.retOffset, lt.stack, lt.framePool
	}

	// compute gc program & stack bitmap for arguments
	// 计算gc程序和堆栈位图出参
	ptrmap := new(bitVector)
	var offset uintptr
	if rcvr != nil {
		// Reflect uses the "interface" calling convention for
		// methods, where receivers take one word of argument
		// space no matter how big they actually are.
		// Reflect使用“接口”调用约定作为方法，接收者无论实际大小如何都占用一个参数空间。
		if ifaceIndir(rcvr) || rcvr.pointers() { // 接口或者指针
			ptrmap.append(1)
		} else {
			ptrmap.append(0)
		}
		offset += ptrSize // const ptrSize = 4 << (^uintptr(0) >> 63) ==> 0b00010000
	}

	// 处理入参
	for _, arg := range t.in() {
		offset += -offset & uintptr(arg.align-1)
		addTypeBits(ptrmap, offset, arg)
		offset += arg.size
	}
	argSize = offset
	offset += -offset & (ptrSize - 1)
	retOffset = offset
	for _, res := range t.out() {
		offset += -offset & uintptr(res.align-1)
		addTypeBits(ptrmap, offset, res)
		offset += res.size
	}
	offset += -offset & (ptrSize - 1)

	// build dummy rtype holding gc program
	// 建立虚拟rtype持有gc程序
	x := &rtype{
		align:   ptrSize,
		size:    offset,
		ptrdata: uintptr(ptrmap.n) * ptrSize,
	}
	if ptrmap.n > 0 {
		x.gcdata = &ptrmap.data[0]
	}

	var s string
	if rcvr != nil {
		s = "methodargs(" + rcvr.String() + ")(" + t.String() + ")"
	} else {
		s = "funcargs(" + t.String() + ")"
	}
	x.str = resolveReflectName(newName(s, "", false))

	// cache result for future callers
    // 缓存结果以供将来的使用
	framePool = &sync.Pool{New: func() interface{} {
		return unsafe_New(x)
	}}
	lti, _ := layoutCache.LoadOrStore(k, layoutType{
		t:         x,
		argSize:   argSize,
		retOffset: retOffset,
		stack:     ptrmap,
		framePool: framePool,
	})
	lt := lti.(layoutType)
	return lt.t, lt.argSize, lt.retOffset, lt.stack, lt.framePool
}

// ifaceIndir reports whether t is stored indirectly in an interface value.
/**
 * ifaceIndir报告t是否间接存储在接口值中。
 */
func ifaceIndir(t *rtype) bool {
	return t.kind&kindDirectIface == 0
}

// 位向量
type bitVector struct {
	n    uint32 // number of bits // 位数
	data []byte
}

// append a bit to the bitmap.
// 向位图中添加一个位，uint8其实只使用了最低位，值为0或者1
func (bv *bitVector) append(bit uint8) {
	if bv.n%8 == 0 { // 位刚好用完了，扩容一个字节
		bv.data = append(bv.data, 0)
	}
	bv.data[bv.n/8] |= bit << (bv.n % 8) // 将位添加到指定位置
	bv.n++
}

/**
 * 为类型t添加位信息到位图bv中
 */
func addTypeBits(bv *bitVector, offset uintptr, t *rtype) {
	if t.ptrdata == 0 {
		return
	}

	switch Kind(t.kind & kindMask) {
	case Chan, Func, Map, Ptr, Slice, String, UnsafePointer:
		// 1 pointer at start of representation
		// 表示开始时有1个指针
		for bv.n < uint32(offset/uintptr(ptrSize)) {
			bv.append(0)
		}
		bv.append(1)

	case Interface:
		// 2 pointers
		// 两个指针
		for bv.n < uint32(offset/uintptr(ptrSize)) {
			bv.append(0)
		}
		bv.append(1)
		bv.append(1)

	case Array:
		// repeat inner type
		// 递归处理每个元素
		tt := (*arrayType)(unsafe.Pointer(t))
		for i := 0; i < int(tt.len); i++ {
			addTypeBits(bv, offset+uintptr(i)*tt.elem.size, tt.elem)
		}

	case Struct:
		// apply fields
		// 递归处理每个字段
		tt := (*structType)(unsafe.Pointer(t))
		for i := range tt.fields {
			f := &tt.fields[i]
			addTypeBits(bv, offset+f.offset(), f.typ)
		}
	}
}
```