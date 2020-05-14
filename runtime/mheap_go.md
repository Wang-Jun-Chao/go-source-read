```go

// Page heap.
//
// See malloc.go for overview.
// 页面堆。
//
// 有关概述，请参见malloc.go。

package runtime

import (
	"internal/cpu"
	"runtime/internal/atomic"
	"runtime/internal/sys"
	"unsafe"
)

const (
	// minPhysPageSize is a lower-bound on the physical page size. The
	// true physical page size may be larger than this. In contrast,
	// sys.PhysPageSize is an upper-bound on the physical page size.
	//
	// minPhysPageSize是物理页面大小的下限。 实际的物理页面大小可能大于此大小。
	// 相反，sys.PhysPageSize是物理页面大小的上限。
	minPhysPageSize = 4096 // 4KB

	// maxPhysPageSize is the maximum page size the runtime supports.
	//
	// maxPhysPageSize是运行时支持的最大页面大小。
	maxPhysPageSize = 512 << 10 // 512KB

	// maxPhysHugePageSize sets an upper-bound on the maximum huge page size
	// that the runtime supports.
	//
	// maxPhysHugePageSize设置运行时支持的最大大页面大小的上限。
	maxPhysHugePageSize = pallocChunkBytes
)

// Main malloc heap.
// The heap itself is the "free" and "scav" treaps,
// but all the other global data is here too.
//
// mheap must not be heap-allocated because it contains mSpanLists,
// which must not be heap-allocated.
//
// 主malloc堆
// 堆本身就是“free”和“scav”，但是所有其他全局数据也都在这里。
//
// mheap一定不能进行堆分配，因为它包含mSpanLists，而mSpanLists不能进行堆分配。
//
//go:notinheap
type mheap struct {
	// lock must only be acquired on the system stack, otherwise a g
	// could self-deadlock if its stack grows with the lock held.
	// 必须仅在系统堆栈上获取锁，否则，如果g的堆栈在持有锁的情况下增长，则g可能会自锁。
	lock      mutex
	pages     pageAlloc // page allocation data structure // 页面分配数据结构
	sweepgen  uint32    // sweep generation, see comment in mspan; written during STW //扫描代数，请参见mspan中的注释；在STW期间被写
	sweepdone uint32    // all spans are swept // 所有的span都被扫描过
	sweepers  uint32    // number of active sweepone calls // 活动的swapone调用数

	// allspans is a slice of all mspans ever created. Each mspan
	// appears exactly once.
	//
	// The memory for allspans is manually managed and can be
	// reallocated and move as the heap grows.
	//
	// In general, allspans is protected by mheap_.lock, which
	// prevents concurrent access as well as freeing the backing
	// store. Accesses during STW might not hold the lock, but
	// must ensure that allocation cannot happen around the
	// access (since that may free the backing store).
	//
	// allspans是所有创建的mspan的切片。每个mspan仅出现一次。
    //
    // allspan的内存是手动管理的，可以随着堆的增长而重新分配和移动。
    //
    // 通常，allspans受mheap_.lock保护，这可以防止并发访问以及释放后备存储。
    // STW期间的访问可能不持有该锁，但必须确保在访问附近不会发生分配（因为这可能释放后备存储）。
	allspans []*mspan // all spans out there // 所有span都在那里

	// sweepSpans contains two mspan stacks: one of swept in-use
	// spans, and one of unswept in-use spans. These two trade
	// roles on each GC cycle. Since the sweepgen increases by 2
	// on each cycle, this means the swept spans are in
	// sweepSpans[sweepgen/2%2] and the unswept spans are in
	// sweepSpans[1-sweepgen/2%2]. Sweeping pops spans from the
	// unswept stack and pushes spans that are still in-use on the
	// swept stack. Likewise, allocating an in-use span pushes it
	// on the swept stack.
	//
	// scanSpans包含两个mspan栈：一个是已扫描的使用跨度，另一个是未扫描的使用跨度。
	// 在每个GC周期中，这两个角色互相交换。由于sweepgen在每个周期上增加2，
	// 这意味着已扫描跨度在sweepSpans[sweepgen/2%2]中，而未扫跨度在sweepSpans[1-sweepgen/2%2]中。
	// 扫描运作从未扫描的栈中出栈跨度，并将出栈跨度入栈到使用中的已扫跨度。
	// 同样，分配使用中的跨度会将其压入已扫描栈。
	sweepSpans [2]gcSweepBuf

	// _ uint32 // align uint64 fields on 32-bit for atomics // 对齐32位的uint64字段以方便原子操作，不使用

	// Proportional sweep
	//
	// These parameters represent a linear function from heap_live
	// to page sweep count. The proportional sweep system works to
	// stay in the black by keeping the current page sweep count
	// above this line at the current heap_live.
	//
	// The line has slope sweepPagesPerByte and passes through a
	// basis point at (sweepHeapLiveBasis, pagesSweptBasis). At
	// any given time, the system is at (memstats.heap_live,
	// pagesSwept) in this space.
	//
	// It's important that the line pass through a point we
	// control rather than simply starting at a (0,0) origin
	// because that lets us adjust sweep pacing at any time while
	// accounting for current progress. If we could only adjust
	// the slope, it would create a discontinuity in debt if any
	// progress has already been made.
	//
	// 比例扫描
    //
    // 这些参数表示从heap_live到页面扫描计数的线性函数。
    // 比例扫描系统通过将当前页面扫描计数保持在当前heap_live的这一线条之上，从而保持黑色状态。
    //
    // 该线条的坡度为sweepPagesPerByte，并通过(sweepHeapLiveBasis, pagesSweptBasis)的基点。
    // 在任何给定时间，系统都位于该空间中的(memstats.heap_live, pagesSwept)。
    //
    // 重要的是，线条要通过我们控制的点，而不是简单地从(0,0)原点开始，因为这可以让我们在考虑当前进度的同时随时调整扫描步调。
    // 如果我们只能调整斜率，那么如果已经取得任何进展，就会造成债务的不连续性。
	pagesInUse         uint64  // pages of spans in stats mSpanInUse; updated atomically // 统计信息中的跨度页面mSpanInUse；原子更新
	pagesSwept         uint64  // pages swept this cycle; updated atomically // 当前周期已经扫描的内存页数；原子更新
	pagesSweptBasis    uint64  // pagesSwept to use as the origin of the sweep ratio; updated atomically // pagesSwept用作扫描率的来源；原子更新
	sweepHeapLiveBasis uint64  // value of heap_live to use as the origin of sweep ratio; written with lock, read without // 将heap_live的值用作扫描比率的起点；带锁写，不带读
	sweepPagesPerByte  float64 // proportional sweep ratio; written with lock, read without // 比例扫描比；带锁写，不带读
	// TODO(austin): pagesInUse should be a uintptr, but the 386
	// compiler can't 8-byte align fields.
	// TODO(austin): pagesInUse应该是uintptr，但是386编译器不能使用8字节对齐字段。

	// scavengeGoal is the amount of total retained heap memory (measured by
	// heapRetained) that the runtime will try to maintain by returning memory
	// to the OS.
	// scavengeGoal是运行时通过将内存返回给OS来尝试维护的总保留堆内存量（由heapRetained测量）。
	scavengeGoal uint64

	// Page reclaimer state
	// 页面回收状态

	// reclaimIndex is the page index in allArenas of next page to
	// reclaim. Specifically, it refers to page (i %
	// pagesPerArena) of arena allArenas[i / pagesPerArena].
	//
	// If this is >= 1<<63, the page reclaimer is done scanning
	// the page marks.
	//
	// This is accessed atomically.
	//
	// reclaimIndex是要回收的下一页allArenas中的页面索引。
	// 具体来说，它是指竞技场allArenas[i/pagesPerArena]的页面(i%pagesPerArena)。
    //
    // 如果值 >> 1<<63，则页面取回器将完成对页面标记的扫描。
    //
    // 这是原子访问的
	reclaimIndex uint64
	// reclaimCredit is spare credit for extra pages swept. Since
	// the page reclaimer works in large chunks, it may reclaim
	// more than requested. Any spare pages released go to this
	// credit pool.
	//
	// This is accessed atomically.
	//
	// reclaimCredit是备用信用，可用于扫除多余的页面。
	// 由于页面回收器的工作量很大，因此它的回收量可能超出请求的数量。释放的所有备用页面都将进入此信用池。
    //
    // 这是原子访问的。
	reclaimCredit uintptr

	// Malloc stats.
	// 分配统计
	largealloc  uint64                  // bytes allocated for large objects // 为大对象分配的字节
	nlargealloc uint64                  // number of large object allocations // 大对象分配的数量
	largefree   uint64                  // bytes freed for large objects (>maxsmallsize) // 大对象释放的字节数（> maxsmallsize）
	nlargefree  uint64                  // number of frees for large objects (>maxsmallsize) // 大对象的释放次数（> maxsmallsize）
	nsmallfree  [_NumSizeClasses]uint64 // number of frees for small objects (<=maxsmallsize) // 小对象的释放次数（<= maxsmallsize）

	// arenas is the heap arena map. It points to the metadata for
	// the heap for every arena frame of the entire usable virtual
	// address space.
	//
	// Use arenaIndex to compute indexes into this array.
	//
	// For regions of the address space that are not backed by the
	// Go heap, the arena map contains nil.
	//
	// Modifications are protected by mheap_.lock. Reads can be
	// performed without locking; however, a given entry can
	// transition from nil to non-nil at any time when the lock
	// isn't held. (Entries never transitions back to nil.)
	//
	// In general, this is a two-level mapping consisting of an L1
	// map and possibly many L2 maps. This saves space when there
	// are a huge number of arena frames. However, on many
	// platforms (even 64-bit), arenaL1Bits is 0, making this
	// effectively a single-level map. In this case, arenas[0]
	// will never be nil.
	//
	// arenas是堆arena map。它指向整个可用虚拟地址空间中每个竞技场帧的堆元数据。
    //
    // 使用arenaIndex来计算该数组的索引。
    //
    // 对于地址空间中未被Go堆支持的区域，arena map包含nil。
    //
    // 修改受mheap_.lock保护。读取时可以不加锁；但是，在不持有锁的任何时候，给定的条目都可以从nil变为non-nil。
    // （条目永远不会转换回零。）
    //
    // 通常，这是一个两级映射，由一个L1映射和可能的许多L2映射组成。当存在大量的竞技场框架时，这可以节省空间。
    // 但是，在许多平台（甚至是64位）上，arenaL1Bits也为0，这实际上使它成为单级映射。在这种情况下，arenas [0]永远不会为零。
	arenas [1 << arenaL1Bits]*[1 << arenaL2Bits]*heapArena

	// heapArenaAlloc is pre-reserved space for allocating heapArena
	// objects. This is only used on 32-bit, where we pre-reserve
	// this space to avoid interleaving it with the heap itself.
	//
	// heapArenaAlloc是用于分配heapArena对象的预保留空间。它仅用于32位，我们在此处预先保留此空间，以避免与堆本身交错。
	heapArenaAlloc linearAlloc

	// arenaHints is a list of addresses at which to attempt to
	// add more heap arenas. This is initially populated with a
	// set of general hint addresses, and grown with the bounds of
	// actual heap arena ranges.
	//
	// arenaHints是要尝试在其中添加更多堆竞技场的地址的列表。
	// 最初使用一组常规hint地址进行填充，然后使用实际堆竞技场范围的边界进行扩展。
	arenaHints *arenaHint

	// arena is a pre-reserved space for allocating heap arenas
	// (the actual arenas). This is only used on 32-bit.
	//
	// arena是用于分配堆竞技场（实际竞技场）的预留空间。仅在32位上使用。
	arena linearAlloc

	// allArenas is the arenaIndex of every mapped arena. This can
	// be used to iterate through the address space.
	//
	// Access is protected by mheap_.lock. However, since this is
	// append-only and old backing arrays are never freed, it is
	// safe to acquire mheap_.lock, copy the slice header, and
	// then release mheap_.lock.
	//
	// allArenas是每个映射的竞技场的arenaIndex。这可用于遍历地址空间。
    //
    // 访问受mheap_.lock保护。但是，由于这仅是追加操作，并且永远不会释放旧的支持数组，
    // 因此可以安全地获取mheap_.lock，复制切片头，然后释放mheap_.lock。
	allArenas []arenaIdx

	// sweepArenas is a snapshot of allArenas taken at the
	// beginning of the sweep cycle. This can be read safely by
	// simply blocking GC (by disabling preemption).
	//
	// scanArenas是在扫描周期开始时获取的所有Arenas的快照。可以通过简单地阻止GC（通过禁用抢占）来安全地读取它。
	sweepArenas []arenaIdx

	// curArena is the arena that the heap is currently growing
	// into. This should always be physPageSize-aligned.
	//
	// curArena是堆当前正在成长的竞技场。这应该始终与physPageSize对齐。
	curArena struct {
		base, end uintptr
	}

	_ uint32 // ensure 64-bit alignment of central // 确保中央的64位对齐

	// central free lists for small size classes.
	// the padding makes sure that the mcentrals are
	// spaced CacheLinePadSize bytes apart, so that each mcentral.lock
	// gets its own cache line.
	// central is indexed by spanClass.
	//
	// 小型类别的中央空闲列表。
    // 填充确保mcentral被CacheLinePadSize字节隔开，以便每个mcentral.lock都有自己的缓存行。central由spanClass索引。
	central [numSpanClasses]struct {
		mcentral mcentral
		pad      [cpu.CacheLinePadSize - unsafe.Sizeof(mcentral{})%cpu.CacheLinePadSize]byte
	}

	spanalloc             fixalloc // allocator for span* // span*的分配器
	cachealloc            fixalloc // allocator for mcache* // mcache*的分配器
	specialfinalizeralloc fixalloc // allocator for specialfinalizer* // specialfinalizer*的分配器
	specialprofilealloc   fixalloc // allocator for specialprofile* // specialprofile*的分配器
	speciallock           mutex    // lock for special record allocators. // 锁定特殊记录分配器。
	arenaHintAlloc        fixalloc // allocator for arenaHints // arenaHints的分配器

	unused *specialfinalizer // never set, just here to force the specialfinalizer type into DWARF // 从未设置，只是在此处将specialfinalizer类型强制为DWARF
}

var mheap_ mheap

// A heapArena stores metadata for a heap arena. heapArenas are stored
// outside of the Go heap and accessed via the mheap_.arenas index.
//
// eapArena存储堆竞技场的元数据。heapArenas存储在Go堆之外，并可以通过mheap_.arenas索引进行访问。
//go:notinheap
type heapArena struct {
	// bitmap stores the pointer/scalar bitmap for the words in
	// this arena. See mbitmap.go for a description. Use the
	// heapBits type to access this.
	// bitmap存储此竞技场中字（word）的指针/标量位图。有关说明，请参见mbitmap.go。使用heapBits类型来访问它。
	bitmap [heapArenaBitmapBytes]byte

	// spans maps from virtual address page ID within this arena to *mspan.
	// For allocated spans, their pages map to the span itself.
	// For free spans, only the lowest and highest pages map to the span itself.
	// Internal pages map to an arbitrary span.
	// For pages that have never been allocated, spans entries are nil.
	//
	// Modifications are protected by mheap.lock. Reads can be
	// performed without locking, but ONLY from indexes that are
	// known to contain in-use or stack spans. This means there
	// must not be a safe-point between establishing that an
	// address is live and looking it up in the spans array.
	//
	// spans从该区域内的虚拟地址页面ID映射到*mspan。
    // 对于已分配的跨度，其页面映射到跨度本身。
    // 对于空闲跨度，只有最低和最高页面会映射到跨度本身。
    // 内部页面映射到任意跨度。
    // 对于从未分配的页面，span条目为nil。
    //
    // 修改受mheap.lock保护。可以在不锁定的情况下执行读取，但是只能从已知包含使用中或栈跨度的索引中进行。
    // 这意味着在确定地址是否存在以及在spans数组中查找地址之间一定没有安全点。
	spans [pagesPerArena]*mspan

	// pageInUse is a bitmap that indicates which spans are in
	// state mSpanInUse. This bitmap is indexed by page number,
	// but only the bit corresponding to the first page in each
	// span is used.
	//
	// Reads and writes are atomic.
	//
	// pageInUse是一个位图，指示哪些跨度处于mSpanInUse状态。该位图由页码索引，但仅使用每个跨度中与第一页相对应的位。
    //
    // 读写是原子的。
	pageInUse [pagesPerArena / 8]uint8

	// pageMarks is a bitmap that indicates which spans have any
	// marked objects on them. Like pageInUse, only the bit
	// corresponding to the first page in each span is used.
	//
	// Writes are done atomically during marking. Reads are
	// non-atomic and lock-free since they only occur during
	// sweeping (and hence never race with writes).
	//
	// This is used to quickly find whole spans that can be freed.
	//
	// TODO(austin): It would be nice if this was uint64 for
	// faster scanning, but we don't have 64-bit atomic bit
	// operations.
	//
	// pageMarks是一个位图，指示哪些跨度上有任何标记的对象。与pageInUse一样，仅使用与每个跨度中第一页相对应的位。
    //
    // 在标记期间自动完成写入。读取是非原子且无锁的，因为它们仅在扫描期间发生（因此从不与写入竞争）。
    //
    // 这用于快速查找可以释放的整个跨度。
    //
    // TODO（austin）：如果这是uint64可以进行更快的扫描，那很好，但是我们没有64位原子位操作。
	pageMarks [pagesPerArena / 8]uint8

	// zeroedBase marks the first byte of the first page in this
	// arena which hasn't been used yet and is therefore already
	// zero. zeroedBase is relative to the arena base.
	// Increases monotonically until it hits heapArenaBytes.
	//
	// This field is sufficient to determine if an allocation
	// needs to be zeroed because the page allocator follows an
	// address-ordered first-fit policy.
	//
	// Read atomically and written with an atomic CAS.
	//
	// zeroedBase标记了此arena上第一页的第一个字节，该字节尚未使用，因此已经为零。 zeroedBase相对于arena基址。
    // 单调递增，直到达到heapArenaBytes。
    //
    // 此字段足以确定是否需要将分配清零，因为页面分配器遵循地址排序的第一适配策略。
    //
    // 原子读取并使用原子CAS编写。
	zeroedBase uintptr
}

// arenaHint is a hint for where to grow the heap arenas. See
// mheap_.arenaHints.
// arenaHint是有关在哪里扩展堆竞技场的hint。请参阅mheap_.arenaHints。
//
//go:notinheap
type arenaHint struct {
	addr uintptr
	down bool
	next *arenaHint
}

// An mspan is a run of pages.
//
// When a mspan is in the heap free treap, state == mSpanFree
// and heapmap(s->start) == span, heapmap(s->start+s->npages-1) == span.
// If the mspan is in the heap scav treap, then in addition to the
// above scavenged == true. scavenged == false in all other cases.
//
// When a mspan is allocated, state == mSpanInUse or mSpanManual
// and heapmap(i) == span for all s->start <= i < s->start+s->npages.

// Every mspan is in one doubly-linked list, either in the mheap's
// busy list or one of the mcentral's span lists.

// An mspan representing actual memory has state mSpanInUse,
// mSpanManual, or mSpanFree. Transitions between these states are
// constrained as follows:
//
// * A span may transition from free to in-use or manual during any GC
//   phase.
//
// * During sweeping (gcphase == _GCoff), a span may transition from
//   in-use to free (as a result of sweeping) or manual to free (as a
//   result of stacks being freed).
//
// * During GC (gcphase != _GCoff), a span *must not* transition from
//   manual or in-use to free. Because concurrent GC may read a pointer
//   and then look up its span, the span state must be monotonic.
//
// Setting mspan.state to mSpanInUse or mSpanManual must be done
// atomically and only after all other span fields are valid.
// Likewise, if inspecting a span is contingent on it being
// mSpanInUse, the state should be loaded atomically and checked
// before depending on other fields. This allows the garbage collector
// to safely deal with potentially invalid pointers, since resolving
// such pointers may race with a span being allocated.
//
// mspan是一系列页面。
//
// 当mspan处于堆中free strap状态时，state == mSpanFree和heapmap（s-> start）== span，heapmap（s-> start + s-> npages-1）== span。
// 如果mspan在堆中free strap状态则除了上述 scavenged == true外。在所有其他情况下scavenged == false。
//
// 分配了mspan后，对所有s->start <= i < s->start + s->npages，有：state == mSpanInUse或mSpanManual和heapmap(i) == span
//
// 每个mspan都在一个双向链接列表中，要么在mheap的繁忙列表中，要么在mcentral的span列表中
//
// 表示实际内存的mspan的状态为mSpanInUse，mSpanManual或mSpanFree。这些状态之间的转换受以下约束：
//
// *在任何GC阶段，跨度可能会从免费变为使用中或手动。
//
// *在清除期间（gcphase == _GCoff），跨度可能从使用中转换为空闲（作为清除的结果），或者从手动过渡到释放（由于堆栈的释放）。
//
// *在GC（gcphase != _GCoff）期间，跨度*绝不能*从手动或使用中过渡到空闲。因为并发GC可能会读取一个指针然后查找其跨度，所以跨度状态必须是单调的。
//
// 必须原子地完成将mspan.state设置为mSpanInUse或mSpanManual的操作，并且必须在所有其他span字段均有效之后才能进行。
// 同样，如果检查跨度取决于mSpanInUse，则应原子地加载状态并在检查其他字段之前检查状态。这使垃圾回收器可以安全地处理潜在的无效指针，因为解析此类指针可能会与分配的跨度竞争。
type mSpanState uint8

const (
	mSpanDead   mSpanState = iota
	mSpanInUse             // allocated for garbage collected heap // 分配给垃圾收集堆
	mSpanManual            // allocated for manual management (e.g., stack allocator) // 分配用于手动管理（例如，堆栈分配器）
)

// mSpanStateNames are the names of the span states, indexed by
// mSpanState.
// mSpanStateNames是跨度状态的名称，由mSpanState索引。
var mSpanStateNames = []string{
	"mSpanDead",
	"mSpanInUse",
	"mSpanManual",
	"mSpanFree",
}

// mSpanStateBox holds an mSpanState and provides atomic operations on
// it. This is a separate type to disallow accidental comparison or
// assignment with mSpanState.
// mSpanStateBox拥有一个mSpanState并对其提供原子操作。这是一个单独的类型，以防止意外地与mSpanState进行比较或分配。
type mSpanStateBox struct {
	s mSpanState
}

func (b *mSpanStateBox) set(s mSpanState) {
	atomic.Store8((*uint8)(&b.s), uint8(s))
}

func (b *mSpanStateBox) get() mSpanState {
	return mSpanState(atomic.Load8((*uint8)(&b.s)))
}

/// mSpanList heads a linked list of spans.
/// mSpanList表示跨度的链接列表。
//go:notinheap
type mSpanList struct {
	first *mspan // first span in list, or nil if none
	last  *mspan // last span in list, or nil if none
}

//go:notinheap
type mspan struct {
	next *mspan     // next span in list, or nil if none
	prev *mspan     // previous span in list, or nil if none
	list *mSpanList // For debugging. TODO: Remove.

	startAddr uintptr // address of first byte of span aka s.base() // span的第一个字节的地址，也称为s.base()
	npages    uintptr // number of pages in span 跨度的页数

	manualFreeList gclinkptr // list of free objects in mSpanManual spans // mSpanManual跨度中的空闲对象列表

	// freeindex is the slot index between 0 and nelems at which to begin scanning
	// for the next free object in this span.
	// Each allocation scans allocBits starting at freeindex until it encounters a 0
	// indicating a free object. freeindex is then adjusted so that subsequent scans begin
	// just past the newly discovered free object.
	//
	// If freeindex == nelem, this span has no free objects.
	//
	// allocBits is a bitmap of objects in this span.
	// If n >= freeindex and allocBits[n/8] & (1<<(n%8)) is 0
	// then object n is free;
	// otherwise, object n is allocated. Bits starting at nelem are
	// undefined and should never be referenced.
	//
	// Object n starts at address n*elemsize + (start << pageShift).
	//
	// freeindex是介于0和nelem之间的槽位索引，从该位置开始扫描此跨度中的下一个空闲对象。
    // 每个分配都扫描从freeindex开始的allocBits，直到遇到表示空闲对象的0。 然后对freeindex进行调整，以使后续扫描刚开始经过新发现的空闲对象。
    //
    // 如果freeindex == nelem，则此跨度没有空闲对象。
    //
    // allocBits是此跨度内对象的位图。
    // 如果n >= freeindex并且allocBits[n/8] & (1<<(n%8))为0，则对象n为空闲； 否则，分配对象n。 从nelem开始的位是未定义的，永远不要被引用。
    //
    // 对象n从地址 n*elemsize + (start << pageShift)开始。
	freeindex uintptr
	// TODO: Look up nelems from sizeclass and remove this field if it
	// helps performance.
	// TODO: 从sizeclass中查找nelems，并在有助于提高性能的情况下删除此字段。
	nelems uintptr // number of object in the span. // 跨度中的对象数。

	// Cache of the allocBits at freeindex. allocCache is shifted
	// such that the lowest bit corresponds to the bit freeindex.
	// allocCache holds the complement of allocBits, thus allowing
	// ctz (count trailing zero) to use it directly.
	// allocCache may contain bits beyond s.nelems; the caller must ignore
	// these.
	//
	// 在freeindex处缓存allocBits。 移位allocCache使得最低位对应于空闲索引位。
	// allocCache保留allocBits的补数，从而允许ctz（计数尾随零）直接使用它。
	// allocCache可能包含s.nelems以外的位； 呼叫者必须忽略这些。
	allocCache uint64

	// allocBits and gcmarkBits hold pointers to a span's mark and
	// allocation bits. The pointers are 8 byte aligned.
	// There are three arenas where this data is held.
	// free: Dirty arenas that are no longer accessed
	//       and can be reused.
	// next: Holds information to be used in the next GC cycle.
	// current: Information being used during this GC cycle.
	// previous: Information being used during the last GC cycle.
	// A new GC cycle starts with the call to finishsweep_m.
	// finishsweep_m moves the previous arena to the free arena,
	// the current arena to the previous arena, and
	// the next arena to the current arena.
	// The next arena is populated as the spans request
	// memory to hold gcmarkBits for the next GC cycle as well
	// as allocBits for newly allocated spans.
	//
	// The pointer arithmetic is done "by hand" instead of using
	// arrays to avoid bounds checks along critical performance
	// paths.
	// The sweep will free the old allocBits and set allocBits to the
	// gcmarkBits. The gcmarkBits are replaced with a fresh zeroed
	// out memory.
	//
	// allocBits和gcmarkBits保存指向跨度标记和分配位的指针。指针是8字节对齐的。
    // 保留了三个数据的竞技场。
    // free：不再访问且可以重用的肮脏竞技场。
    // next：保存要在下一个GC周期中使用的信息。
    // current：此GC周期中正在使用的信息。
    // previous：上一个GC周期中正在使用的信息。
    // 一个新的GC周期从对finishsweep_m的调用开始。
    // finishsweep_m将previous竞技场移至free竞技场，将current的竞技场移至previous的竞技场，并将next竞技场移至current竞技场。
    // 将next竞技场填充为跨度请求内存，以保存下一个GC周期的gcmarkBits以及新分配的跨度的allocBits。
    //
    // 指针算术是“手动”完成的，而不是使用数组来避免沿关键性能路径进行边界检查。
    // 扫描将释放旧的allocBits，并将allocBits设置为gcmarkBits。 gcmarkBits被替换为新的清零内存。
	allocBits  *gcBits
	gcmarkBits *gcBits

	// sweep generation:
	// if sweepgen == h->sweepgen - 2, the span needs sweeping
	// if sweepgen == h->sweepgen - 1, the span is currently being swept
	// if sweepgen == h->sweepgen, the span is swept and ready to use
	// if sweepgen == h->sweepgen + 1, the span was cached before sweep began and is still cached, and needs sweeping
	// if sweepgen == h->sweepgen + 3, the span was swept and then cached and is still cached
	// h->sweepgen is incremented by 2 after every GC
	//
	// 扫描生成：
    // 如果sweepgen == h->sweepgen-2，则需要扫描
    // 如果sweepgen == h->sweepgen-1，则正在扫描当前跨度
    // 如果sweepgen == h->sweepgen，则扫描并准备使用
    // 如果sweepgen == h->sweepgen + 1，则扫描在扫描开始之前已被缓存，并且仍然被缓存，需要进行扫描
    // 如果sweepgen == h->sweepgen + 3，则扫掠跨度，然后将其缓存并仍然缓存
    // 每个GC之后，h->sweepgen将增加2

	sweepgen    uint32
	divMul      uint16        // for divide by elemsize - divMagic.mul // 除以elemsize-divMagic.mul
	baseMask    uint16        // if non-0, elemsize is a power of 2, & this will get object allocation base // 如果非零，则elemsize是2的幂，这将获得对象分配的基址
	allocCount  uint16        // number of allocated objects // 分配的对象数
	spanclass   spanClass     // size class and noscan (uint8) // 大小类别和noscan（uint8）
	state       mSpanStateBox // mSpanInUse etc; accessed atomically (get/set methods) // mSpanInUse等；原子访问（获取/设置方法）
	needzero    uint8         // needs to be zeroed before allocation // 分配前需要清零
	divShift    uint8         // for divide by elemsize - divMagic.shift // 除以elemsize-divMagic.shift
	divShift2   uint8         // for divide by elemsize - divMagic.shift2 // 除以elemsize-divMagic.shift2
	elemsize    uintptr       // computed from sizeclass or from npages // 从sizeclass或npages计算
	limit       uintptr       // end of data in span // 跨度中的结尾数据
	speciallock mutex         // guards specials list // 保护special列表
	specials    *special      // linked list of special records sorted by offset. // 记录special的链接列表，按偏移量排序。
}

// 计算跨度的基址
func (s *mspan) base() uintptr {
	return s.startAddr
}

// 计算跨度的底层信息，返回元素大小，可容纳的元素数，跨度总字节数
func (s *mspan) layout() (size, n, total uintptr) {
	total = s.npages << _PageShift
	size = s.elemsize
	if size > 0 {
		n = total / size
	}
	return
}

// recordspan adds a newly allocated span to h.allspans.
//
// This only happens the first time a span is allocated from
// mheap.spanalloc (it is not called when a span is reused).
//
// Write barriers are disallowed here because it can be called from
// gcWork when allocating new workbufs. However, because it's an
// indirect call from the fixalloc initializer, the compiler can't see
// this.
//
// recordspan将新分配的跨度添加到h.allspans。
//
// 仅在第一次从mheap.spanalloc中分配跨度时才会发生这种情况（当重新使用跨度时不会调用它）。
//
// 这里不允许写障碍，因为在分配新的工作缓冲区时可以从gcWork调用它。
// 但是，由于它是来自fixalloc初始化程序的间接调用，因此编译器看不到这一点。
//
//go:nowritebarrierrec
func recordspan(vh unsafe.Pointer, p unsafe.Pointer) {
	h := (*mheap)(vh)
	s := (*mspan)(p)
	if len(h.allspans) >= cap(h.allspans) {
		n := 64 * 1024 / sys.PtrSize // 默认：32位，64位机器： 64*1024/(4|8) = 16384|8196
		if n < cap(h.allspans)*3/2 { // 增长量小于伴，默认就是增长一半
			n = cap(h.allspans) * 3 / 2
		}
		var new []*mspan
		sp := (*slice)(unsafe.Pointer(&new))
		sp.array = sysAlloc(uintptr(n)*sys.PtrSize, &memstats.other_sys)
		if sp.array == nil {
			throw("runtime: cannot allocate memory")
		}
		sp.len = len(h.allspans)
		sp.cap = n
		if len(h.allspans) > 0 { // 拷贝数据
			copy(new, h.allspans)
		}
		oldAllspans := h.allspans
		*(*notInHeapSlice)(unsafe.Pointer(&h.allspans)) = *(*notInHeapSlice)(unsafe.Pointer(&new))
		if len(oldAllspans) != 0 { // 释放旧数据内存
			sysFree(unsafe.Pointer(&oldAllspans[0]), uintptr(cap(oldAllspans))*unsafe.Sizeof(oldAllspans[0]), &memstats.other_sys)
		}
	}
	h.allspans = h.allspans[:len(h.allspans)+1]
	h.allspans[len(h.allspans)-1] = s
}

// A spanClass represents the size class and noscan-ness of a span.
//
// Each size class has a noscan spanClass and a scan spanClass. The
// noscan spanClass contains only noscan objects, which do not contain
// pointers and thus do not need to be scanned by the garbage
// collector.
//
// spanClass表示跨度的大小类别和无扫描跨度。
//
// 每个大小类都有一个noscan spanClass 和一个scan spanClass。
// noscan spanClass仅包含noscan对象，该对象不包含指针，因此不需要由垃圾收集器进行扫描。
type spanClass uint8

const (
	numSpanClasses = _NumSizeClasses << 1 // 跨度类别数目
	tinySpanClass  = spanClass(tinySizeClass<<1 | 1) //
)

// 最低位表示是否需要扫描
func makeSpanClass(sizeclass uint8, noscan bool) spanClass {
	return spanClass(sizeclass<<1) | spanClass(bool2int(noscan))
}


func (sc spanClass) sizeclass() int8 {
	return int8(sc >> 1)
}

func (sc spanClass) noscan() bool {
	return sc&1 != 0
}

// arenaIndex returns the index into mheap_.arenas of the arena
// containing metadata for p. This index combines of an index into the
// L1 map and an index into the L2 map and should be used as
// mheap_.arenas[ai.l1()][ai.l2()].
//
// If p is outside the range of valid heap addresses, either l1() or
// l2() will be out of bounds.
//
// It is nosplit because it's called by spanOf and several other
// nosplit functions.
//
// arenaIndex将索引返回mheap_.arenas中包含p元数据的arena。
// 该索引将L1映射中的索引和L2映射中的索引组合在一起，应用作mheap_.arenas[ai.l1()] [ai.l2()]。
//
// 如果p在有效堆地址范围之外，则l1()或l2()都将超出范围。
//
// 它是nosplit的，因为它是由spanOf和其他几个nosplit函数调用的。
//
//go:nosplit
func arenaIndex(p uintptr) arenaIdx {
	return arenaIdx((p + arenaBaseOffset) / heapArenaBytes)
}

// arenaBase returns the low address of the region covered by heap
// arena i.
// arenaBase返回堆i覆盖的arena区域的低地址。
func arenaBase(i arenaIdx) uintptr {
	return uintptr(i)*heapArenaBytes - arenaBaseOffset
}

type arenaIdx uint

func (i arenaIdx) l1() uint {
	if arenaL1Bits == 0 {
		// Let the compiler optimize this away if there's no
		// L1 map.
		// 如果没有L1映射，让编译器对其进行优化。
		return 0
	} else {
		return uint(i) >> arenaL1Shift
	}
}

func (i arenaIdx) l2() uint {
	if arenaL1Bits == 0 {
		return uint(i)
	} else {
		return uint(i) & (1<<arenaL2Bits - 1)
	}
}

// inheap reports whether b is a pointer into a (potentially dead) heap object.
// It returns false for pointers into mSpanManual spans.
// Non-preemptible because it is used by write barriers.
//
// inheap报告b是否是指向（可能已死）堆对象的指针。
// 对于指向mSpanManual跨度的指针，它返回false。
// 不可抢占，因为它被写屏障使用。
//go:nowritebarrier
//go:nosplit
func inheap(b uintptr) bool {
	return spanOfHeap(b) != nil
}

// inHeapOrStack is a variant of inheap that returns true for pointers
// into any allocated heap span.
// inHeapOrStack是inheap的一种变体，对于指向任何已分配堆范围的指针，它返回true。
//go:nowritebarrier
//go:nosplit
func inHeapOrStack(b uintptr) bool {
	s := spanOf(b)
	if s == nil || b < s.base() {
		return false
	}
	switch s.state.get() {
	case mSpanInUse, mSpanManual:
		return b < s.limit
	default:
		return false
	}
}

// spanOf returns the span of p. If p does not point into the heap
// arena or no span has ever contained p, spanOf returns nil.
//
// If p does not point to allocated memory, this may return a non-nil
// span that does *not* contain p. If this is a possibility, the
// caller should either call spanOfHeap or check the span bounds
// explicitly.
//
// Must be nosplit because it has callers that are nosplit.
//
// spanOf返回p的跨度。如果p没有指向堆arena或没有任何span包含p，则spanOf返回nil。
//
// 如果p没有指向分配的内存，则可能会返回一个非零跨度，该跨度*不*包含p。如果可能，调用者应调用spanOfHeap或显式检查跨度边界。
//
// 必须是nosplit的，因为它具有nosplit的调用方。
//
//go:nosplit
func spanOf(p uintptr) *mspan {
	// This function looks big, but we use a lot of constant
	// folding around arenaL1Bits to get it under the inlining
	// budget. Also, many of the checks here are safety checks
	// that Go needs to do anyway, so the generated code is quite
	// short.
	ri := arenaIndex(p)
	if arenaL1Bits == 0 {
		// If there's no L1, then ri.l1() can't be out of bounds but ri.l2() can.
		// 如果没有L1，则ri.l1()不能超出范围，但ri.l2()可以。
		if ri.l2() >= uint(len(mheap_.arenas[0])) {
			return nil
		}
	} else {
		// If there's an L1, then ri.l1() can be out of bounds but ri.l2() can't.
		// 如果没有L1，则ri.l1()不能超出范围，但ri.l2()可以。
		if ri.l1() >= uint(len(mheap_.arenas)) {
			return nil
		}
	}
	l2 := mheap_.arenas[ri.l1()]
	if arenaL1Bits != 0 && l2 == nil { // Should never happen if there's no L1. // 如果没有L1，则永远不会发生。
		return nil
	}
	ha := l2[ri.l2()]
	if ha == nil {
		return nil
	}
	return ha.spans[(p/pageSize)%pagesPerArena]
}

// spanOfUnchecked is equivalent to spanOf, but the caller must ensure
// that p points into an allocated heap arena.
//
// Must be nosplit because it has callers that are nosplit.
//
// spanOfUnchecked等效于spanOf，但是调用者必须确保p指向分配的堆空间。
//
// 必须是nosplit的，因为它具有nosplit的调用方。
//
//go:nosplit
func spanOfUnchecked(p uintptr) *mspan {
	ai := arenaIndex(p)
	return mheap_.arenas[ai.l1()][ai.l2()].spans[(p/pageSize)%pagesPerArena]
}

// spanOfHeap is like spanOf, but returns nil if p does not point to a
// heap object.
//
// Must be nosplit because it has callers that are nosplit.
//
// spanOfHeap类似于spanOf，但如果p没有指向堆对象，则返回nil。
// 必须是nosplit的，因为它具有nosplit的调用方。
//
//go:nosplit
func spanOfHeap(p uintptr) *mspan {
	s := spanOf(p)
	// s is nil if it's never been allocated. Otherwise, we check
	// its state first because we don't trust this pointer, so we
	// have to synchronize with span initialization. Then, it's
	// still possible we picked up a stale span pointer, so we
	// have to check the span's bounds.
	// 如果从未分配过，则s为零。否则，我们将首先检查其状态，因为我们不信任该指针，因此必须与跨度初始化进行同步。
	// 然后，仍然有可能我们选择了一个过时的跨度指针，因此我们必须检查跨度的范围。
	if s == nil || s.state.get() != mSpanInUse || p < s.base() || p >= s.limit {
		return nil
	}
	return s
}

// pageIndexOf returns the arena, page index, and page mask for pointer p.
// The caller must ensure p is in the heap.
// pageIndexOf返回指针p的heapArena，pageIdx和pageMask。调用者必须确保p在堆中。
func pageIndexOf(p uintptr) (arena *heapArena, pageIdx uintptr, pageMask uint8) {
	ai := arenaIndex(p)
	arena = mheap_.arenas[ai.l1()][ai.l2()]
	pageIdx = ((p / pageSize) / 8) % uintptr(len(arena.pageInUse))
	pageMask = byte(1 << ((p / pageSize) % 8))
	return
}

// Initialize the heap.
// 堆初始化
func (h *mheap) init() {
	h.spanalloc.init(unsafe.Sizeof(mspan{}), recordspan, unsafe.Pointer(h), &memstats.mspan_sys)
	h.cachealloc.init(unsafe.Sizeof(mcache{}), nil, nil, &memstats.mcache_sys)
	h.specialfinalizeralloc.init(unsafe.Sizeof(specialfinalizer{}), nil, nil, &memstats.other_sys)
	h.specialprofilealloc.init(unsafe.Sizeof(specialprofile{}), nil, nil, &memstats.other_sys)
	h.arenaHintAlloc.init(unsafe.Sizeof(arenaHint{}), nil, nil, &memstats.other_sys)

	// Don't zero mspan allocations. Background sweeping can
	// inspect a span concurrently with allocating it, so it's
	// important that the span's sweepgen survive across freeing
	// and re-allocating a span to prevent background sweeping
	// from improperly cas'ing it from 0.
	//
	// This is safe because mspan contains no heap pointers.
	//
	//不要将mspan分配设为零。后台扫描可以同时检查跨度和分配跨度，
	// 因此跨度的spangen在释放和重新分配跨度时必须幸存，
	// 以防止后台扫描将其不正确地从0设置为空，这一点很重要。
    //
    // 这是安全的，因为mspan不包含堆指针。
	h.spanalloc.zero = false

	// h->mapcache needs no init // h->mapcache不初需要初始化

    // 初始化中央列表
	for i := range h.central {
		h.central[i].mcentral.init(spanClass(i))
	}

    // 进行页初始化
	h.pages.init(&h.lock, &memstats.gc_sys)
}

// reclaim sweeps and reclaims at least npage pages into the heap.
// It is called before allocating npage pages to keep growth in check.
//
// reclaim implements the page-reclaimer half of the sweeper.
//
// h must NOT be locked.
//
// reclaim扫描并将至少npage页回收到堆中。
// 在分配npage页之前调用以保证增长检查。
//
// reclaim实现清半扫描的页面回收器。
//
// h不能被锁定。
func (h *mheap) reclaim(npage uintptr) {
	// This scans pagesPerChunk at a time. Higher values reduce
	// contention on h.reclaimPos, but increase the minimum
	// latency of performing a reclaim.
	//
	// Must be a multiple of the pageInUse bitmap element size.
	//
	// The time required by this can vary a lot depending on how
	// many spans are actually freed. Experimentally, it can scan
	// for pages at ~300 GB/ms on a 2.6GHz Core i7, but can only
	// free spans at ~32 MB/ms. Using 512 pages bounds this at
	// roughly 100µs.
	//
	// TODO(austin): Half of the time spent freeing spans is in
	// locking/unlocking the heap (even with low contention). We
	// could make the slow path here several times faster by
	// batching heap frees.
	//
	// 一次扫描pagesPerChunk。较高的值会减少h.reclaimPos上的争用，但会增加执行回收的最小延迟。
    //
    // 必须是pageInUse位图元素大小的倍数。
    //
    // 实际需要多少时间取决于实际释放的跨度。实验上，它可以在2.6GHz Core i7上以〜300GB/ms的速度扫描页面，
    // 但只能以〜32MB/ms的速度扫描跨度。使用512页限制了大约100µs的时间。
    //
    // TODO（austin）：释放跨度所花费的时间的一半是锁定/解锁堆（即使争用程度较低）。
    // 通过批量释放堆，我们可以使慢速路径快几倍。
	const pagesPerChunk = 512

	// Bail early if there's no more reclaim work.
	// 如果没有其他回收工作，请尽早保释。
	if atomic.Load64(&h.reclaimIndex) >= 1<<63 {
		return
	}

	// Disable preemption so the GC can't start while we're
	// sweeping, so we can read h.sweepArenas, and so
	// traceGCSweepStart/Done pair on the P.
	// 禁用抢占，以便在扫描时无法启动GC，因此我们可以读取h.sweepArenas，并在P上执行traceGCSweepStart/Done组合。
	mp := acquirem()

	if trace.enabled {
		traceGCSweepStart()
	}

	arenas := h.sweepArenas
	locked := false
	for npage > 0 {
		// Pull from accumulated credit first.
		// 首先从累积的信用中提取。
		if credit := atomic.Loaduintptr(&h.reclaimCredit); credit > 0 {
			take := credit
			if take > npage {
				// Take only what we need. // 仅获取我们需要的内容。
				take = npage
			}
			if atomic.Casuintptr(&h.reclaimCredit, credit, credit-take) {
				npage -= take
			}
			continue
		}

		// Claim a chunk of work. // 要求大量工作。
		idx := uintptr(atomic.Xadd64(&h.reclaimIndex, pagesPerChunk) - pagesPerChunk)
		if idx/pagesPerArena >= uintptr(len(arenas)) {
			// Page reclaiming is done. // 页面回收已完成。
			atomic.Store64(&h.reclaimIndex, 1<<63)
			break
		}

		if !locked {
			// Lock the heap for reclaimChunk. // 锁定堆以进行回收。
			lock(&h.lock)
			locked = true
		}

		// Scan this chunk. // 扫描此块。
		nfound := h.reclaimChunk(arenas, idx, pagesPerChunk)
		if nfound <= npage {
			npage -= nfound
		} else { // 回收的页面数比npage多，多的页面放到全局使用
			// Put spare pages toward global credit. // 将备用页面用于全局信用。
			atomic.Xadduintptr(&h.reclaimCredit, nfound-npage)
			npage = 0
		}
	}
	if locked { // 解锁
		unlock(&h.lock)
	}

	if trace.enabled {
		traceGCSweepDone()
	}
	releasem(mp) // 释放当前的m
}

// reclaimChunk sweeps unmarked spans that start at page indexes [pageIdx, pageIdx+n).
// It returns the number of pages returned to the heap.
//
// h.lock must be held and the caller must be non-preemptible. Note: h.lock may be
// temporarily unlocked and re-locked in order to do sweeping or if tracing is
// enabled.
//
// reclaimChunk扫描从页面索引[pageIdx，pageIdx+n)开始的未标记范围。
// 返回返回堆的页面数。
//
// 必须持有h.lock，并且调用者必须不可抢占。注意：h.lock可能会暂时解锁并重新锁定，以便进行扫描或启用了跟踪。
func (h *mheap) reclaimChunk(arenas []arenaIdx, pageIdx, n uintptr) uintptr {
	// The heap lock must be held because this accesses the
	// heapArena.spans arrays using potentially non-live pointers.
	// In particular, if a span were freed and merged concurrently
	// with this probing heapArena.spans, it would be possible to
	// observe arbitrary, stale span pointers.
	//
	// 必须持有堆的锁，因为这会使用潜在的非活动指针访问heapArena.spans数组。
    // 尤其是，如果释放了一个跨度并与此探测heapArena.spans并发合并，则可以观察到任意的，陈旧的范围指针。
	n0 := n
	var nFreed uintptr
	sg := h.sweepgen
	for n > 0 {
		ai := arenas[pageIdx/pagesPerArena]
		ha := h.arenas[ai.l1()][ai.l2()]

		// Get a chunk of the bitmap to work on. // 获取一部分位图以进行处理。
		arenaPage := uint(pageIdx % pagesPerArena)
		inUse := ha.pageInUse[arenaPage/8:]
		marked := ha.pageMarks[arenaPage/8:]
		if uintptr(len(inUse)) > n/8 {
			inUse = inUse[:n/8]
			marked = marked[:n/8]
		}

		// Scan this bitmap chunk for spans that are in-use
		// but have no marked objects on them.
		for i := range inUse {
			inUseUnmarked := atomic.Load8(&inUse[i]) &^ marked[i] // 这个值是最初记录
			if inUseUnmarked == 0 { // 表示整个堆竞技场没有被使用
				continue
			}

			for j := uint(0); j < 8; j++ {
				if inUseUnmarked&(1<<j) != 0 {
					s := ha.spans[arenaPage+uint(i)*8+j]
					if atomic.Load(&s.sweepgen) == sg-2 && atomic.Cas(&s.sweepgen, sg-2, sg-1) {
						npages := s.npages
						unlock(&h.lock)
						if s.sweep(false) { // 进行扫描
							nFreed += npages // 扫描成功增加计数
						}
						lock(&h.lock)
						// Reload inUse. It's possible nearby
						// spans were freed when we dropped the
						// lock and we don't want to get stale
						// pointers from the spans array.
						// 重新加载inUse。当我们放开锁并且我们不想从spans数组中获取过时的指针时，可能释放了附近的span。
						// NOTE: 如果inUse[i]和marked[i]值相同，&^操作后，inUseUnmarked会变为0
						inUseUnmarked = atomic.Load8(&inUse[i]) &^ marked[i]
					}
				}
			}
		}

		// Advance. // 每次前进8
		pageIdx += uintptr(len(inUse) * 8)
		n -= uintptr(len(inUse) * 8)
	}
	if trace.enabled {
		unlock(&h.lock)
		// Account for pages scanned but not reclaimed.
		traceGCSweepSpan((n0 - nFreed) * pageSize)
		lock(&h.lock)
	}
	return nFreed
}

// alloc allocates a new span of npage pages from the GC'd heap.
//
// spanclass indicates the span's size class and scannability.
//
// If needzero is true, the memory for the returned span will be zeroed.
//
// alloc从GC的堆中分配新的npage页跨度。
// spanclass指示跨度的大小类别和可扫描性。
// 如果needzero为true，则返回跨度的内存将清零。
func (h *mheap) alloc(npages uintptr, spanclass spanClass, needzero bool) *mspan {
	// Don't do any operations that lock the heap on the G stack.
	// It might trigger stack growth, and the stack growth code needs
	// to be able to allocate heap.
	//
	// 不要执行任何将堆锁定在G堆栈上的操作。
    // 它可能会触发堆栈增长，并且堆栈增长代码需要能够分配堆。
	var s *mspan
	systemstack(func() {
		// To prevent excessive heap growth, before allocating n pages
		// we need to sweep and reclaim at least n pages.
		// 为了防止堆过度增长，在分配n页之前，我们需要清除并回收至少n页。
		if h.sweepdone == 0 {
			h.reclaim(npages) // 释放页
		}
		s = h.allocSpan(npages, false, spanclass, &memstats.heap_inuse) // 分配置跨度
	})

	if s != nil {
		if needzero && s.needzero != 0 { // 需要清零
			memclrNoHeapPointers(unsafe.Pointer(s.base()), s.npages<<_PageShift)
		}
		s.needzero = 0
	}
	return s
}

// allocManual allocates a manually-managed span of npage pages.
// allocManual returns nil if allocation fails.
//
// allocManual adds the bytes used to *stat, which should be a
// memstats in-use field. Unlike allocations in the GC'd heap, the
// allocation does *not* count toward heap_inuse or heap_sys.
//
// The memory backing the returned span may not be zeroed if
// span.needzero is set.
//
// allocManual must be called on the system stack because it may
// acquire the heap lock via allocSpan. See mheap for details.
//
// allocManual分配npage页的手动管理范围。
// 如果分配失败，则allocManual返回nil。
//
// allocManual用于将字节添加到*stat中，该字节应该是memstats使用中的字段。与GC堆中的分配不同，该分配*不*计入heap_inuse或heap_sys。
//
// 如果设置了span.needzero，则支持返回的跨度内存可能不会为零。
//
// 必须在系统堆栈上调用allocManual，因为它可能通过allocSpan获取堆锁。有关详细信息，请参见mheap。
//go:systemstack
func (h *mheap) allocManual(npages uintptr, stat *uint64) *mspan {
	return h.allocSpan(npages, true, 0, stat)
}

// setSpans modifies the span map so [spanOf(base), spanOf(base+npage*pageSize))
// is s.
// setSpans修改跨度映射，因此[spanOf(base)，spanOf(base+npage*pageSize))为s。
func (h *mheap) setSpans(base, npage uintptr, s *mspan) {
	p := base / pageSize
	ai := arenaIndex(base)
	ha := h.arenas[ai.l1()][ai.l2()]
	for n := uintptr(0); n < npage; n++ {
		i := (p + n) % pagesPerArena
		if i == 0 {
			ai = arenaIndex(base + n*pageSize)
			ha = h.arenas[ai.l1()][ai.l2()]
		}
		ha.spans[i] = s
	}
}

// allocNeedsZero checks if the region of address space [base, base+npage*pageSize),
// assumed to be allocated, needs to be zeroed, updating heap arena metadata for
// future allocations.
//
// This must be called each time pages are allocated from the heap, even if the page
// allocator can otherwise prove the memory it's allocating is already zero because
// they're fresh from the operating system. It updates heapArena metadata that is
// critical for future page allocations.
//
// There are no locking constraints on this method.
//
// allocNeedsZero检查假定已分配的地址空间[base，base+npage*pageSize)的区域是否需要清零，更新堆舞台元数据以供将来分配。
//
// 每次从堆分配页面时，都必须调用此方法，即使页面分配器可以以其他方式证明其分配的内存已经为零，因为它们是从操作系统中获取的。
// 它将更新对于将来页面分配至关重要的heapArena元数据。
//
// 此方法没有锁定约束。
func (h *mheap) allocNeedsZero(base, npage uintptr) (needZero bool) {
	for npage > 0 {
		ai := arenaIndex(base)
		ha := h.arenas[ai.l1()][ai.l2()]

		zeroedBase := atomic.Loaduintptr(&ha.zeroedBase)
		arenaBase := base % heapArenaBytes
		if arenaBase < zeroedBase {
			// We extended into the non-zeroed part of the
			// arena, so this region needs to be zeroed before use.
			//
			// zeroedBase is monotonically increasing, so if we see this now then
			// we can be sure we need to zero this memory region.
			//
			// We still need to update zeroedBase for this arena, and
			// potentially more arenas.
			//
			// 我们扩展到了竞技场的非清零部分，因此在使用之前需要将该区域清零。
            //
            // zeroedBase正在单调增加，因此，如果现在看到此信息，则可以确定需要将该内存区域清零。
            //
            // 我们仍然需要为此竞技场以及可能更多的竞技场更新zeroedBase。
			needZero = true
		}
		// We may observe arenaBase > zeroedBase if we're racing with one or more
		// allocations which are acquiring memory directly before us in the address
		// space. But, because we know no one else is acquiring *this* memory, it's
		// still safe to not zero.
		//
		// 在我们进入地址空间之前，如果我们正在与一个或多个分配直接竞争获得内存的分配，我们可能会观察到arenaBase> zeroedBase。
		// 但是，因为我们知道没有其他人正在获取此内存，所以不为零仍然是安全的。

		// Compute how far into the arena we extend into, capped
		// at heapArenaBytes.
		// 计算我们延伸到竞技场的距离，以heapArenaBytes为上限。
		arenaLimit := arenaBase + npage*pageSize
		if arenaLimit > heapArenaBytes {
			arenaLimit = heapArenaBytes
		}
		// Increase ha.zeroedBase so it's >= arenaLimit.
		// We may be racing with other updates.
		//增加ha.zeroedBase，使其 >= arenaLimit。我们可能正在与其他更新竞争。
		for arenaLimit > zeroedBase {
			if atomic.Casuintptr(&ha.zeroedBase, zeroedBase, arenaLimit) {
				break
			}
			zeroedBase = atomic.Loaduintptr(&ha.zeroedBase)
			// Sanity check zeroedBase. // 健全性检查zeroedBase。
			if zeroedBase <= arenaLimit && zeroedBase > arenaBase {
				// The zeroedBase moved into the space we were trying to
				// claim. That's very bad, and indicates someone allocated
				// the same region we did.
				// zeroedBase移入了我们试图声明的空间。这很糟糕，表示有人分配了与我们相同的区域。
				throw("potentially overlapping in-use allocations detected")
			}
		}

		// Move base forward and subtract from npage to move into
		// the next arena, or finish.
		//向前移动base并从npage减去值以进入下一个区域，或结束。
		base += arenaLimit - arenaBase
		npage -= (arenaLimit - arenaBase) / pageSize
	}
	return
}

// tryAllocMSpan attempts to allocate an mspan object from
// the P-local cache, but may fail.
//
// h need not be locked.
//
// This caller must ensure that its P won't change underneath
// it during this function. Currently to ensure that we enforce
// that the function is run on the system stack, because that's
// the only place it is used now. In the future, this requirement
// may be relaxed if its use is necessary elsewhere.
//
// tryAllocMSpan尝试从P本地缓存分配mspan对象，但可能会失败。
//
// h不需要锁定。
//
// 此调用者必须确保在此函数期间，其P不会在其下方更改。当前，为了确保我们强制该功能在系统堆栈上运行，因为这是现在唯一使用它的地方。
// 将来，如果有必要在其他地方使用它，则可以放宽此要求。
//go:systemstack
func (h *mheap) tryAllocMSpan() *mspan {
	pp := getg().m.p.ptr()
	// If we don't have a p or the cache is empty, we can't do
	// anything here.
	// 如果我们没有p或缓存为空，那么我们将无法在此处执行任何操作。
	if pp == nil || pp.mspancache.len == 0 {
		return nil
	}
	// Pull off the last entry in the cache.
	// 提取缓存中的最后一个条目。
	s := pp.mspancache.buf[pp.mspancache.len-1]
	pp.mspancache.len--
	return s
}

// allocMSpanLocked allocates an mspan object.
//
// h must be locked.
//
// allocMSpanLocked must be called on the system stack because
// its caller holds the heap lock. See mheap for details.
// Running on the system stack also ensures that we won't
// switch Ps during this function. See tryAllocMSpan for details.
//
// allocMSpanLocked分配一个mspan对象。
//
// h必须被锁定。
//
// 必须在系统堆栈上调用allocMSpanLocked，因为它的调用方持有堆锁。有关详细信息，请参见mheap。
// 在系统堆栈上运行还可以确保我们在此功能期间不会切换Ps。有关详细信息，请参见tryAllocMSpan。
//go:systemstack
func (h *mheap) allocMSpanLocked() *mspan {
	pp := getg().m.p.ptr()
	if pp == nil {
		// We don't have a p so just do the normal thing.
		// 我们没有p，因此只需执行正常操作即可。
		return (*mspan)(h.spanalloc.alloc())
	}
	// Refill the cache if necessary.
	// 如有必要，请重新填充缓存。填充一半
	if pp.mspancache.len == 0 {
		const refillCount = len(pp.mspancache.buf) / 2
		for i := 0; i < refillCount; i++ {
			pp.mspancache.buf[i] = (*mspan)(h.spanalloc.alloc())
		}
		pp.mspancache.len = refillCount
	}
	// Pull off the last entry in the cache.
	// 提取缓存中的最后一个条目。做为结果返回
	s := pp.mspancache.buf[pp.mspancache.len-1]
	pp.mspancache.len--
	return s
}

// freeMSpanLocked free an mspan object.
//
// h must be locked.
//
// freeMSpanLocked must be called on the system stack because
// its caller holds the heap lock. See mheap for details.
// Running on the system stack also ensures that we won't
// switch Ps during this function. See tryAllocMSpan for details.
//
// freeMSpanLocked释放mspan对象。
//
// h必须被锁定。
//
// 必须在系统堆栈上调用freeMSpanLocked，因为其调用方拥有堆锁。有关详细信息，请参见mheap。
// 在系统堆栈上运行还可以确保我们在此功能期间不会切换Ps。有关详细信息，请参见tryAllocMSpan。
//go:systemstack
func (h *mheap) freeMSpanLocked(s *mspan) {
	pp := getg().m.p.ptr()
	// First try to free the mspan directly to the cache.
	// 首先尝试将mspan直接释放到缓存中。
	if pp != nil && pp.mspancache.len < len(pp.mspancache.buf) {
		pp.mspancache.buf[pp.mspancache.len] = s
		pp.mspancache.len++
		return
	}
	// Failing that (or if we don't have a p), just free it to
	// the heap.
	// 失败（或者如果我们没有p的话），只需将其释放到堆中即可。
	h.spanalloc.free(unsafe.Pointer(s))
}

// allocSpan allocates an mspan which owns npages worth of memory.
//
// If manual == false, allocSpan allocates a heap span of class spanclass
// and updates heap accounting. If manual == true, allocSpan allocates a
// manually-managed span (spanclass is ignored), and the caller is
// responsible for any accounting related to its use of the span. Either
// way, allocSpan will atomically add the bytes in the newly allocated
// span to *sysStat.
//
// The returned span is fully initialized.
//
// h must not be locked.
//
// allocSpan must be called on the system stack both because it acquires
// the heap lock and because it must block GC transitions.
//
// allocSpan分配一个拥有npages内存的mspan。
//
// 如果manual == false，则allocSpan分配class spanclass类别的堆跨度并更新堆记数。
// 如果manual == true，则allocSpan分配一个手动管理的跨度（spanclass被忽略），
// 并且调用方负责与其使用跨度有关的任何记帐。无论哪种方式，allocSpan都会自动将新分配的跨度中
// 的字节添加到*sysStat中。
//
// 返回的跨度已完全初始化。
//
// h不能被锁定。
//
// 必须在系统堆栈上调用allocSpan，这是因为它获取了堆锁并且因为它必须阻止GC转换。
//go:systemstack
func (h *mheap) allocSpan(npages uintptr, manual bool, spanclass spanClass, sysStat *uint64) (s *mspan) {
	// Function-global state. // 函数全局状态。
	gp := getg()
	base, scav := uintptr(0), uintptr(0)

	// If the allocation is small enough, try the page cache!
	// 如果分配足够小，请尝试页面缓存！
	pp := gp.m.p.ptr()
	if pp != nil && npages < pageCachePages/4 {
		c := &pp.pcache

		// If the cache is empty, refill it.
		// 如果缓存为空，请重新填充。
		if c.empty() {
			lock(&h.lock)
			*c = h.pages.allocToCache()
			unlock(&h.lock)
		}

		// Try to allocate from the cache.
		// 尝试从缓存中分配。
		base, scav = c.alloc(npages)
		if base != 0 {
			s = h.tryAllocMSpan()

			if s != nil && gcBlackenEnabled == 0 && (manual || spanclass.sizeclass() != 0) {
				goto HaveSpan
			}
			// We're either running duing GC, failed to acquire a mspan,
			// or the allocation is for a large object. This means we
			// have to lock the heap and do a bunch of extra work,
			// so go down the HaveBaseLocked path.
			//
			// We must do this during GC to avoid skew with heap_scan
			// since we flush mcache stats whenever we lock.
			//
			// TODO(mknyszek): It would be nice to not have to
			// lock the heap if it's a large allocation, but
			// it's fine for now. The critical section here is
			// short and large object allocations are relatively
			// infrequent.
			//
			// 我们正在运行GC，或者无法获得mspan，或者分配是针对大型对象的。
			// 这意味着我们必须锁定堆并做很多额外的工作，所以走在HaveBaseLocked路径上。
            //
            // 我们必须在GC期间执行此操作，以免使用heap_scan产生偏差，因为只要锁定就刷新mcache统计信息。
            //
            // TODO（mknyszek）：如果分配量很大，不必锁定堆会很好，但是现在很不错。
            // 这里的临界区是简短的，大型对象的分配相对较少。
		}
	}

	// For one reason or another, we couldn't get the
	// whole job done without the heap lock.
	// 由于某种原因，没有堆锁就无法完成整个工作。
	lock(&h.lock)

	if base == 0 {
		// Try to acquire a base address.
		// 尝试获取基址。
		base, scav = h.pages.alloc(npages)
		if base == 0 { // 没有基址就尝试扩展页
			if !h.grow(npages) {
				unlock(&h.lock) // 扩展失败，解锁返回nil
				return nil
			}
			base, scav = h.pages.alloc(npages) // 再次尝试获取基址。
			if base == 0 { // 没有就报错
				throw("grew heap, but no adequate free space found")
			}
		}
	}
	if s == nil {
		// We failed to get an mspan earlier, so grab
		// one now that we have the heap lock.
		// 我们早期获得mspan失败，因此现在有了堆锁，现在就抓取一个作为结果。
		s = h.allocMSpanLocked()
	}
	if !manual {
		// This is a heap span, so we should do some additional accounting
		// which may only be done with the heap locked.
		// 这是一个堆跨度，因此我们应该做一些额外的记录，只有在锁定堆时才能完成。

		// Transfer stats from mcache to global.
		// 将统计信息从mcache传输到全局。
		memstats.heap_scan += uint64(gp.m.mcache.local_scan)
		gp.m.mcache.local_scan = 0
		memstats.tinyallocs += uint64(gp.m.mcache.local_tinyallocs)
		gp.m.mcache.local_tinyallocs = 0

		// Do some additional accounting if it's a large allocation.
		// 如果分配量很大，请执行一些其他统计。
		if spanclass.sizeclass() == 0 {
			mheap_.largealloc += uint64(npages * pageSize)
			mheap_.nlargealloc++
			atomic.Xadd64(&memstats.heap_live, int64(npages*pageSize))
		}

		// Either heap_live or heap_scan could have been updated.
		// heap_live或heap_scan可能已被更新。
		if gcBlackenEnabled != 0 {
			gcController.revise()
		}
	}
	unlock(&h.lock)

HaveSpan:
	// At this point, both s != nil and base != 0, and the heap
	// lock is no longer held. Initialize the span.
	// 此时，s != nil并且base != 0，并且不再持有堆锁。初始化跨度。
	s.init(base, npages)
	if h.allocNeedsZero(base, npages) {
		s.needzero = 1
	}
	nbytes := npages * pageSize
	if manual {
		s.manualFreeList = 0
		s.nelems = 0
		s.limit = s.base() + s.npages*pageSize
		// Manually managed memory doesn't count toward heap_sys.
		// 手动管理的内存不计入heap_sys。
		mSysStatDec(&memstats.heap_sys, s.npages*pageSize)
		s.state.set(mSpanManual)
	} else {
		// We must set span properties before the span is published anywhere
		// since we're not holding the heap lock.
		// 由于未持有堆锁，因此必须在将跨度发布到任何地方之前设置跨度属性。
		s.spanclass = spanclass
		if sizeclass := spanclass.sizeclass(); sizeclass == 0 {
			s.elemsize = nbytes
			s.nelems = 1

			s.divShift = 0
			s.divMul = 0
			s.divShift2 = 0
			s.baseMask = 0
		} else {
			s.elemsize = uintptr(class_to_size[sizeclass])
			s.nelems = nbytes / s.elemsize

			m := &class_to_divmagic[sizeclass]
			s.divShift = m.shift
			s.divMul = m.mul
			s.divShift2 = m.shift2
			s.baseMask = m.baseMask
		}

		// Initialize mark and allocation structures.
		// 初始化标记和分配结构。
		s.freeindex = 0
		s.allocCache = ^uint64(0) // all 1s indicating all free. // 所有1表示全部空闲。
		s.gcmarkBits = newMarkBits(s.nelems)
		s.allocBits = newAllocBits(s.nelems)

		// It's safe to access h.sweepgen without the heap lock because it's
		// only ever updated with the world stopped and we run on the
		// systemstack which blocks a STW transition.
		// 在没有堆锁的情况下访问h.sweepgen是安全的，因为只有在全局停机的情况下才进行更新，
		// 并且我们在阻止STW转换的系统堆栈上运行。
		atomic.Store(&s.sweepgen, h.sweepgen)

		// Now that the span is filled in, set its state. This
		// is a publication barrier for the other fields in
		// the span. While valid pointers into this span
		// should never be visible until the span is returned,
		// if the garbage collector finds an invalid pointer,
		// access to the span may race with initialization of
		// the span. We resolve this race by atomically
		// setting the state after the span is fully
		// initialized, and atomically checking the state in
		// any situation where a pointer is suspect.
		// 现在跨度已填充，其状态已设置。这是跨度中其他字段的发布障碍。
		// 尽管在返回该跨度之前，将永远不会看到进入该跨度的有效指针，
		// 但是，如果垃圾收集器发现了无效的指针，则对该范围的访问可能会与该范围的初始化竞争。
		// 我们通过在完全初始化跨度之后自动设置状态，并在任何可疑指针的情况下自动检查状态，
		// 来解决此竞争。
		s.state.set(mSpanInUse)
	}

	// Commit and account for any scavenged memory that the span now owns.
	// 提交并考虑跨度现在拥有的所有清理内存。
	if scav != 0 {
		// sysUsed all the pages that are actually available
		// in the span since some of them might be scavenged.
		// sysUsed 跨度中实际可用的所有页面，因为其中的某些页面可能会被清除。
		sysUsed(unsafe.Pointer(base), nbytes)
		mSysStatDec(&memstats.heap_released, scav)
	}
	// Update stats.
	// 更新统计
	mSysStatInc(sysStat, nbytes)
	mSysStatDec(&memstats.heap_idle, nbytes)

	// Publish the span in various locations.
	// 在不同位置发布跨度。

	// This is safe to call without the lock held because the slots
	// related to this span will only every be read or modified by
	// this thread until pointers into the span are published or
	// pageInUse is updated.
	// 这是安全的，无需持有锁定，因为与此跨度相关的插槽将仅由该线程读取或修改，
	// 直到该跨度的指针被发布或pageInUse被更新为止。
	h.setSpans(s.base(), npages, s)

	if !manual {
		// Add to swept in-use list.
		//
		// This publishes the span to root marking.
		//
		// h.sweepgen is guaranteed to only change during STW,
		// and preemption is disabled in the page allocator.
		// 添加到已清除的使用中列表。
        //
        // 这会将跨度发布到根标记。
        //
        // 确保h.sweepgen仅在STW期间更改，并且在页面分配器中禁用了抢占。
		h.sweepSpans[h.sweepgen/2%2].push(s)

		// Mark in-use span in arena page bitmap.
		//
		// This publishes the span to the page sweeper, so
		// it's imperative that the span be completely initialized
		// prior to this line.
		// 在竞技场页面位图中标记使用范围。
        //
        // 这会将范围发布到页面清除程序，因此必须在此行之前完全初始化跨度。
		arena, pageIdx, pageMask := pageIndexOf(s.base())
		atomic.Or8(&arena.pageInUse[pageIdx], pageMask)

		// Update related page sweeper stats.
		// 更新相关的页面清除器统计信息。
		atomic.Xadd64(&h.pagesInUse, int64(npages))

		if trace.enabled {
			// Trace that a heap alloc occurred.
			traceHeapAlloc()
		}
	}
	return s
}

// Try to add at least npage pages of memory to the heap,
// returning whether it worked.
//
// h must be locked.
//
// 尝试至少将npage页的内存添加到堆中，并返回其是否起作用。
// h必须被锁定。
func (h *mheap) grow(npage uintptr) bool {
	// We must grow the heap in whole palloc chunks.
	// 我们必须在整个palloc块中增加堆。
	ask := alignUp(npage, pallocChunkPages) * pageSize

	totalGrowth := uintptr(0)
	nBase := alignUp(h.curArena.base+ask, physPageSize)
	if nBase > h.curArena.end {
		// Not enough room in the current arena. Allocate more
		// arena space. This may not be contiguous with the
		// current arena, so we have to request the full ask.
		// 当前竞技场上没有足够的空间。分配更多的竞技场空间。
		// 这可能与当前的领域不连续有关，因此我们必须要求完整的ask内存。
		av, asize := h.sysAlloc(ask)
		if av == nil {
			print("runtime: out of memory: cannot allocate ", ask, "-byte block (", memstats.heap_sys, " in use)\n")
			return false
		}

		if uintptr(av) == h.curArena.end {
			// The new space is contiguous with the old
			// space, so just extend the current space.
			// 新空间与旧空间相邻，因此只需扩展当前空间即可。
			h.curArena.end = uintptr(av) + asize
		} else {
			// The new space is discontiguous. Track what
			// remains of the current space and switch to
			// the new space. This should be rare.
			// 新空间不连续。跟踪剩余的当前空间并切换到新空间。这应该很少见。
			if size := h.curArena.end - h.curArena.base; size != 0 {
				h.pages.grow(h.curArena.base, size)
				totalGrowth += size
			}
			// Switch to the new space.
			// 切换到新空间。
			h.curArena.base = uintptr(av)
			h.curArena.end = uintptr(av) + asize
		}

		// The memory just allocated counts as both released
		// and idle, even though it's not yet backed by spans.
		//
		// The allocation is always aligned to the heap arena
		// size which is always > physPageSize, so its safe to
		// just add directly to heap_released.
		//
		// 刚刚分配的内存即使没有跨度支持也算为已释放和空闲。
        //
        // 分配始终与的堆竞技场大小对齐，堆竞技场大小总是>physPageSize
        // 因此可以安全地将其直接添加到heap_released中。
		mSysStatInc(&memstats.heap_released, asize)
		mSysStatInc(&memstats.heap_idle, asize)

		// Recalculate nBase
		// 重新计算nBase
		nBase = alignUp(h.curArena.base+ask, physPageSize)
	}

	// Grow into the current arena.
	// 进入当前的arena。
	v := h.curArena.base
	h.curArena.base = nBase
	h.pages.grow(v, nBase-v)
	totalGrowth += nBase - v

	// We just caused a heap growth, so scavenge down what will soon be used.
	// By scavenging inline we deal with the failure to allocate out of
	// memory fragments by scavenging the memory fragments that are least
	// likely to be re-used.
	// 我们只是导致了堆的增长，因此请清除即将使用的内容。
    // 通过清除内联，我们通过清除最可能被重用的内存片段来处理无法分配内存碎片的问题。
	if retained := heapRetained(); retained+uint64(totalGrowth) > h.scavengeGoal {
		todo := totalGrowth
		if overage := uintptr(retained + uint64(totalGrowth) - h.scavengeGoal); todo > overage {
			todo = overage
		}
		h.pages.scavenge(todo, true)
	}
	return true
}

// Free the span back into the heap.
// 将跨度释放回堆中。
func (h *mheap) freeSpan(s *mspan) {
	systemstack(func() {
		mp := getg().m
		lock(&h.lock)
		memstats.heap_scan += uint64(mp.mcache.local_scan)
		mp.mcache.local_scan = 0
		memstats.tinyallocs += uint64(mp.mcache.local_tinyallocs)
		mp.mcache.local_tinyallocs = 0
		if msanenabled {
			// Tell msan that this entire span is no longer in use.
			// 告诉msan这整个跨度已不再使用。
			base := unsafe.Pointer(s.base())
			bytes := s.npages << _PageShift
			msanfree(base, bytes)
		}
		if gcBlackenEnabled != 0 {
			// heap_scan changed.
			// heap_scan已更改。
			gcController.revise()
		}
		h.freeSpanLocked(s, true, true)
		unlock(&h.lock)
	})
}

// freeManual frees a manually-managed span returned by allocManual.
// stat must be the same as the stat passed to the allocManual that
// allocated s.
//
// This must only be called when gcphase == _GCoff. See mSpanState for
// an explanation.
//
// freeManual must be called on the system stack because it acquires
// the heap lock. See mheap for details.
//
// freeManual释放由allocManual返回的手动管理跨度。stat必须与传递给分配s的allocManual的stat相同。
//
// 仅当gcphase == _GCoff时才必须调用此函数。有关说明，请参见mSpanState。
//
// 必须在系统堆栈上调用freeManual，因为它获取了堆锁。有关详细信息，请参见mheap。
//
//go:systemstack
func (h *mheap) freeManual(s *mspan, stat *uint64) {
	s.needzero = 1
	lock(&h.lock)
	mSysStatDec(stat, s.npages*pageSize)
	mSysStatInc(&memstats.heap_sys, s.npages*pageSize)
	h.freeSpanLocked(s, false, true)
	unlock(&h.lock)
}

func (h *mheap) freeSpanLocked(s *mspan, acctinuse, acctidle bool) {
	switch s.state.get() {
	case mSpanManual:
		if s.allocCount != 0 {
			throw("mheap.freeSpanLocked - invalid stack free")
		}
	case mSpanInUse:
		if s.allocCount != 0 || s.sweepgen != h.sweepgen {
			print("mheap.freeSpanLocked - span ", s, " ptr ", hex(s.base()), " allocCount ", s.allocCount, " sweepgen ", s.sweepgen, "/", h.sweepgen, "\n")
			throw("mheap.freeSpanLocked - invalid free")
		}
		atomic.Xadd64(&h.pagesInUse, -int64(s.npages))

		// Clear in-use bit in arena page bitmap.
		// 清除arena页面位图中的使用中的位标记。
		arena, pageIdx, pageMask := pageIndexOf(s.base())
		atomic.And8(&arena.pageInUse[pageIdx], ^pageMask)
	default:
		throw("mheap.freeSpanLocked - invalid span state")
	}

	if acctinuse {
		mSysStatDec(&memstats.heap_inuse, s.npages*pageSize)
	}
	if acctidle {
		mSysStatInc(&memstats.heap_idle, s.npages*pageSize)
	}

	// Mark the space as free.
	//将空间标记为空闲。
	h.pages.free(s.base(), s.npages)

	// Free the span structure. We no longer have a use for it.
	// 释放跨度结构。我们不再使用它。
	s.state.set(mSpanDead)
	h.freeMSpanLocked(s)
}

// scavengeAll visits each node in the free treap and scavenges the
// treapNode's span. It then removes the scavenged span from
// unscav and adds it into scav before continuing.
// scavengeAll访问空闲treap中的每个节点并清除treapNode的跨度。
// 然后，它从unscav中删除清除的跨度，然后将其添加到scav中，然后再继续。
func (h *mheap) scavengeAll() {
	// Disallow malloc or panic while holding the heap lock. We do
	// this here because this is a non-mallocgc entry-point to
	// the mheap API.
	// 持有堆锁时禁止malloc或panic。我们在这里这样做是因为这是mheap API的非mallocgc入口点。
	gp := getg()
	gp.m.mallocing++
	lock(&h.lock)
	// Reset the scavenger address so we have access to the whole heap.
	// 重置清除程序地址，以便我们可以访问整个堆。
	h.pages.resetScavengeAddr()
	released := h.pages.scavenge(^uintptr(0), true)
	unlock(&h.lock)
	gp.m.mallocing--

	if debug.scavtrace > 0 {
		printScavTrace(released, true)
	}
}

//go:linkname runtime_debug_freeOSMemory runtime/debug.freeOSMemory
func runtime_debug_freeOSMemory() {
	GC()
	systemstack(func() { mheap_.scavengeAll() })
}

// Initialize a new span with the given start and npages.
// 使用给定的start和npage初始化一个新的跨度。
func (span *mspan) init(base uintptr, npages uintptr) {
	// span is *not* zeroed.
	span.next = nil
	span.prev = nil
	span.list = nil
	span.startAddr = base
	span.npages = npages
	span.allocCount = 0
	span.spanclass = 0
	span.elemsize = 0
	span.speciallock.key = 0
	span.specials = nil
	span.needzero = 0
	span.freeindex = 0
	span.allocBits = nil
	span.gcmarkBits = nil
	span.state.set(mSpanDead)
}

func (span *mspan) inList() bool {
	return span.list != nil
}

// Initialize an empty doubly-linked list.
// 初始化一个空的双向链接列表
func (list *mSpanList) init() {
	list.first = nil
	list.last = nil
}

func (list *mSpanList) remove(span *mspan) {
	if span.list != list {
		print("runtime: failed mSpanList.remove span.npages=", span.npages,
			" span=", span, " prev=", span.prev, " span.list=", span.list, " list=", list, "\n")
		throw("mSpanList.remove")
	}
	if list.first == span {
		list.first = span.next
	} else {
		span.prev.next = span.next
	}
	if list.last == span {
		list.last = span.prev
	} else {
		span.next.prev = span.prev
	}
	span.next = nil
	span.prev = nil
	span.list = nil
}

func (list *mSpanList) isEmpty() bool {
	return list.first == nil
}

func (list *mSpanList) insert(span *mspan) {
	if span.next != nil || span.prev != nil || span.list != nil {
		println("runtime: failed mSpanList.insert", span, span.next, span.prev, span.list)
		throw("mSpanList.insert")
	}
	span.next = list.first
	if list.first != nil {
		// The list contains at least one span; link it in.
		// The last span in the list doesn't change.
		list.first.prev = span
	} else {
		// The list contains no spans, so this is also the last span.
		list.last = span
	}
	list.first = span
	span.list = list
}

// 插入到最后
func (list *mSpanList) insertBack(span *mspan) {
	if span.next != nil || span.prev != nil || span.list != nil {
		println("runtime: failed mSpanList.insertBack", span, span.next, span.prev, span.list)
		throw("mSpanList.insertBack")
	}
	span.prev = list.last
	if list.last != nil {
		// The list contains at least one span.
		list.last.next = span
	} else {
		// The list contains no spans, so this is also the first span.
		list.first = span
	}
	list.last = span
	span.list = list
}

// takeAll removes all spans from other and inserts them at the front
// of list.
// takeAll从所有其他跨度中删除所有跨度并将其插入列表的开头。
func (list *mSpanList) takeAll(other *mSpanList) {
	if other.isEmpty() {
		return
	}

	// Reparent everything in other to list.
	// 将其他所有内容都列出来。
	for s := other.first; s != nil; s = s.next {
		s.list = list
	}

	// Concatenate the lists.
	// 连接列表。
	if list.isEmpty() {
		*list = *other
	} else {
		// Neither list is empty. Put other before list.
		//两个列表都不为空。将other放在list前。
		other.last.next = list.first
		list.first.prev = other.last
		list.first = other.first
	}

	other.first, other.last = nil, nil
}

const (
	_KindSpecialFinalizer = 1
	_KindSpecialProfile   = 2
	// Note: The finalizer special must be first because if we're freeing
	// an object, a finalizer special will cause the freeing operation
	// to abort, and we want to keep the other special records around
	// if that happens.
	// 注意：finalizer special必须是第一个，因为如果我们要释放对象，则finalizer special将导致释放操作中止，
	// 并且如果发生这种情况，我们希望保留其他特殊记录。
)

//go:notinheap
type special struct {
	next   *special // linked list in span // 跨度中的链表
	offset uint16   // span offset of object // 对象的跨度偏移
	kind   byte     // kind of special // special类型
}

// Adds the special record s to the list of special records for
// the object p. All fields of s should be filled in except for
// offset & next, which this routine will fill in.
// Returns true if the special was successfully added, false otherwise.
// (The add will fail only if a record with the same p and s->kind
//  already exists.)
func addspecial(p unsafe.Pointer, s *special) bool {
	span := spanOfHeap(uintptr(p))
	if span == nil {
		throw("addspecial on invalid pointer")
	}

	// Ensure that the span is swept.
	// Sweeping accesses the specials list w/o locks, so we have
	// to synchronize with it. And it's just much safer.
	mp := acquirem()
	span.ensureSwept()

	offset := uintptr(p) - span.base()
	kind := s.kind

	lock(&span.speciallock)

	// Find splice point, check for existing record.
	t := &span.specials
	for {
		x := *t
		if x == nil {
			break
		}
		if offset == uintptr(x.offset) && kind == x.kind {
			unlock(&span.speciallock)
			releasem(mp)
			return false // already exists
		}
		if offset < uintptr(x.offset) || (offset == uintptr(x.offset) && kind < x.kind) {
			break
		}
		t = &x.next
	}

	// Splice in record, fill in offset.
	s.offset = uint16(offset)
	s.next = *t
	*t = s
	unlock(&span.speciallock)
	releasem(mp)

	return true
}

// Removes the Special record of the given kind for the object p.
// Returns the record if the record existed, nil otherwise.
// The caller must FixAlloc_Free the result.
func removespecial(p unsafe.Pointer, kind uint8) *special {
	span := spanOfHeap(uintptr(p))
	if span == nil {
		throw("removespecial on invalid pointer")
	}

	// Ensure that the span is swept.
	// Sweeping accesses the specials list w/o locks, so we have
	// to synchronize with it. And it's just much safer.
	mp := acquirem()
	span.ensureSwept()

	offset := uintptr(p) - span.base()

	lock(&span.speciallock)
	t := &span.specials
	for {
		s := *t
		if s == nil {
			break
		}
		// This function is used for finalizers only, so we don't check for
		// "interior" specials (p must be exactly equal to s->offset).
		if offset == uintptr(s.offset) && kind == s.kind {
			*t = s.next
			unlock(&span.speciallock)
			releasem(mp)
			return s
		}
		t = &s.next
	}
	unlock(&span.speciallock)
	releasem(mp)
	return nil
}

// The described object has a finalizer set for it.
//
// specialfinalizer is allocated from non-GC'd memory, so any heap
// pointers must be specially handled.
//
//go:notinheap
type specialfinalizer struct {
	special special
	fn      *funcval // May be a heap pointer.
	nret    uintptr
	fint    *_type   // May be a heap pointer, but always live.
	ot      *ptrtype // May be a heap pointer, but always live.
}

// Adds a finalizer to the object p. Returns true if it succeeded.
func addfinalizer(p unsafe.Pointer, f *funcval, nret uintptr, fint *_type, ot *ptrtype) bool {
	lock(&mheap_.speciallock)
	s := (*specialfinalizer)(mheap_.specialfinalizeralloc.alloc())
	unlock(&mheap_.speciallock)
	s.special.kind = _KindSpecialFinalizer
	s.fn = f
	s.nret = nret
	s.fint = fint
	s.ot = ot
	if addspecial(p, &s.special) {
		// This is responsible for maintaining the same
		// GC-related invariants as markrootSpans in any
		// situation where it's possible that markrootSpans
		// has already run but mark termination hasn't yet.
		if gcphase != _GCoff {
			base, _, _ := findObject(uintptr(p), 0, 0)
			mp := acquirem()
			gcw := &mp.p.ptr().gcw
			// Mark everything reachable from the object
			// so it's retained for the finalizer.
			scanobject(base, gcw)
			// Mark the finalizer itself, since the
			// special isn't part of the GC'd heap.
			scanblock(uintptr(unsafe.Pointer(&s.fn)), sys.PtrSize, &oneptrmask[0], gcw, nil)
			releasem(mp)
		}
		return true
	}

	// There was an old finalizer
	lock(&mheap_.speciallock)
	mheap_.specialfinalizeralloc.free(unsafe.Pointer(s))
	unlock(&mheap_.speciallock)
	return false
}

// Removes the finalizer (if any) from the object p.
func removefinalizer(p unsafe.Pointer) {
	s := (*specialfinalizer)(unsafe.Pointer(removespecial(p, _KindSpecialFinalizer)))
	if s == nil {
		return // there wasn't a finalizer to remove
	}
	lock(&mheap_.speciallock)
	mheap_.specialfinalizeralloc.free(unsafe.Pointer(s))
	unlock(&mheap_.speciallock)
}

// The described object is being heap profiled.
//
//go:notinheap
type specialprofile struct {
	special special
	b       *bucket
}

// Set the heap profile bucket associated with addr to b.
func setprofilebucket(p unsafe.Pointer, b *bucket) {
	lock(&mheap_.speciallock)
	s := (*specialprofile)(mheap_.specialprofilealloc.alloc())
	unlock(&mheap_.speciallock)
	s.special.kind = _KindSpecialProfile
	s.b = b
	if !addspecial(p, &s.special) {
		throw("setprofilebucket: profile already set")
	}
}

// Do whatever cleanup needs to be done to deallocate s. It has
// already been unlinked from the mspan specials list.
func freespecial(s *special, p unsafe.Pointer, size uintptr) {
	switch s.kind {
	case _KindSpecialFinalizer:
		sf := (*specialfinalizer)(unsafe.Pointer(s))
		queuefinalizer(p, sf.fn, sf.nret, sf.fint, sf.ot)
		lock(&mheap_.speciallock)
		mheap_.specialfinalizeralloc.free(unsafe.Pointer(sf))
		unlock(&mheap_.speciallock)
	case _KindSpecialProfile:
		sp := (*specialprofile)(unsafe.Pointer(s))
		mProf_Free(sp.b, size)
		lock(&mheap_.speciallock)
		mheap_.specialprofilealloc.free(unsafe.Pointer(sp))
		unlock(&mheap_.speciallock)
	default:
		throw("bad special kind")
		panic("not reached")
	}
}

// gcBits is an alloc/mark bitmap. This is always used as *gcBits.
//
//go:notinheap
type gcBits uint8

// bytep returns a pointer to the n'th byte of b.
func (b *gcBits) bytep(n uintptr) *uint8 {
	return addb((*uint8)(b), n)
}

// bitp returns a pointer to the byte containing bit n and a mask for
// selecting that bit from *bytep.
func (b *gcBits) bitp(n uintptr) (bytep *uint8, mask uint8) {
	return b.bytep(n / 8), 1 << (n % 8)
}

const gcBitsChunkBytes = uintptr(64 << 10)
const gcBitsHeaderBytes = unsafe.Sizeof(gcBitsHeader{})

type gcBitsHeader struct {
	free uintptr // free is the index into bits of the next free byte.
	next uintptr // *gcBits triggers recursive type bug. (issue 14620)
}

//go:notinheap
type gcBitsArena struct {
	// gcBitsHeader // side step recursive type bug (issue 14620) by including fields by hand.
	free uintptr // free is the index into bits of the next free byte; read/write atomically
	next *gcBitsArena
	bits [gcBitsChunkBytes - gcBitsHeaderBytes]gcBits
}

var gcBitsArenas struct {
	lock     mutex
	free     *gcBitsArena
	next     *gcBitsArena // Read atomically. Write atomically under lock.
	current  *gcBitsArena
	previous *gcBitsArena
}

// tryAlloc allocates from b or returns nil if b does not have enough room.
// This is safe to call concurrently.
func (b *gcBitsArena) tryAlloc(bytes uintptr) *gcBits {
	if b == nil || atomic.Loaduintptr(&b.free)+bytes > uintptr(len(b.bits)) {
		return nil
	}
	// Try to allocate from this block.
	end := atomic.Xadduintptr(&b.free, bytes)
	if end > uintptr(len(b.bits)) {
		return nil
	}
	// There was enough room.
	start := end - bytes
	return &b.bits[start]
}

// newMarkBits returns a pointer to 8 byte aligned bytes
// to be used for a span's mark bits.
func newMarkBits(nelems uintptr) *gcBits {
	blocksNeeded := uintptr((nelems + 63) / 64)
	bytesNeeded := blocksNeeded * 8

	// Try directly allocating from the current head arena.
	head := (*gcBitsArena)(atomic.Loadp(unsafe.Pointer(&gcBitsArenas.next)))
	if p := head.tryAlloc(bytesNeeded); p != nil {
		return p
	}

	// There's not enough room in the head arena. We may need to
	// allocate a new arena.
	lock(&gcBitsArenas.lock)
	// Try the head arena again, since it may have changed. Now
	// that we hold the lock, the list head can't change, but its
	// free position still can.
	if p := gcBitsArenas.next.tryAlloc(bytesNeeded); p != nil {
		unlock(&gcBitsArenas.lock)
		return p
	}

	// Allocate a new arena. This may temporarily drop the lock.
	fresh := newArenaMayUnlock()
	// If newArenaMayUnlock dropped the lock, another thread may
	// have put a fresh arena on the "next" list. Try allocating
	// from next again.
	if p := gcBitsArenas.next.tryAlloc(bytesNeeded); p != nil {
		// Put fresh back on the free list.
		// TODO: Mark it "already zeroed"
		fresh.next = gcBitsArenas.free
		gcBitsArenas.free = fresh
		unlock(&gcBitsArenas.lock)
		return p
	}

	// Allocate from the fresh arena. We haven't linked it in yet, so
	// this cannot race and is guaranteed to succeed.
	p := fresh.tryAlloc(bytesNeeded)
	if p == nil {
		throw("markBits overflow")
	}

	// Add the fresh arena to the "next" list.
	fresh.next = gcBitsArenas.next
	atomic.StorepNoWB(unsafe.Pointer(&gcBitsArenas.next), unsafe.Pointer(fresh))

	unlock(&gcBitsArenas.lock)
	return p
}

// newAllocBits returns a pointer to 8 byte aligned bytes
// to be used for this span's alloc bits.
// newAllocBits is used to provide newly initialized spans
// allocation bits. For spans not being initialized the
// mark bits are repurposed as allocation bits when
// the span is swept.
func newAllocBits(nelems uintptr) *gcBits {
	return newMarkBits(nelems)
}

// nextMarkBitArenaEpoch establishes a new epoch for the arenas
// holding the mark bits. The arenas are named relative to the
// current GC cycle which is demarcated by the call to finishweep_m.
//
// All current spans have been swept.
// During that sweep each span allocated room for its gcmarkBits in
// gcBitsArenas.next block. gcBitsArenas.next becomes the gcBitsArenas.current
// where the GC will mark objects and after each span is swept these bits
// will be used to allocate objects.
// gcBitsArenas.current becomes gcBitsArenas.previous where the span's
// gcAllocBits live until all the spans have been swept during this GC cycle.
// The span's sweep extinguishes all the references to gcBitsArenas.previous
// by pointing gcAllocBits into the gcBitsArenas.current.
// The gcBitsArenas.previous is released to the gcBitsArenas.free list.
func nextMarkBitArenaEpoch() {
	lock(&gcBitsArenas.lock)
	if gcBitsArenas.previous != nil {
		if gcBitsArenas.free == nil {
			gcBitsArenas.free = gcBitsArenas.previous
		} else {
			// Find end of previous arenas.
			last := gcBitsArenas.previous
			for last = gcBitsArenas.previous; last.next != nil; last = last.next {
			}
			last.next = gcBitsArenas.free
			gcBitsArenas.free = gcBitsArenas.previous
		}
	}
	gcBitsArenas.previous = gcBitsArenas.current
	gcBitsArenas.current = gcBitsArenas.next
	atomic.StorepNoWB(unsafe.Pointer(&gcBitsArenas.next), nil) // newMarkBits calls newArena when needed
	unlock(&gcBitsArenas.lock)
}

// newArenaMayUnlock allocates and zeroes a gcBits arena.
// The caller must hold gcBitsArena.lock. This may temporarily release it.
func newArenaMayUnlock() *gcBitsArena {
	var result *gcBitsArena
	if gcBitsArenas.free == nil {
		unlock(&gcBitsArenas.lock)
		result = (*gcBitsArena)(sysAlloc(gcBitsChunkBytes, &memstats.gc_sys))
		if result == nil {
			throw("runtime: cannot allocate memory")
		}
		lock(&gcBitsArenas.lock)
	} else {
		result = gcBitsArenas.free
		gcBitsArenas.free = gcBitsArenas.free.next
		memclrNoHeapPointers(unsafe.Pointer(result), gcBitsChunkBytes)
	}
	result.next = nil
	// If result.bits is not 8 byte aligned adjust index so
	// that &result.bits[result.free] is 8 byte aligned.
	if uintptr(unsafe.Offsetof(gcBitsArena{}.bits))&7 == 0 {
		result.free = 0
	} else {
		result.free = 8 - (uintptr(unsafe.Pointer(&result.bits[0])) & 7)
	}
	return result
}
```