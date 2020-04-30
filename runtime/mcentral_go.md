```go


// Central free lists.
//
// See malloc.go for an overview.
//
// The mcentral doesn't actually contain the list of free objects; the mspan does.
// Each mcentral is two lists of mspans: those with free objects (c->nonempty)
// and those that are completely allocated (c->empty).
// 中央空闲列表。
//
// 有关概述，请参见malloc.go。
//
// mcentral实际上并不包含空闲对象的列表； mspan可以。
// 每个mcentral都有两个mspan列表：带有空闲对象的列表（c-> nonempty）和已完全分配的对象（c-> empty）。

package runtime

import "runtime/internal/atomic"

// Central list of free objects of a given size.
// 给定大小的空闲对象的中央列表。
//
//go:notinheap
type mcentral struct {
	lock      mutex
	spanclass spanClass
	nonempty  mSpanList // list of spans with a free object, ie a nonempty free list // 具有空闲对象的跨度列表，即非空的空闲列表
	empty     mSpanList // list of spans with no free objects (or cached in an mcache) // 没有空闲对象的跨度列表（或缓存在mcache中）

	// nmalloc is the cumulative count of objects allocated from
	// this mcentral, assuming all spans in mcaches are
	// fully-allocated. Written atomically, read under STW.
    // nmalloc是从此中心分配的对象的累积计数，假定mcache中的所有跨度都已完全分配。写是原子操作的，在全局停机（STW）下读。
	nmalloc uint64
}

// Initialize a single central free list.
// 初始化单个中央空闲列表。
func (c *mcentral) init(spc spanClass) {
	c.spanclass = spc
	c.nonempty.init()
	c.empty.init()
}

// Allocate a span to use in an mcache.
// 在mcache中分配跨度以使用。
func (c *mcentral) cacheSpan() *mspan {
	// Deduct credit for this span allocation and sweep if necessary.
    // 扣除此跨度分配的信用，并在必要时进行扫描。
	spanBytes := uintptr(class_to_allocnpages[c.spanclass.sizeclass()]) * _PageSize
	deductSweepCredit(spanBytes, 0)

	lock(&c.lock)
	traceDone := false
	if trace.enabled {
		traceGCSweepStart()
	}
	sg := mheap_.sweepgen
retry:
	var s *mspan
	for s = c.nonempty.first; s != nil; s = s.next {
		if s.sweepgen == sg-2 && atomic.Cas(&s.sweepgen, sg-2, sg-1) {
			c.nonempty.remove(s)
			c.empty.insertBack(s)
			unlock(&c.lock)
			s.sweep(true)
			goto havespan
		}
		if s.sweepgen == sg-1 {
			// the span is being swept by background sweeper, skip
            // 跨度正被后台扫描器扫描，跳过
			continue
		}
		// we have a nonempty span that does not require sweeping, allocate from it
        // 我们有一个非空跨度，不需要扫除，从中分配
		c.nonempty.remove(s)
		c.empty.insertBack(s)
		unlock(&c.lock)
		goto havespan
	}

	for s = c.empty.first; s != nil; s = s.next {
		if s.sweepgen == sg-2 && atomic.Cas(&s.sweepgen, sg-2, sg-1) {
			// we have an empty span that requires sweeping,
			// sweep it and see if we can free some space in it
            // 我们有一个空的跨度，需要扫描，扫描看看是否可以释放其中的一些空间
			c.empty.remove(s)
			// swept spans are at the end of the list
            // 扫描的跨度在列表的末尾
			c.empty.insertBack(s)
			unlock(&c.lock)
			s.sweep(true)
			freeIndex := s.nextFreeIndex()
			if freeIndex != s.nelems {
				s.freeindex = freeIndex
				goto havespan
			}
			lock(&c.lock)
			// the span is still empty after sweep
			// it is already in the empty list, so just retry
            // 扫描后跨度仍然为空，它已经在空列表中，因此只需重试
			goto retry
		}
		if s.sweepgen == sg-1 {
			// the span is being swept by background sweeper, skip
            // 跨度正被后台扫描器扫描，跳过
			continue
		}
		// already swept empty span,
		// all subsequent ones must also be either swept or in process of sweeping
        // 已经扫过的空跨度，所有随后的子序列，必须是扫描过或在扫描过程中
		break
	}
	if trace.enabled {
		traceGCSweepDone()
		traceDone = true
	}
	unlock(&c.lock)

	// Replenish central list if empty.
    // 如果为空，请补充中央列表。
	s = c.grow()
	if s == nil {
		return nil
	}
	lock(&c.lock)
	c.empty.insertBack(s)
	unlock(&c.lock)

	// At this point s is a non-empty span, queued at the end of the empty list,
	// c is unlocked.
    // 在这个时候，s是一个非空跨度，在空列表的末尾排队，c被解锁。
havespan:
	if trace.enabled && !traceDone {
		traceGCSweepDone()
	}
	n := int(s.nelems) - int(s.allocCount)
	if n == 0 || s.freeindex == s.nelems || uintptr(s.allocCount) == s.nelems {
		throw("span has no free objects")
	}
	// Assume all objects from this span will be allocated in the
	// mcache. If it gets uncached, we'll adjust this.
    // 假设此范围内的所有对象都将分配在mcache中。如果未缓存，我们将对其进行调整。
	atomic.Xadd64(&c.nmalloc, int64(n))
	usedBytes := uintptr(s.allocCount) * s.elemsize
	atomic.Xadd64(&memstats.heap_live, int64(spanBytes)-int64(usedBytes))
	if trace.enabled {
		// heap_live changed.
		traceHeapAlloc()
	}
	if gcBlackenEnabled != 0 {
		// heap_live changed.
		gcController.revise()
	}
	freeByteBase := s.freeindex &^ (64 - 1)
	whichByte := freeByteBase / 8
	// Init alloc bits cache.
    // 初始化分配位缓存。
	s.refillAllocCache(whichByte)

	// Adjust the allocCache so that s.freeindex corresponds to the low bit in
	// s.allocCache.
    // 调整allocCache，使s.freeindex对应于s.allocCache的低位。
	s.allocCache >>= s.freeindex % 64

	return s
}

// Return span from an mcache.
// 从mcache返回跨度。
func (c *mcentral) uncacheSpan(s *mspan) {
	if s.allocCount == 0 {
		throw("uncaching span but s.allocCount == 0")
	}

	sg := mheap_.sweepgen
	stale := s.sweepgen == sg+1
	if stale {
		// Span was cached before sweep began. It's our
		// responsibility to sweep it.
		//
		// Set sweepgen to indicate it's not cached but needs
		// sweeping and can't be allocated from. sweep will
		// set s.sweepgen to indicate s is swept.
        // Span在开始扫描之前已被缓存。扫描它是我们的责任。
        //
        // 设置sweepgen以指示它尚未缓存，但需要清除并且不能从中分配。扫描将设置s.sweepgen以指示已被扫描。
		atomic.Store(&s.sweepgen, sg-1)
	} else {
		// Indicate that s is no longer cached.
        // 表示s不再被缓存。
		atomic.Store(&s.sweepgen, sg)
	}

	n := int(s.nelems) - int(s.allocCount)
	if n > 0 {
		// cacheSpan updated alloc assuming all objects on s
		// were going to be allocated. Adjust for any that
		// weren't. We must do this before potentially
		// sweeping the span.
        // cacheSpan更新了alloc，假设s上的所有对象都将被分配。
		// 调整任何不是的。我们必须在可能扫描跨度之前执行此操作。
		atomic.Xadd64(&c.nmalloc, -int64(n))

		lock(&c.lock)
		c.empty.remove(s)
		c.nonempty.insert(s)
		if !stale {
			// mCentral_CacheSpan conservatively counted
			// unallocated slots in heap_live. Undo this.
			//
			// If this span was cached before sweep, then
			// heap_live was totally recomputed since
			// caching this span, so we don't do this for
			// stale spans.
            // mCentral_CacheSpan保守地计算了heap_live中的未分配插槽。取消这个。
            //
            //如果此跨度是在清除之前缓存的，则自从缓存此跨度以来，heap_live已被完全重新计算，因此对于陈旧的跨度，我们不这样做。
			atomic.Xadd64(&memstats.heap_live, -int64(n)*int64(s.elemsize))
		}
		unlock(&c.lock)
	}

	if stale {
		// Now that s is in the right mcentral list, we can
		// sweep it.
        // 现在s在正确的mcentral列表中，我们可以对其进行扫描。
		s.sweep(false)
	}
}

// freeSpan updates c and s after sweeping s.
// It sets s's sweepgen to the latest generation,
// and, based on the number of free objects in s,
// moves s to the appropriate list of c or returns it
// to the heap.
// freeSpan reports whether s was returned to the heap.
// If preserve=true, it does not move s (the caller
// must take care of it).
// freeSpan在清除s之后更新c和s。
// 将s的swapgen设置为最新一代，然后根据s中空闲对象的数量将s移至c的适当列表或将其返回到堆。
// freeSpan报告s是否返回到堆。
// 如果preserve = true，则不会移动s（调用者必须小心处理它）。
func (c *mcentral) freeSpan(s *mspan, preserve bool, wasempty bool) bool {
	if sg := mheap_.sweepgen; s.sweepgen == sg+1 || s.sweepgen == sg+3 {
		throw("freeSpan given cached span")
	}
	s.needzero = 1

	if preserve {
		// preserve is set only when called from (un)cacheSpan above,
		// the span must be in the empty list.
        // 仅当从上面的(un)cacheSpan调用时设置一次save，该跨度必须在空列表中。
		if !s.inList() {
			throw("can't preserve unlinked span")
		}
		atomic.Store(&s.sweepgen, mheap_.sweepgen)
		return false
	}

	lock(&c.lock)

	// Move to nonempty if necessary.
    // 如有必要，请移至非空。
	if wasempty {
		c.empty.remove(s)
		c.nonempty.insert(s)
	}

	// delay updating sweepgen until here. This is the signal that
	// the span may be used in an mcache, so it must come after the
	// linked list operations above (actually, just after the
	// lock of c above.)
    // 延迟更新scangen直到此处。这是表明跨度可能在mcache中使用的信号，
    // 因此跨度必须在上面的链表操作之后（实际上，恰好在上面的c锁之后）。
	atomic.Store(&s.sweepgen, mheap_.sweepgen)

	if s.allocCount != 0 {
		unlock(&c.lock)
		return false
	}

	c.nonempty.remove(s)
	unlock(&c.lock)
	mheap_.freeSpan(s)
	return true
}

// grow allocates a new empty span from the heap and initializes it for c's size class.
// grow从堆中分配一个新的空跨度，并将其初始化为c的size类。
func (c *mcentral) grow() *mspan {
	npages := uintptr(class_to_allocnpages[c.spanclass.sizeclass()])
	size := uintptr(class_to_size[c.spanclass.sizeclass()])

	s := mheap_.alloc(npages, c.spanclass, true)
	if s == nil {
		return nil
	}

	// Use division by multiplication and shifts to quickly compute:
	// n := (npages << _PageShift) / size
    // 使用乘除法和移位来快速计算：
    // n：=(npages << _PageShift) / size
	n := (npages << _PageShift) >> s.divShift * uintptr(s.divMul) >> s.divShift2
	s.limit = s.base() + size*n
	heapBitsForAddr(s.base()).initSpan(s)
	return s
}

```