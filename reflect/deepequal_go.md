```go

// Deep equality test via reflection

package reflect

import "unsafe"

// 在deepValueEqual期间，必须跟踪正在进行的检查。 比较算法假定重新遇到它们时，所有进行中的检查都是true。访问的比较结果存储在按visit索引的map中。
// During deepValueEqual, must keep track of checks that are
// in progress. The comparison algorithm assumes that all
// checks in progress are true when it reencounters them.
// Visited comparisons are stored in a map indexed by visit.
type visit struct {
	a1  unsafe.Pointer
	a2  unsafe.Pointer
	typ Type
}

// 使用反射类型测试深度相等。 map参数跟踪已经看到(已经比较过并且为true)的比较，这允许递归类型上的短路。
// @param v1 待比较的值
// @param v2 待比较的值
// @param visited 保存已经访问过的值
// @param 递归深度
// Tests for deep equality using reflected types. The map argument tracks
// comparisons that have already been seen, which allows short circuiting on
// recursive types.
func deepValueEqual(v1, v2 Value, visited map[visit]bool, depth int) bool {
	if !v1.IsValid() || !v2.IsValid() { // 同为valid类型时才会相等
		return v1.IsValid() == v2.IsValid()
	}
	if v1.Type() != v2.Type() { // 类型不同
		return false
	}

    // 递归深度大于10了就抛出异常
	// if depth > 10 { panic("deepValueEqual") }	// for debugging // 调试使用
    // 我们希望避免在访问的map中放置过多的内容。对于可能遇到的任何可能的循环引用，hard（v1，v2）需要为循环中的至少一种类型返回true，并且获取Value的内部指针是安全有效的。
	// We want to avoid putting more in the visited map than we need to.
	// For any possible reference cycle that might be encountered,
	// hard(v1, v2) needs to return true for at least one of the types in the cycle,
	// and it's safe and valid to get Value's internal pointer.
	hard := func(v1, v2 Value) bool { // 函数表示v1是映射，切片，指针，接口类型的非空值，v2是任意类型的非空值
		switch v1.Kind() {
		case Map, Slice, Ptr, Interface: // 映射，切片，指针，接口类型
		    // Nil指针不能是循环的。 避免将它们放在访问过的map中。
			// Nil pointers cannot be cyclic. Avoid putting them in the visited map.
			return !v1.IsNil() && !v2.IsNil()
		}
		return false
	}

	if hard(v1, v2) {
		addr1 := v1.ptr
		addr2 := v2.ptr
		if uintptr(addr1) > uintptr(addr2) {
		    // 顺序化以减少访问的条目数。 假定不移动垃圾收集器。
			// Canonicalize order to reduce number of entries in visited.
			// Assumes non-moving garbage collector.
			addr1, addr2 = addr2, addr1
		}

        // 如果引用已经处理过，则短路。
		// Short circuit if references are already seen.
		typ := v1.Type()
		v := visit{addr1, addr2, typ}
		if visited[v] { // 之前已经处理过，不需要再处理了
			return true
		}

         // 标记已经处理过
		// Remember for later.
		visited[v] = true
	}

    // 对具体类型进行比较
	switch v1.Kind() {
	case Array: // 数组类型，比较每一个元素的值 TODO 不处理两个元素的长度？
		for i := 0; i < v1.Len(); i++ {
			if !deepValueEqual(v1.Index(i), v2.Index(i), visited, depth+1) {
				return false
			}
		}
		return true
	case Slice: // 切片类型
		if v1.IsNil() != v2.IsNil() { // 有一个不为nil
			return false
		}
		if v1.Len() != v2.Len() { // 长度不相等
			return false
		}
		if v1.Pointer() == v2.Pointer() { // 同一个值
			return true
		}
		for i := 0; i < v1.Len(); i++ { // 比较每一个元素的值
			if !deepValueEqual(v1.Index(i), v2.Index(i), visited, depth+1) {
				return false
			}
		}
		return true
	case Interface: // 接口类型
		if v1.IsNil() || v2.IsNil() { // 有一个值为nil
			return v1.IsNil() == v2.IsNil()
		}
		// 都不为nil，深度比较
		return deepValueEqual(v1.Elem(), v2.Elem(), visited, depth+1)
	case Ptr: // 指针类型
		if v1.Pointer() == v2.Pointer() { // 同一个值
			return true
		}
		// 比较指针所指向的元素值
		return deepValueEqual(v1.Elem(), v2.Elem(), visited, depth+1)
	case Struct: // 结构体类型
		for i, n := 0, v1.NumField(); i < n; i++ { // 比较每一个属性的值 TODO 不处理元素属性个数不相等的情况？
			if !deepValueEqual(v1.Field(i), v2.Field(i), visited, depth+1) {
				return false
			}
		}
		return true
	case Map: // 映射
		if v1.IsNil() != v2.IsNil() { // 有值为nil
			return false
		}
		if v1.Len() != v2.Len() { // 元素个数不相等
			return false
		}
		if v1.Pointer() == v2.Pointer() { // 同一个值
			return true
		}
		for _, k := range v1.MapKeys() { // 比较每一个元素的值
			val1 := v1.MapIndex(k)
			val2 := v2.MapIndex(k)
			if !val1.IsValid() || !val2.IsValid() || !deepValueEqual(val1, val2, visited, depth+1) {
				return false
			}
		}
		return true
	case Func: // 函数
		if v1.IsNil() && v2.IsNil() { // 只有当两个函数为nil时，比较才返回true，否则返回false
			return true
		}
		// Can't do better than this:
		return false
	default: // 其他情况
	    // 正常比较就足够了
		// Normal equality suffices
		return valueInterface(v1, false) == valueInterface(v2, false)
	}
}

// DeepEqual报告x和y是否``深度相等''，定义如下。
// 如果满足以下情况之一，则两个相同类型的值将完全相等。
// 不同类型的值永远不会完全相等。
//
// 当数组值对应的元素深度相等时，它们的值深度相等。
//
// 如果导出和未导出的对应字段深度相等，则结构值深度相等。
//
// 如果两者均为nil，则func值非常相等；否则，它们就不会完全平等。
//
// 如果接口值具有完全相等的具体值，则它们是高度相等的。
//
// 当满足以下所有条件时，映射值就深度相等：
// 它们均为nil或均为非nil，它们具有相同的长度，并且它们是相同的映射对象或它们的对应键（使用Go equals匹配）映射为深度相等的值。
// 如果指针值使用Go==运算符相等，或者它们指向深度相等的值，则它们的深度相等。
//
// 当满足以下所有条件时，slice值深度相等：它们都为nil或都不为nil，它们的长度相同，并且它们指向同一基础数组的相同初始条目
//（即&x[0]==&y[0]）或它们对应的元素（最大长度）深度相等。
// 请注意，非零空片和零片（例如[]byte{}和[]byt(nil)）不深度相等。
//
// 如果其他值-数字，布尔值，字符串和通道-如果使用Go的==运算符相等，则它们深度相等。
//
// 通常，DeepEqual是Go的==运算符的递归松弛。
// 但是，如果没有一些不一致，就不可能实现这个想法。
// 特别是，值可能与自身不相等，可能是因为它是func类型（通常无法比较），或者是因为它是浮点NaN值（在浮点比较中不等于其自身），或因为它是包含此类值的数组，结构或接口。
//
// 另一方面，即使指针值指向或包含此类有问题的值，它们也始终等于它们自己，因为它们使用Go的==运算符比较相等，并且这是一个足以完全相等的条件，而与内容无关 。
// 已定义DeepEqual，以便对切片和映射应用相同的快捷方式：如果x和y是相同的切片或相同的映射，则无论内容如何，它们的深度都相等。
// 当DeepEqual遍历数据值时，可能会发现一个循环。 DeepEqual在第二次及以后比较两个之前已比较过的指针值时，会将这些值视为相等，而不是检查它们所指向的值。 这样可以确保DeepEqual终止。
// Two values of identical type are deeply equal if one of the following cases applies.
// Values of distinct types are never deeply equal.
//
// Array values are deeply equal when their corresponding elements are deeply equal.
//
// Struct values are deeply equal if their corresponding fields, both exported and unexported, are deeply equal.
//
// Func values are deeply equal if both are nil; otherwise they are not deeply equal.
//
// Interface values are deeply equal if they hold deeply equal concrete values.
//
// DeepEqual reports whether x and y are ``deeply equal,'' defined as follows.
// Two values of identical type are deeply equal if one of the following cases applies.
// Values of distinct types are never deeply equal.
//
// Array values are deeply equal when their corresponding elements are deeply equal.
//
// Struct values are deeply equal if their corresponding fields,
// both exported and unexported, are deeply equal.
//
// Func values are deeply equal if both are nil; otherwise they are not deeply equal.
//
// Interface values are deeply equal if they hold deeply equal concrete values.
//
// Map values are deeply equal when all of the following are true:
// they are both nil or both non-nil, they have the same length,
// and either they are the same map object or their corresponding keys
// (matched using Go equality) map to deeply equal values.
//
// Pointer values are deeply equal if they are equal using Go's == operator
// or if they point to deeply equal values.
//
// Slice values are deeply equal when all of the following are true:
// they are both nil or both non-nil, they have the same length,
// and either they point to the same initial entry of the same underlying array
// (that is, &x[0] == &y[0]) or their corresponding elements (up to length) are deeply equal.
// Note that a non-nil empty slice and a nil slice (for example, []byte{} and []byte(nil))
// are not deeply equal.
//
// Other values - numbers, bools, strings, and channels - are deeply equal
// if they are equal using Go's == operator.
//
// In general DeepEqual is a recursive relaxation of Go's == operator.
// However, this idea is impossible to implement without some inconsistency.
// Specifically, it is possible for a value to be unequal to itself,
// either because it is of func type (uncomparable in general)
// or because it is a floating-point NaN value (not equal to itself in floating-point comparison),
// or because it is an array, struct, or interface containing
// such a value.
// On the other hand, pointer values are always equal to themselves,
// even if they point at or contain such problematic values,
// because they compare equal using Go's == operator, and that
// is a sufficient condition to be deeply equal, regardless of content.
// DeepEqual has been defined so that the same short-cut applies
// to slices and maps: if x and y are the same slice or the same map,
// they are deeply equal regardless of content.
//
// As DeepEqual traverses the data values it may find a cycle. The
// second and subsequent times that DeepEqual compares two pointer
// values that have been compared before, it treats the values as
// equal rather than examining the values to which they point.
// This ensures that DeepEqual terminates.
func DeepEqual(x, y interface{}) bool {
	if x == nil || y == nil { // 有一个值为nil
		return x == y
	}
	v1 := ValueOf(x) // ValueOf返回一个新的Value，初始化为存储在接口i中的具体值。 ValueOf（nil）返回零值。
	v2 := ValueOf(y)
	if v1.Type() != v2.Type() { // 类型不同
		return false
	}
	// 深度比较
	return deepValueEqual(v1, v2, make(map[visit]bool), 0)
}
```