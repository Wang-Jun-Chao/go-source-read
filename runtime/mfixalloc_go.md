```go

// Fixed-size object allocator. Returned memory is not zeroed.
//
// See malloc.go for overview.
// 固定大小的对象分配器。返回的内存未归零。
//
// 有关概述，请参见malloc.go。
package runtime

import "unsafe"

// FixAlloc is a simple free-list allocator for fixed size objects.
// Malloc uses a FixAlloc wrapped around sysAlloc to manage its
// mcache and mspan objects.
//
// Memory returned by fixalloc.alloc is zeroed by default, but the
// caller may take responsibility for zeroing allocations by setting
// the zero flag to false. This is only safe if the memory never
// contains heap pointers.
//
// The caller is responsible for locking around FixAlloc calls.
// Callers can keep state in the object but the first word is
// smashed by freeing and reallocating.
//
// Consider marking fixalloc'd types go:notinheap.
// FixAlloc是用于简单的固定大小对象的空闲列表分配器。 Malloc使用包裹在sysAlloc周围的FixAlloc来管理其mcache和mspan对象。
//
// 默认情况下，由fixalloc.alloc返回的内存被归零，但是调用者可以通过将zero标志设置为false来负责归零分配。
// 只有在内存中永远不包含堆指针的情况下，这才是安全的。
//
// 调用方负责锁定FixAlloc调用。
// 调用方可以将状态保持在对象中，但是通过释放和重新分配可以摧毁第一个字（word）：注：个人理解应该是fixalloc.size属性中的值。
//
// 考虑标记fixalloc的类型go:notinheap。
type fixalloc struct {
	size   uintptr
	first  func(arg, p unsafe.Pointer) // called first time p is returned // 首次分配内存时调用
	arg    unsafe.Pointer
	list   *mlink
	chunk  uintptr // use uintptr instead of unsafe.Pointer to avoid write barriers // 使用uintptr而不是unsafe.Pointer来避免写屏障
	nchunk uint32
	inuse  uintptr // in-use bytes now // 当前已经使用的字节数
	stat   *uint64
	zero   bool // zero allocations
}

// A generic linked list of blocks.  (Typically the block is bigger than sizeof(MLink).)
// Since assignments to mlink.next will result in a write barrier being performed
// this cannot be used by some of the internal GC structures. For example when
// the sweeper is placing an unmarked object on the free list it does not want the
// write barrier to be called since that could result in the object being reachable.
// 块的通用链接列表。 （通常，该块大于sizeof（MLink）。）
// 由于对mlink.next的赋值将导致执行写屏障，因此某些内部GC结构无法使用它。
// 例如，当清除程序将未标记的对象放在空闲列表上时，它不希望调用写屏障，因为这可能导致该对象可访问。
//
//go:notinheap
type mlink struct {
	next *mlink
}

// Initialize f to allocate objects of the given size,
// using the allocator to obtain chunks of memory.
// 使用分配器初始化f以分配给定大小的对象，以获取内存块。
func (f *fixalloc) init(size uintptr, first func(arg, p unsafe.Pointer), arg unsafe.Pointer, stat *uint64) {
	f.size = size
	f.first = first
	f.arg = arg
	f.list = nil
	f.chunk = 0
	f.nchunk = 0
	f.inuse = 0
	f.stat = stat
	f.zero = true
}

// 进行内存分配
func (f *fixalloc) alloc() unsafe.Pointer {
	if f.size == 0 {
		print("runtime: use of FixAlloc_Alloc before FixAlloc_Init\n")
		throw("runtime: internal error")
	}
    
    // list不为空就取list作为开始地址进行内存分配
	if f.list != nil {
		v := unsafe.Pointer(f.list)
		f.list = f.list.next
		f.inuse += f.size
		if f.zero {
			memclrNoHeapPointers(v, f.size) // 内存归零
		}
		return v
	}
	if uintptr(f.nchunk) < f.size {
		f.chunk = uintptr(persistentalloc(_FixAllocChunk, 0, f.stat))
		f.nchunk = _FixAllocChunk
	}

	v := unsafe.Pointer(f.chunk)
	if f.first != nil {
		f.first(f.arg, v)
	}
	f.chunk = f.chunk + f.size
	f.nchunk -= uint32(f.size)
	f.inuse += f.size
	return v
}

// 释放内存
func (f *fixalloc) free(p unsafe.Pointer) {
	f.inuse -= f.size
	v := (*mlink)(p)
	v.next = f.list
	f.list = v
}
```