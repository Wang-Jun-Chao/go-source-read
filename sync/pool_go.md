```go

package sync

import (
	"internal/race"
	"runtime"
	"sync/atomic"
	"unsafe"
)

// A Pool is a set of temporary objects that may be individually saved and
// retrieved.
//
// Any item stored in the Pool may be removed automatically at any time without
// notification. If the Pool holds the only reference when this happens, the
// item might be deallocated.
//
// A Pool is safe for use by multiple goroutines simultaneously.
//
// Pool's purpose is to cache allocated but unused items for later reuse,
// relieving pressure on the garbage collector. That is, it makes it easy to
// build efficient, thread-safe free lists. However, it is not suitable for all
// free lists.
//
// An appropriate use of a Pool is to manage a group of temporary items
// silently shared among and potentially reused by concurrent independent
// clients of a package. Pool provides a way to amortize allocation overhead
// across many clients.
//
// An example of good use of a Pool is in the fmt package, which maintains a
// dynamically-sized store of temporary output buffers. The store scales under
// load (when many goroutines are actively printing) and shrinks when
// quiescent.
//
// On the other hand, a free list maintained as part of a short-lived object is
// not a suitable use for a Pool, since the overhead does not amortize well in
// that scenario. It is more efficient to have such objects implement their own
// free list.
//
// A Pool must not be copied after first use.
//
// 池是一组可以单独保存和检索的临时对象。
//
// 池中存储的任何项目都可以随时自动删除，恕不另行通知。如果发生这种情况时，池中只有唯一的引用，则该项目可能会被释放。
//
// 一个Pool可以安全地同时被多个goroutine使用。
//
// Pool的目的是缓存已分配但未使用的项目，以供以后重用，从而减轻了垃圾回收器的压力。
// 也就是说，它使构建高效，线程安全的空闲列表变得容易。但是，它并不适合所有空闲列表。
//
// 池的适当用法是管理一组临时项目，这些临时项目在程序包的并发独立客户端之间静默共享并有可能被重用。
// 池提供了一种摊销许多客户端上的分配开销的方法。
//
// fmt包中有一个很好使用Pool的示例，该包维护着动态大小的临时输出缓冲区存储。
// 存储在负载下会缩放（当许多goroutine正在主动打印时），并且在静止时会收缩。
//
// 另一方面，作为短期对象的一部分维护的空闲列表不适用于Pool，因为在这种情况下，开销无法很好地摊销。
// 让这样的对象实现自己的空闲列表会更有效。
//
// 首次使用后不得复制池。
type Pool struct {
    // noCopy可以嵌入到第一次使用后不得复制的结构中。
	noCopy noCopy // noCopy may be embedded into structs which must not be copied after the first use.

	local     unsafe.Pointer // local fixed-size per-P pool, actual type is [P]poolLocal // 本地固定大小的每个P池，实际类型为[P] poolLocal
	localSize uintptr        // size of the local array // 本地数组的大小

	victim     unsafe.Pointer // local from previous cycle // 上一个周期的local
	victimSize uintptr        // size of victims array // victim的大小

	// New optionally specifies a function to generate
	// a value when Get would otherwise return nil.
	// It may not be changed concurrently with calls to Get.
	// New（可选）指定一个函数，当Get否则将返回nil时生成一个值。它可能不会与Get调用同时更改。
	New func() interface{}
}

// Local per-P Pool appendix. // 本地每个P池附录。
type poolLocalInternal struct {
	private interface{} // Can be used only by the respective P. // 只能由相应的P使用。
	shared  poolChain   // Local P can pushHead/popHead; any P can popTail. // 本地P可以pushHead/popHead； 任何P都可以popTail。
}

type poolLocal struct {
	poolLocalInternal

	// Prevents false sharing on widespread platforms with
	// 128 mod (cache line size) = 0 .
	// 防止在128 mod（缓存行大小）= 0的广泛平台上进行虚假共享。
	pad [128 - unsafe.Sizeof(poolLocalInternal{})%128]byte
}

// from runtime
func fastrand() uint32

var poolRaceHash [128]uint64

// poolRaceAddr returns an address to use as the synchronization point
// for race detector logic. We don't use the actual pointer stored in x
// directly, for fear of conflicting with other synchronization on that address.
// Instead, we hash the pointer to get an index into poolRaceHash.
// See discussion on golang.org/cl/31589.
// poolRaceAddr返回一个地址，用作竞态检测器逻辑的同步点。 我们不直接使用存储在x中的实际指针，以免与该地址上的其他同步发生冲突。
// 相反，我们对指针进行哈希处理以获取poolRaceHash的索引。
// 请参阅golang.org/cl/31589上的讨论。
func poolRaceAddr(x interface{}) unsafe.Pointer {
	ptr := uintptr((*[2]unsafe.Pointer)(unsafe.Pointer(&x))[1])
	h := uint32((uint64(uint32(ptr)) * 0x85ebca6b) >> 16)
	return unsafe.Pointer(&poolRaceHash[h%uint32(len(poolRaceHash))])
}

// Put adds x to the pool. // 将x添加到池中
func (p *Pool) Put(x interface{}) {
	if x == nil { // 空数据不入池
		return
	}
	if race.Enabled {
		if fastrand()%4 == 0 {
			// Randomly drop x on floor.
			return
		}
		race.ReleaseMerge(poolRaceAddr(x))
		race.Disable()
	}
	l, _ := p.pin()
	if l.private == nil { // 优先放到私有缓存中
		l.private = x
		x = nil
	}
	if x != nil { // 放到共享缓存中
		l.shared.pushHead(x)
	}
	runtime_procUnpin()
	if race.Enabled {
		race.Enable()
	}
}

// Get selects an arbitrary item from the Pool, removes it from the
// Pool, and returns it to the caller.
// Get may choose to ignore the pool and treat it as empty.
// Callers should not assume any relation between values passed to Put and
// the values returned by Get.
//
// If Get would otherwise return nil and p.New is non-nil, Get returns
// the result of calling p.New.
//
// Get从池中选择一个任意项，将其从池中删除，然后将其返回给调用方。
// Get可以选择忽略池并将其视为空。
// 调用者不应假定传递给Put的值和Get返回的值之间有任何关系。
//
// 如果Get返回nil并且p.New不为nil，则Get返回调用p.New的结果。
func (p *Pool) Get() interface{} {
	if race.Enabled {
		race.Disable()
	}
	l, pid := p.pin()
	x := l.private
	l.private = nil
	if x == nil {
		// Try to pop the head of the local shard. We prefer
		// the head over the tail for temporal locality of
		// reuse.
		// 尝试弹出本地分片的头部。 对于重用的时间局部性，头部优先于尾部。
		x, _ = l.shared.popHead()
		if x == nil {
			x = p.getSlow(pid)
		}
	}
	runtime_procUnpin()
	if race.Enabled {
		race.Enable()
		if x != nil {
			race.Acquire(poolRaceAddr(x))
		}
	}
	if x == nil && p.New != nil { // 没有取到就新创建一个
		x = p.New()
	}
	return x
}

func (p *Pool) getSlow(pid int) interface{} {
	// See the comment in pin regarding ordering of the loads. // 有关负载的排序，请参见pin中的注释。
	size := atomic.LoadUintptr(&p.localSize) // load-acquire
	locals := p.local                        // load-consume
	// Try to steal one element from other procs. // 尝试从其他进程中窃取一个元素。
	for i := 0; i < int(size); i++ {
		l := indexLocal(locals, (pid+i+1)%int(size))
		if x, _ := l.shared.popTail(); x != nil {
			return x
		}
	}

	// Try the victim cache. We do this after attempting to steal
	// from all primary caches because we want objects in the
	// victim cache to age out if at all possible.
	// 尝试受害者缓存。 我们尝试从所有主缓存中窃取后执行此操作，因为我们希望受害者缓存中的对象尽可能地老化。
	size = atomic.LoadUintptr(&p.victimSize)
	if uintptr(pid) >= size {
		return nil
	}
	locals = p.victim
	l := indexLocal(locals, pid)
	if x := l.private; x != nil {
		l.private = nil
		return x
	}
	for i := 0; i < int(size); i++ {
		l := indexLocal(locals, (pid+i)%int(size))
		if x, _ := l.shared.popTail(); x != nil {
			return x
		}
	}

	// Mark the victim cache as empty for future gets don't bother
	// with it.
	// 将受害者缓存标记为空，以备将来使用时不要打扰。
	atomic.StoreUintptr(&p.victimSize, 0)

	return nil
}

// pin pins the current goroutine to P, disables preemption and
// returns poolLocal pool for the P and the P's id.
// Caller must call runtime_procUnpin() when done with the pool.
// pin将当前goroutine固定为P，禁用抢占并返回P和P的ID的poolLocal pool。
// 完成对池的操作后，调用者必须调用runtime_procUnpin（）。
func (p *Pool) pin() (*poolLocal, int) {
	pid := runtime_procPin()
	// In pinSlow we store to local and then to localSize, here we load in opposite order.
	// Since we've disabled preemption, GC cannot happen in between.
	// Thus here we must observe local at least as large localSize.
	// We can observe a newer/larger local, it is fine (we must observe its zero-initialized-ness).
	//
	// 在pinSlow中，我们先存储到local，然后再存储到localSize，这里我们以相反的顺序加载。
    // 由于我们已禁用抢占，因此GC不能在两者之间发生。
    // 因此，在这里我们必须观察local至少与localSize一样大。
    // 我们可以观察到一个新的/更大的local，这很好（我们必须观察其零初始化度）。
	s := atomic.LoadUintptr(&p.localSize) // load-acquire
	l := p.local                          // load-consume
	if uintptr(pid) < s {
		return indexLocal(l, pid), pid
	}
	return p.pinSlow()
}

func (p *Pool) pinSlow() (*poolLocal, int) {
	// Retry under the mutex.
	// Can not lock the mutex while pinned.
	// 在互斥锁下重试。 pin后无法锁定互斥锁。
	runtime_procUnpin()
	allPoolsMu.Lock()
	defer allPoolsMu.Unlock()
	pid := runtime_procPin()
	// poolCleanup won't be called while we are pinned.
	// 当我们pin时，不会调用poolCleanup。
	s := p.localSize
	l := p.local
	if uintptr(pid) < s {
		return indexLocal(l, pid), pid
	}
	if p.local == nil {
		allPools = append(allPools, p)
	}
	// If GOMAXPROCS changes between GCs, we re-allocate the array and lose the old one.
	// 如果GOMAXPROCS在GC之间更改，我们将重新分配该数组，并丢失旧数组。
	size := runtime.GOMAXPROCS(0)
	local := make([]poolLocal, size)
	atomic.StorePointer(&p.local, unsafe.Pointer(&local[0])) // store-release
	atomic.StoreUintptr(&p.localSize, uintptr(size))         // store-release
	return &local[pid], pid
}

func poolCleanup() {
	// This function is called with the world stopped, at the beginning of a garbage collection.
	// It must not allocate and probably should not call any runtime functions.
	// 在垃圾回收开始时，在全局停止的情况下调用此函数。 它不得分配，并且可能不应调用任何运行时函数。

	// Because the world is stopped, no pool user can be in a
	// pinned section (in effect, this has all Ps pinned).
	// 因为全局已停止，所以没有任何池用户可以处于pin代码段（实际上，所有P都固定了）。

	// Drop victim caches from all pools. // 从所有池中删除受害者缓存。
	for _, p := range oldPools {
		p.victim = nil
		p.victimSize = 0
	}

	// Move primary cache to victim cache. // 将主缓存移到受害者缓存。
	for _, p := range allPools {
		p.victim = p.local
		p.victimSize = p.localSize
		p.local = nil
		p.localSize = 0
	}

	// The pools with non-empty primary caches now have non-empty
	// victim caches and no pools have primary caches.
	// 具有非空主缓存的池现在具有非空受害者缓存，没有池具有主缓存。
	oldPools, allPools = allPools, nil
}

var (
	allPoolsMu Mutex

	// allPools is the set of pools that have non-empty primary
	// caches. Protected by either 1) allPoolsMu and pinning or 2)
	// STW.
	// allPools是具有非空主缓存的池的集合。 由1）allPoolsMu和pinning 或2）STW保护。
	allPools []*Pool

	// oldPools is the set of pools that may have non-empty victim
	// caches. Protected by STW.
	// oldPools是可能具有非空受害者缓存的一组池。 受STW保护。
	oldPools []*Pool
)

func init() {
	runtime_registerPoolCleanup(poolCleanup)
}

func indexLocal(l unsafe.Pointer, i int) *poolLocal {
	lp := unsafe.Pointer(uintptr(l) + uintptr(i)*unsafe.Sizeof(poolLocal{}))
	return (*poolLocal)(lp)
}

// Implemented in runtime.
func runtime_registerPoolCleanup(cleanup func())
func runtime_procPin() int
func runtime_procUnpin()
```