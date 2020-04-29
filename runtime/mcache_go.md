
```go

package runtime

import (
	"runtime/internal/atomic"
	"unsafe"
)

// Per-thread (in Go, per-P) cache for small objects.
// No locking needed because it is per-thread (per-P).
//
// mcaches are allocated from non-GC'd memory, so any heap pointers
// must be specially handled.
//
// 小对象的按线程（在Go，每个P）缓存。
// 不需要锁定，因为它是每个线程都有的（每个P）。
//
// mcache是​​从非GC的内存中分配的，因此任何堆指针都必须经过特殊处理。
//go:notinheap
type mcache struct {
	// The following members are accessed on every malloc,
	// so they are grouped here for better caching.
    // 在每个malloc上都访问以下成员，因此将它们分组在此处可以更好地进行缓存。
	next_sample uintptr // trigger heap sample after allocating this many bytes // 分配这么多字节后触发堆采样
	local_scan  uintptr // bytes of scannable heap allocated // 分配的可扫描堆字节数

	// Allocator cache for tiny objects w/o pointers.
	// See "Tiny allocator" comment in malloc.go.
    // w/o指针的微小对象的分配器缓存。
    // 请参阅malloc.go中的“Tiny allocator”注释。

	// tiny points to the beginning of the current tiny block, or
	// nil if there is no current tiny block.
	//
	// tiny is a heap pointer. Since mcache is in non-GC'd memory,
	// we handle it by clearing it in releaseAll during mark
	// termination.
    // tiny指向当前小块的开头，如果没有当前小块，则为nil。
    // 
    // tiny是一个堆指针。由于mcache位于非GC的内存中，因此我们通过在标记终止期间在releaseAll中将其清除来对其进行处理。
	tiny             uintptr
	tinyoffset       uintptr
	local_tinyallocs uintptr // number of tiny allocs not counted in other stats // 未在其他统计信息中计算的微小分配数量

	// The rest is not accessed on every malloc.
    // 其余的不是在每个malloc上访问的。
	alloc [numSpanClasses]*mspan // spans to allocate from, indexed by spanClass // 要分配的范围，由spanClass索引

	stackcache [_NumStackOrders]stackfreelist

	// Local allocator stats, flushed during GC.
    // 本地分配器统计信息，在GC期间刷新。
	local_largefree  uintptr                  // bytes freed for large objects (>maxsmallsize) // 大对象释放的字节数（> maxsmallsize）
	local_nlargefree uintptr                  // number of frees for large objects (>maxsmallsize) // 大对象的释放次数（> maxsmallsize）
	local_nsmallfree [_NumSizeClasses]uintptr // number of frees for small objects (<=maxsmallsize) // 小对象的释放次数（<= maxsmallsize）

	// flushGen indicates the sweepgen during which this mcache
	// was last flushed. If flushGen != mheap_.sweepgen, the spans
	// in this mcache are stale and need to the flushed so they
	// can be swept. This is done in acquirep.
    // flushGen指示上一次刷新此mcache的sweepgen。如果flushGen != mheap_.sweepgen，
    // 则此mcache中的跨度是旧的，需要刷新，以便可以对其进行清除。这是在acquirep中完成的。
	flushGen uint32
}

// A gclink is a node in a linked list of blocks, like mlink,
// but it is opaque to the garbage collector.
// The GC does not trace the pointers during collection,
// and the compiler does not emit write barriers for assignments
// of gclinkptr values. Code should store references to gclinks
// as gclinkptr, not as *gclink.
// gclink是块的链接列表中的一个节点，例如mlink，但是对于垃圾收集器而言是不透明的。
// GC不会在收集期间跟踪指针，并且编译器不会为分配gclinkptr值发出写障碍。
// 代码应将对gclinks的引用存储为gclinkptr，而不是*gclink。
type gclink struct {
	next gclinkptr
}

// A gclinkptr is a pointer to a gclink, but it is opaque
// to the garbage collector.
// gclinkptr是指向gclink的指针，但对垃圾收集器不透明。
type gclinkptr uintptr

// ptr returns the *gclink form of p.
// The result should be used for accessing fields, not stored
// in other data structures.
func (p gclinkptr) ptr() *gclink {
	return (*gclink)(unsafe.Pointer(p))
}

type stackfreelist struct {
	list gclinkptr // linked list of free stacks // 空闲栈的链接列表
	size uintptr   // total size of stacks in list // 列表中栈的总大小
}

// dummy mspan that contains no free objects.
// 不包含空闲对象的虚拟mspan。
var emptymspan mspan

// 分配mchache
func allocmcache() *mcache {
	var c *mcache
	systemstack(func() {
		lock(&mheap_.lock)
		c = (*mcache)(mheap_.cachealloc.alloc())
		c.flushGen = mheap_.sweepgen
		unlock(&mheap_.lock)
	})
	for i := range c.alloc {
		c.alloc[i] = &emptymspan
	}
	c.next_sample = nextSample()
	return c
}

// 释放mcache
func freemcache(c *mcache) {
	systemstack(func() {
		c.releaseAll()
		stackcache_clear(c)

		// NOTE(rsc,rlh): If gcworkbuffree comes back, we need to coordinate
		// with the stealing of gcworkbufs during garbage collection to avoid
		// a race where the workbuf is double-freed.
		// gcworkbuffree(c.gcworkbuf)
        // 注意（rsc，rlh）：如果gcworkbuffree回来了，我们需要在垃圾回收期间与gcworkbufs的窃取进行协调，以避免竞争以防workbuff被双重释放。
        // gcworkbuffree（c.gcworkbuf）

		lock(&mheap_.lock)
		purgecachedstats(c)
		mheap_.cachealloc.free(unsafe.Pointer(c))
		unlock(&mheap_.lock)
	})
}

// refill acquires a new span of span class spc for c. This span will
// have at least one free object. The current span in c must be full.
//
// Must run in a non-preemptible context since otherwise the owner of
// c could change.
// refill 为c获取一个新的span类spc。此跨度将至少有一个空闲对象。 c中的当前范围必须已满。
//
// 必须在不可抢占的上下文中运行，因为否则c的所有者可能会更改。
func (c *mcache) refill(spc spanClass) {
	// Return the current cached span to the central lists.
    // 将当前缓存的跨度返回到中央列表。
	s := c.alloc[spc]

	if uintptr(s.allocCount) != s.nelems {
		throw("refill of span with free space remaining")
	}
	if s != &emptymspan {
		// Mark this span as no longer cached.
        // 将此跨度标记为不再缓存。
		if s.sweepgen != mheap_.sweepgen+3 {
			throw("bad sweepgen in refill")
		}
		atomic.Store(&s.sweepgen, mheap_.sweepgen)
	}

	// Get a new cached span from the central lists.
    // 从中央列表中获取一个新的缓存跨度。
	s = mheap_.central[spc].mcentral.cacheSpan()
	if s == nil {
		throw("out of memory")
	}

	if uintptr(s.allocCount) == s.nelems {
		throw("span has no free space")
	}

	// Indicate that this span is cached and prevent asynchronous
	// sweeping in the next sweep phase.
    // 指示此跨度已缓存，并防止在下一个扫描阶段进行异步扫描。
	s.sweepgen = mheap_.sweepgen + 3

	c.alloc[spc] = s
}

// 释放mcache的所有信息
func (c *mcache) releaseAll() {
	for i := range c.alloc {
		s := c.alloc[i]
		if s != &emptymspan {
			mheap_.central[i].mcentral.uncacheSpan(s)
			c.alloc[i] = &emptymspan
		}
	}
	// Clear tinyalloc pool.
	c.tiny = 0
	c.tinyoffset = 0
}

// prepareForSweep flushes c if the system has entered a new sweep phase
// since c was populated. This must happen between the sweep phase
// starting and the first allocation from c.
// 如果自填充c以来系统进入新的扫描阶段，则prepareForSweep将刷新c。
// 这必须在扫描阶段开始和从c开始的第一次分配之间发生。
func (c *mcache) prepareForSweep() {
	// Alternatively, instead of making sure we do this on every P
	// between starting the world and allocating on that P, we
	// could leave allocate-black on, allow allocation to continue
	// as usual, use a ragged barrier at the beginning of sweep to
	// ensure all cached spans are swept, and then disable
	// allocate-black. However, with this approach it's difficult
	// to avoid spilling mark bits into the *next* GC cycle.
    // 或者，可以确保在启动世界和对该P进行分配之间的每个P上都执行此操作，而不是让其处于启用状态，
	// 允许分配继续照常进行，在扫描开始时使用粗糙的屏障以确保所有已缓存的跨度被扫描过，然后禁用allocate-black。
	// 但是，使用这种方法很难避免将标记位溢出到* next * GC循环中。
	sg := mheap_.sweepgen
	if c.flushGen == sg {
		return
	} else if c.flushGen != sg-2 {
		println("bad flushGen", c.flushGen, "in prepareForSweep; sweepgen", sg)
		throw("bad flushGen")
	}
	c.releaseAll()
	stackcache_clear(c)
	atomic.Store(&c.flushGen, mheap_.sweepgen) // Synchronizes with gcStart // 与gcStart同步
}

```