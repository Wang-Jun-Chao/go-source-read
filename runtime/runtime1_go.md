
```go
package runtime

import (
	"runtime/internal/atomic"
	"runtime/internal/sys"
	"unsafe"
)

// Keep a cached value to make gotraceback fast,
// since we call it on every call to gentraceback.
// The cached value is a uint32 in which the low bits
// are the "crash" and "all" settings and the remaining
// bits are the traceback value (0 off, 1 on, 2 include system).
//
// 保留一个缓存的值以使gotraceback更快，因为我们在每次gentraceback调用时都调用它。
// 缓存的值是uint32，其中低位是“崩溃”和“所有”设置，其余位是回溯值（0关，1开，2包括系统）。
const (
	tracebackCrash = 1 << iota
	tracebackAll
	tracebackShift = iota
)

var traceback_cache uint32 = 2 << tracebackShift
var traceback_env uint32

// gotraceback returns the current traceback settings.
//
// If level is 0, suppress all tracebacks.
// If level is 1, show tracebacks, but exclude runtime frames.
// If level is 2, show tracebacks including runtime frames.
// If all is set, print all goroutine stacks. Otherwise, print just the current goroutine.
// If crash is set, crash (core dump, etc) after tracebacking.
//
// gotraceback返回当前的回溯设置。
//
//如果level为0，则禁止所有回溯。
//如果级别为1，则显示回溯，但不包括运行时帧。
//如果级别为2，则显示回溯，包括运行时框架。
//如果全部设置，则打印所有goroutine堆栈。否则，仅打印当前goroutine。
//如果设置了崩溃，则在回溯后崩溃（核心转储等）。
//
//go:nosplit
func gotraceback() (level int32, all, crash bool) {
	_g_ := getg()
	t := atomic.Load(&traceback_cache)
	crash = t&tracebackCrash != 0
	all = _g_.m.throwing > 0 || t&tracebackAll != 0
	if _g_.m.traceback != 0 {
		level = int32(_g_.m.traceback)
	} else {
		level = int32(t >> tracebackShift)
	}
	return
}

var (
	argc int32
	argv **byte
)

// nosplit for use in linux startup sysargs
// 在Linux启动sysargs中使用nosplit
//go:nosplit
func argv_index(argv **byte, i int32) *byte {
	return *(**byte)(add(unsafe.Pointer(argv), uintptr(i)*sys.PtrSize))
}

func args(c int32, v **byte) {
	argc = c
	argv = v
	sysargs(c, v)
}

func goargs() {
	if GOOS == "windows" {
		return
	}
	argslice = make([]string, argc)
	for i := int32(0); i < argc; i++ {
		argslice[i] = gostringnocopy(argv_index(argv, i))
	}
}

func goenvs_unix() {
	// TODO(austin): ppc64 in dynamic linking mode doesn't
	// guarantee env[] will immediately follow argv. Might cause
	// problems.
    // TODO(austin)：动态链接模式下的ppc64不能保证env[]将立即跟随argv。可能会引起问题。
	n := int32(0)
	for argv_index(argv, argc+1+n) != nil {
		n++
	}

	envs = make([]string, n)
	for i := int32(0); i < n; i++ {
		envs[i] = gostring(argv_index(argv, argc+1+i))
	}
}

func environ() []string {
	return envs
}

// TODO: These should be locals in testAtomic64, but we don't 8-byte
// align stack variables on 386.
// TODO：这些应该是testAtomic64中的局部变量，但是我们不对386上的8字节对齐堆栈变量。
var test_z64, test_x64 uint64

func testAtomic64() {
	test_z64 = 42
	test_x64 = 0
	if atomic.Cas64(&test_z64, test_x64, 1) {
		throw("cas64 failed")
	}
	if test_x64 != 0 {
		throw("cas64 failed")
	}
	test_x64 = 42
	if !atomic.Cas64(&test_z64, test_x64, 1) {
		throw("cas64 failed")
	}
	if test_x64 != 42 || test_z64 != 1 {
		throw("cas64 failed")
	}
	if atomic.Load64(&test_z64) != 1 {
		throw("load64 failed")
	}
	atomic.Store64(&test_z64, (1<<40)+1)
	if atomic.Load64(&test_z64) != (1<<40)+1 {
		throw("store64 failed")
	}
	if atomic.Xadd64(&test_z64, (1<<40)+1) != (2<<40)+2 {
		throw("xadd64 failed")
	}
	if atomic.Load64(&test_z64) != (2<<40)+2 {
		throw("xadd64 failed")
	}
	if atomic.Xchg64(&test_z64, (3<<40)+3) != (2<<40)+2 {
		throw("xchg64 failed")
	}
	if atomic.Load64(&test_z64) != (3<<40)+3 {
		throw("xchg64 failed")
	}
}

// 进行内存测试的
func check() {
	var (
		a     int8
		b     uint8
		c     int16
		d     uint16
		e     int32
		f     uint32
		g     int64
		h     uint64
		i, i1 float32
		j, j1 float64
		k     unsafe.Pointer
		l     *uint16
		m     [4]byte
	)
	type x1t struct {
		x uint8
	}
	type y1t struct {
		x1 x1t
		y  uint8
	}
	var x1 x1t
	var y1 y1t

	if unsafe.Sizeof(a) != 1 {
		throw("bad a")
	}
	if unsafe.Sizeof(b) != 1 {
		throw("bad b")
	}
	if unsafe.Sizeof(c) != 2 {
		throw("bad c")
	}
	if unsafe.Sizeof(d) != 2 {
		throw("bad d")
	}
	if unsafe.Sizeof(e) != 4 {
		throw("bad e")
	}
	if unsafe.Sizeof(f) != 4 {
		throw("bad f")
	}
	if unsafe.Sizeof(g) != 8 {
		throw("bad g")
	}
	if unsafe.Sizeof(h) != 8 {
		throw("bad h")
	}
	if unsafe.Sizeof(i) != 4 {
		throw("bad i")
	}
	if unsafe.Sizeof(j) != 8 {
		throw("bad j")
	}
	if unsafe.Sizeof(k) != sys.PtrSize {
		throw("bad k")
	}
	if unsafe.Sizeof(l) != sys.PtrSize {
		throw("bad l")
	}
	if unsafe.Sizeof(x1) != 1 {
		throw("bad unsafe.Sizeof x1")
	}
	if unsafe.Offsetof(y1.y) != 1 {
		throw("bad offsetof y1.y")
	}
	if unsafe.Sizeof(y1) != 2 {
		throw("bad unsafe.Sizeof y1")
	}

	if timediv(12345*1000000000+54321, 1000000000, &e) != 12345 || e != 54321 {
		throw("bad timediv")
	}

	var z uint32
	z = 1
	if !atomic.Cas(&z, 1, 2) {
		throw("cas1")
	}
	if z != 2 {
		throw("cas2")
	}

	z = 4
	if atomic.Cas(&z, 5, 6) {
		throw("cas3")
	}
	if z != 4 {
		throw("cas4")
	}

	z = 0xffffffff
	if !atomic.Cas(&z, 0xffffffff, 0xfffffffe) {
		throw("cas5")
	}
	if z != 0xfffffffe {
		throw("cas6")
	}

	m = [4]byte{1, 1, 1, 1}
	atomic.Or8(&m[1], 0xf0)
	if m[0] != 1 || m[1] != 0xf1 || m[2] != 1 || m[3] != 1 {
		throw("atomicor8")
	}

	m = [4]byte{0xff, 0xff, 0xff, 0xff}
	atomic.And8(&m[1], 0x1)
	if m[0] != 0xff || m[1] != 0x1 || m[2] != 0xff || m[3] != 0xff {
		throw("atomicand8")
	}

	*(*uint64)(unsafe.Pointer(&j)) = ^uint64(0)
	if j == j {
		throw("float64nan")
	}
	if !(j != j) {
		throw("float64nan1")
	}

	*(*uint64)(unsafe.Pointer(&j1)) = ^uint64(1)
	if j == j1 {
		throw("float64nan2")
	}
	if !(j != j1) {
		throw("float64nan3")
	}

	*(*uint32)(unsafe.Pointer(&i)) = ^uint32(0)
	if i == i {
		throw("float32nan")
	}
	if i == i {
		throw("float32nan1")
	}

	*(*uint32)(unsafe.Pointer(&i1)) = ^uint32(1)
	if i == i1 {
		throw("float32nan2")
	}
	if i == i1 {
		throw("float32nan3")
	}

	testAtomic64()

	if _FixedStack != round2(_FixedStack) {
		throw("FixedStack is not power-of-2")
	}

	if !checkASM() {
		throw("assembly checks failed")
	}
}

type dbgVar struct {
	name  string
	value *int32
}

// Holds variables parsed from GODEBUG env var,
// except for "memprofilerate" since there is an
// existing int var for that value, which may
// already have an initial value.
// //保存从GODEBUG env var解析的变量，但“ memprofilerate”除外，因为该值已经存在一个int var，它可能已经具有初始值。
var debug struct {
	allocfreetrace     int32
	cgocheck           int32
	clobberfree        int32
	efence             int32
	gccheckmark        int32
	gcpacertrace       int32
	gcshrinkstackoff   int32
	gcstoptheworld     int32
	gctrace            int32
	invalidptr         int32
	madvdontneed       int32 // for Linux; issue 28466
	sbrk               int32
	scavenge           int32
	scavtrace          int32
	scheddetail        int32
	schedtrace         int32
	tracebackancestors int32
	asyncpreemptoff    int32
}

var dbgvars = []dbgVar{
	{"allocfreetrace", &debug.allocfreetrace},
	{"clobberfree", &debug.clobberfree},
	{"cgocheck", &debug.cgocheck},
	{"efence", &debug.efence},
	{"gccheckmark", &debug.gccheckmark},
	{"gcpacertrace", &debug.gcpacertrace},
	{"gcshrinkstackoff", &debug.gcshrinkstackoff},
	{"gcstoptheworld", &debug.gcstoptheworld},
	{"gctrace", &debug.gctrace},
	{"invalidptr", &debug.invalidptr},
	{"madvdontneed", &debug.madvdontneed},
	{"sbrk", &debug.sbrk},
	{"scavenge", &debug.scavenge},
	{"scavtrace", &debug.scavtrace},
	{"scheddetail", &debug.scheddetail},
	{"schedtrace", &debug.schedtrace},
	{"tracebackancestors", &debug.tracebackancestors},
	{"asyncpreemptoff", &debug.asyncpreemptoff},
}

// 解析debug变量
func parsedebugvars() {
	// defaults // 默认值
	debug.cgocheck = 1
	debug.invalidptr = 1

	for p := gogetenv("GODEBUG"); p != ""; {
		field := ""
		i := index(p, ",")
		if i < 0 {
			field, p = p, ""
		} else {
			field, p = p[:i], p[i+1:]
		}
		i = index(field, "=")
		if i < 0 {
			continue
		}
		key, value := field[:i], field[i+1:]

		// Update MemProfileRate directly here since it
		// is int, not int32, and should only be updated
		// if specified in GODEBUG.
		// 在此处直接更新MemProfileRate，因为它是int而不是int32，并且只有在GODEBUG中指定时才应更新。
		if key == "memprofilerate" {
			if n, ok := atoi(value); ok {
				MemProfileRate = n
			}
		} else {
			for _, v := range dbgvars {
				if v.name == key {
					if n, ok := atoi32(value); ok {
						*v.value = n
					}
				}
			}
		}
	}

	setTraceback(gogetenv("GOTRACEBACK"))
	traceback_env = traceback_cache
}

// 设置回溯
//go:linkname setTraceback runtime/debug.SetTraceback
func setTraceback(level string) {
	var t uint32
	switch level {
	case "none":
		t = 0
	case "single", "":
		t = 1 << tracebackShift
	case "all":
		t = 1<<tracebackShift | tracebackAll
	case "system":
		t = 2<<tracebackShift | tracebackAll
	case "crash":
		t = 2<<tracebackShift | tracebackAll | tracebackCrash
	default:
		t = tracebackAll
		if n, ok := atoi(level); ok && n == int(uint32(n)) {
			t |= uint32(n) << tracebackShift
		}
	}
	// when C owns the process, simply exit'ing the process on fatal errors
	// and panics is surprising. Be louder and abort instead.
	// 当C拥有该进程时，由于致命错误和恐慌而简单地退出该进程是令人惊讶的。大声一点，然后放弃。
	if islibrary || isarchive {
		t |= tracebackCrash
	}

	t |= traceback_env

	atomic.Store(&traceback_cache, t)
}

// Poor mans 64-bit division.
// This is a very special function, do not use it if you are not sure what you are doing.
// int64 division is lowered into _divv() call on 386, which does not fit into nosplit functions.
// Handles overflow in a time-specific manner.
// This keeps us within no-split stack limits on 32-bit processors.
//
// 可怜的人64位除法。
// 这是一个非常特殊的函数，如果不确定自己在做什么，请不要使用它。
// int64除法被简化为386上的_divv（）调用，它不适合nosplit函数。以特定时间处理溢出。
// 这使我们保持限制，在32位处理器的不拆分堆栈。
//go:nosplit
func timediv(v int64, div int32, rem *int32) int32 {
	res := int32(0)
	for bit := 30; bit >= 0; bit-- {
		if v >= int64(div)<<uint(bit) {
			v = v - (int64(div) << uint(bit))
			// Before this for loop, res was 0, thus all these
			// power of 2 increments are now just bitsets.
			res |= 1 << uint(bit)
		}
	}
	if v >= int64(div) {
		if rem != nil {
			*rem = 0
		}
		return 0x7fffffff
	}
	if rem != nil {
		*rem = int32(v)
	}
	return res
}

// Helpers for Go. Must be NOSPLIT, must only call NOSPLIT functions, and must not block.
// Go的助手。必须为NOSPLIT，必须仅调用NOSPLIT函数，并且不得阻塞。

// 获取当前g的m
//go:nosplit
func acquirem() *m {
	_g_ := getg()
	_g_.m.locks++
	return _g_.m
}

// 释放当前g的m
//go:nosplit
func releasem(mp *m) {
	_g_ := getg()
	mp.locks--
	if mp.locks == 0 && _g_.preempt {
		// restore the preemption request in case we've cleared it in newstack
		// 恢复抢占请求，以防我们在新堆栈中将其清除
		_g_.stackguard0 = stackPreempt
	}
}

// 获取当前g的m上的mcache
//go:nosplit
func gomcache() *mcache {
	return getg().m.mcache
}

//go:linkname reflect_typelinks reflect.typelinks
func reflect_typelinks() ([]unsafe.Pointer, [][]int32) {
	modules := activeModules()
	sections := []unsafe.Pointer{unsafe.Pointer(modules[0].types)}
	ret := [][]int32{modules[0].typelinks}
	for _, md := range modules[1:] {
		sections = append(sections, unsafe.Pointer(md.types))
		ret = append(ret, md.typelinks)
	}
	return sections, ret
}

// reflect_resolveNameOff resolves a name offset from a base pointer.
// Reflection_resolveNameOff解析与基址指针的名称偏移。
//go:linkname reflect_resolveNameOff reflect.resolveNameOff
func reflect_resolveNameOff(ptrInModule unsafe.Pointer, off int32) unsafe.Pointer {
	return unsafe.Pointer(resolveNameOff(ptrInModule, nameOff(off)).bytes)
}

// reflect_resolveTypeOff resolves an *rtype offset from a base type.
// reflect_resolveTypeOff解析*rtype与基本类型的偏移量。
//go:linkname reflect_resolveTypeOff reflect.resolveTypeOff
func reflect_resolveTypeOff(rtype unsafe.Pointer, off int32) unsafe.Pointer {
	return unsafe.Pointer((*_type)(rtype).typeOff(typeOff(off)))
}

// reflect_resolveTextOff resolves a function pointer offset from a base type.
// reflect_resolveTextOff解析函数指针与基本类型的偏移量。
//go:linkname reflect_resolveTextOff reflect.resolveTextOff
func reflect_resolveTextOff(rtype unsafe.Pointer, off int32) unsafe.Pointer {
	return (*_type)(rtype).textOff(textOff(off))

}

// reflectlite_resolveNameOff resolves a name offset from a base pointer.
// reflectlite_resolveNameOff解析与基本指针的名称偏移。
//go:linkname reflectlite_resolveNameOff internal/reflectlite.resolveNameOff
func reflectlite_resolveNameOff(ptrInModule unsafe.Pointer, off int32) unsafe.Pointer {
	return unsafe.Pointer(resolveNameOff(ptrInModule, nameOff(off)).bytes)
}

// reflectlite_resolveTypeOff resolves an *rtype offset from a base type.
// reflectlite_resolveTypeOff解析*rtype与基本类型的偏移量。
//go:linkname reflectlite_resolveTypeOff internal/reflectlite.resolveTypeOff
func reflectlite_resolveTypeOff(rtype unsafe.Pointer, off int32) unsafe.Pointer {
	return unsafe.Pointer((*_type)(rtype).typeOff(typeOff(off)))
}

// reflect_addReflectOff adds a pointer to the reflection offset lookup map.
// reflect_addReflectOff将一个指针添加到反射偏移量查找map。
//go:linkname reflect_addReflectOff reflect.addReflectOff
func reflect_addReflectOff(ptr unsafe.Pointer) int32 {
	reflectOffsLock()
	if reflectOffs.m == nil {
		reflectOffs.m = make(map[int32]unsafe.Pointer)
		reflectOffs.minv = make(map[unsafe.Pointer]int32)
		reflectOffs.next = -1
	}
	id, found := reflectOffs.minv[ptr]
	if !found {
		id = reflectOffs.next
		reflectOffs.next-- // use negative offsets as IDs to aid debugging
		reflectOffs.m[id] = ptr
		reflectOffs.minv[ptr] = id
	}
	reflectOffsUnlock()
	return id
}
```