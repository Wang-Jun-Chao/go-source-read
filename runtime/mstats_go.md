```go

// Memory statistics

package runtime

import (
	"runtime/internal/atomic"
	"runtime/internal/sys"
	"unsafe"
)

// Statistics.
// If you edit this structure, also edit type MemStats below.
// Their layouts must match exactly.
//
// For detailed descriptions see the documentation for MemStats.
// Fields that differ from MemStats are further documented here.
//
// Many of these fields are updated on the fly, while others are only
// updated when updatememstats is called.
//
// 统计信息。
// 如果您编辑此结构，请同时在下面编辑MemStats类型。它们的布局必须完全匹配。
//
// 有关详细说明，请参阅MemStats文档。与MemStats不同的字段将在此处进一步记录。
//
// 这些字段中的许多都是动态更新的，而其他字段仅在调用updatememstats时才更新。
// Question: 怎么动态更新？
type mstats struct {
	// General statistics.
	// 一般统计。
	alloc       uint64 // bytes allocated and not yet freed // 已分配但尚未释放的字节
	total_alloc uint64 // bytes allocated (even if freed) // 分配的字节（即使已释放）
	sys         uint64 // bytes obtained from system (should be sum of xxx_sys below, no locking, approximate) // 从系统获得的字节（应为以下xxx_sys的总和，无锁定，近似值）
	nlookup     uint64 // number of pointer lookups (unused) // 指针查找数（未使用）
	nmalloc     uint64 // number of mallocs // malloc的次数
	nfree       uint64 // number of frees // 释放的次数

	// Statistics about malloc heap.
	// Updated atomically, or with the world stopped.
	//
	// Like MemStats, heap_sys and heap_inuse do not count memory
	// in manually-managed spans.
    // 有关malloc堆的统计信息。
    // 原子更新，或者全局停顿。
    //
    // 与MemStats一样，heap_sys和heap_inuse不会在手动管理的范围内计算内存。
	heap_alloc    uint64 // bytes allocated and not yet freed (same as alloc above) // 已分配但尚未释放的字节（与上面的alloc相同）
	heap_sys      uint64 // virtual address space obtained from system for GC'd heap // 从系统获得的用于GC堆的虚拟地址空间
	heap_idle     uint64 // bytes in idle spans // 空闲跨度（span）中的字节
	heap_inuse    uint64 // bytes in mSpanInUse spans // mSpanInUse跨度（span）中的字节
	heap_released uint64 // bytes released to the os // 释放到操作系统的字节

	// heap_objects is not used by the runtime directly and instead
	// computed on the fly by updatememstats.
    // 运行时不直接使用heap_objects，而是由updatememstats动态计算。
	heap_objects uint64 // total number of allocated objects // 分配的对象总数

	// Statistics about allocation of low-level fixed-size structures.
	// Protected by FixAlloc locks.
    // 有关低级别固定大小结构分配的统计信息。
    // 受FixAlloc锁保护。
	stacks_inuse uint64 // bytes in manually-managed stack spans; updated atomically or during STW // 手动管理的堆栈跨度中的字节； 自动更新或在STW期间更新
	stacks_sys   uint64 // only counts newosproc0 stack in mstats; differs from MemStats.StackSys // 仅在mstats中计算newosproc0堆栈； 与MemStats.StackSys不同
	mspan_inuse  uint64 // mspan structures // mspan结构
	mspan_sys    uint64
	mcache_inuse uint64 // mcache structures // mcache结构
	mcache_sys   uint64
	buckhash_sys uint64 // profiling bucket hash table // 分析存储桶哈希表
	gc_sys       uint64 // updated atomically or during STW // 自动更新或在STW期间更新
	other_sys    uint64 // updated atomically or during STW // 自动更新或在STW期间更新

	// Statistics about garbage collector.
	// Protected by mheap or stopping the world during GC.
    // 有关垃圾收集器的统计信息。
    //在GC期间受mheap保护或全局停机。
	next_gc         uint64 // goal heap_live for when next GC ends; ^0 if disabled // 下一个GC何时结束的目标heap_live； ^0（如果禁用）
	last_gc_unix    uint64 // last gc (in unix time) // 最后一次gc（Unix时间）
	pause_total_ns  uint64
	pause_ns        [256]uint64 // circular buffer of recent gc pause lengths // 最近gc暂停长度的循环缓冲区
	pause_end       [256]uint64 // circular buffer of recent gc end times (nanoseconds since 1970) // 最近gc结束时间的循环缓冲区（自1970年以来的纳秒）
	numgc           uint32
	numforcedgc     uint32  // number of user-forced GCs // 用户强制GC的数量
	gc_cpu_fraction float64 // fraction of CPU time used by GC // GC使用的CPU时间的一部分
	enablegc        bool
	debuggc         bool

	// Statistics about allocation size classes.
    // 有关分配大小类的统计信息。

	by_size [_NumSizeClasses]struct {
		size    uint32
		nmalloc uint64
		nfree   uint64
	}

	// Statistics below here are not exported to MemStats directly.
    // 下面的统计信息不会直接导出到MemStats。

	last_gc_nanotime uint64 // last gc (monotonic time) // 最后gc（单调时间）
	tinyallocs       uint64 // number of tiny allocations that didn't cause actual allocation; not exported to go directly // 不会导致实际分配的微小分配的数量； 没有直接导出到go
	last_next_gc     uint64 // next_gc for the previous GC // 上一个GC：next_gc
	last_heap_inuse  uint64 // heap_inuse at mark termination of the previous GC // 前一个GC的标记终止处的heap_inuse

	// triggerRatio is the heap growth ratio that triggers marking.
	//
	// E.g., if this is 0.6, then GC should start when the live
	// heap has reached 1.6 times the heap size marked by the
	// previous cycle. This should be ≤ GOGC/100 so the trigger
	// heap size is less than the goal heap size. This is set
	// during mark termination for the next cycle's trigger.
    //
    // triggerRatio是触发标记的堆增长比率。
    //
    // 例如，如果该值为0.6，则当活动堆达到上一个周期标记的堆大小的1.6倍时，GC应该启动。
    // 该值应≤GOGC/100，以便触发器堆大小小于目标堆大小。这是在标记终止期间为下一个循环的触发设置的。
	triggerRatio float64

	// gc_trigger is the heap size that triggers marking.
	//
	// When heap_live ≥ gc_trigger, the mark phase will start.
	// This is also the heap size by which proportional sweeping
	// must be complete.
	//
	// This is computed from triggerRatio during mark termination
	// for the next cycle's trigger.
    //
    // gc_trigger是触发标记的堆大小。
    //
    // 当heap_live≥gc_trigger时，标记阶段将开始。
    // 这也是必须完成比例扫描的堆大小。
    //
    // 这是在标记终止期间为下一个循环的触发器从triggerRatio计算的。
	gc_trigger uint64

	// heap_live is the number of bytes considered live by the GC.
	// That is: retained by the most recent GC plus allocated
	// since then. heap_live <= heap_alloc, since heap_alloc
	// includes unmarked objects that have not yet been swept (and
	// hence goes up as we allocate and down as we sweep) while
	// heap_live excludes these objects (and hence only goes up
	// between GCs).
	//
	// This is updated atomically without locking. To reduce
	// contention, this is updated only when obtaining a span from
	// an mcentral and at this point it counts all of the
	// unallocated slots in that span (which will be allocated
	// before that mcache obtains another span from that
	// mcentral). Hence, it slightly overestimates the "true" live
	// heap size. It's better to overestimate than to
	// underestimate because 1) this triggers the GC earlier than
	// necessary rather than potentially too late and 2) this
	// leads to a conservative GC rate rather than a GC rate that
	// is potentially too low.
	//
	// Reads should likewise be atomic (or during STW).
	//
	// Whenever this is updated, call traceHeapAlloc() and
	// gcController.revise().
    // heap_live是GC认为存活的字节数。
    // 即：由最近的GC保留，此后再分配。 heap_live <= heap_alloc，
    // 因为heap_alloc包括尚未被清除的未标记对象（因此在我们分配时上升，而在扫描时下降），
    // 而heap_live排除这些对象（因此仅在GC之间上升）。
    //
    // 这是自动更新的，没有锁定。为了减少争用，仅当从一个mcentrol获得一个跨度(span)时才更新该争用，
    // 此时，它会计算该跨度中所有未分配的槽位（在该mcache从该mcentrol获取另一个跨度之前将对其进行分配）。
    // 因此，它稍微高估了“真实”活动堆的大小。最好高估而不是低估，因为
    //  1）触发GC的时间比必要的要早，而不是可能太晚； 
    //  2）导致保守的GC速率，而不是可能太低的GC速率。
    //
    // 读取应该同样是原子的（或在STW期间）。
    //
    // 每当更新时，都调用traceHeapAlloc()和gcController.revise()。
	heap_live uint64

	// heap_scan is the number of bytes of "scannable" heap. This
	// is the live heap (as counted by heap_live), but omitting
	// no-scan objects and no-scan tails of objects.
	//
	// Whenever this is updated, call gcController.revise().
    // 
    // heap_scan是“可扫描”堆的字节数。这是活动堆（由heap_live计算），
    // 但是省略了非扫描对象和对象的非扫描尾部。
    //
    // 每当更新时，都调用gcController.revise()。
	heap_scan uint64

	// heap_marked is the number of bytes marked by the previous
	// GC. After mark termination, heap_live == heap_marked, but
	// unlike heap_live, heap_marked does not change until the
	// next mark termination.
    // heap_marked是前一个GC标记的字节数。标记终止后，heap_live == heap_marked，
    // 但是与heap_live不同，heap_marked直到下一个标记终止才更改。
	heap_marked uint64
}

var memstats mstats

// A MemStats records statistics about the memory allocator.
// MemStats记录有关内存分配器的统计信息。
type MemStats struct {
	// General statistics.
    // 常规统计信息。

	// Alloc is bytes of allocated heap objects.
	//
	// This is the same as HeapAlloc (see below).
    //
    // Alloc是分配的堆对象的字节。
    //
    // 这与HeapAlloc相同（请参见下文）。
	Alloc uint64

	// TotalAlloc is cumulative bytes allocated for heap objects.
	//
	// TotalAlloc increases as heap objects are allocated, but
	// unlike Alloc and HeapAlloc, it does not decrease when
	// objects are freed.
    // TotalAlloc是分配给堆对象的累积字节。
    //
    // TotalAlloc随着分配堆对象而增加，但是与Alloc和HeapAlloc不同，释放对象时它不会减少。
	TotalAlloc uint64

	// Sys is the total bytes of memory obtained from the OS.
	//
	// Sys is the sum of the XSys fields below. Sys measures the
	// virtual address space reserved by the Go runtime for the
	// heap, stacks, and other internal data structures. It's
	// likely that not all of the virtual address space is backed
	// by physical memory at any given moment, though in general
	// it all was at some point.
    // 
    // Sys是从OS获得的内存的总字节数。
    //
    // Sys是以下XSys字段的总和。 Sys衡量Go运行时为堆，栈和其他内部数据结构保留的虚拟地址空间。
    // 在任何给定时刻，可能并非所有虚拟地址空间都由物理内存支持，尽管通常情况下，所有虚拟地址空间都在某个时刻。
	Sys uint64

	// Lookups is the number of pointer lookups performed by the
	// runtime.
	//
	// This is primarily useful for debugging runtime internals.
    // 查找是运行时执行的指针查找的次数。
    //
    // 这主要用于调试运行时内部。
	Lookups uint64

	// Mallocs is the cumulative count of heap objects allocated.
	// The number of live objects is Mallocs - Frees.
    // Mallocs是分配的堆对象的累积计数。
    // 活动对象的数量为Malloc-Frees。
	Mallocs uint64

	// Frees is the cumulative count of heap objects freed.
    // Frees是已释放的堆对象的累积计数。
	Frees uint64

	// Heap memory statistics.
	//
	// Interpreting the heap statistics requires some knowledge of
	// how Go organizes memory. Go divides the virtual address
	// space of the heap into "spans", which are contiguous
	// regions of memory 8K or larger. A span may be in one of
	// three states:
	//
	// An "idle" span contains no objects or other data. The
	// physical memory backing an idle span can be released back
	// to the OS (but the virtual address space never is), or it
	// can be converted into an "in use" or "stack" span.
	//
	// An "in use" span contains at least one heap object and may
	// have free space available to allocate more heap objects.
	//
	// A "stack" span is used for goroutine stacks. Stack spans
	// are not considered part of the heap. A span can change
	// between heap and stack memory; it is never used for both
	// simultaneously.

	// HeapAlloc is bytes of allocated heap objects.
	//
	// "Allocated" heap objects include all reachable objects, as
	// well as unreachable objects that the garbage collector has
	// not yet freed. Specifically, HeapAlloc increases as heap
	// objects are allocated and decreases as the heap is swept
	// and unreachable objects are freed. Sweeping occurs
	// incrementally between GC cycles, so these two processes
	// occur simultaneously, and as a result HeapAlloc tends to
	// change smoothly (in contrast with the sawtooth that is
	// typical of stop-the-world garbage collectors).
    // 堆内存统计信息。
    //
    // 解释堆统计信息需要了解Go如何组织内存。 Go将堆的虚拟地址空间划分为跨度（“span”），
    // 即内存8K或更大的连续区域。跨度可能处于以下三种状态之一：
    //
    // “空闲”跨度（span）不包含任何对象或其他数据。支持空闲范围的物理内存可以释放回操作系统
    // （但虚拟地址空间永远不会释放），也可以将其转换为“使用中”或“堆栈”跨度（span）。
    //
    // 一个“使用中”的跨度至少包含一个堆对象，并且可能具有可用空间来分配更多的堆对象。
    //
    // “堆栈”跨度用于goroutine堆栈。堆栈跨度不视为堆的一部分。跨度可以在堆和堆栈内存之间改变；永远不会同时使用它们。
    // 
    // HeapAlloc是分配的堆对象的字节。
    //
    // “已分配”堆对象包括所有可访问对象，以及垃圾回收器尚未释放的不可访问对象。
    // 具体来说，HeapAlloc随着分配堆对象而增加，而随着堆被清除而无法访问的对象被释放而减少。
    // 清除过程在GC周期之间逐渐发生，因此这两个过程同时发生，因此HeapAlloc趋于平稳变化
    // （与全局停机的垃圾收集器所采用的典型锯齿形成对比）。
	HeapAlloc uint64

	// HeapSys is bytes of heap memory obtained from the OS.
	//
	// HeapSys measures the amount of virtual address space
	// reserved for the heap. This includes virtual address space
	// that has been reserved but not yet used, which consumes no
	// physical memory, but tends to be small, as well as virtual
	// address space for which the physical memory has been
	// returned to the OS after it became unused (see HeapReleased
	// for a measure of the latter).
	//
	// HeapSys estimates the largest size the heap has had.
    // HeapSys是从操作系统获得的堆内存字节。
    //
    // HeapSys测量为堆保留的虚拟地址空间量。这包括已保留但尚未使用的虚拟地址空间，
    // 该虚拟地址空间不占用物理内存，但往往很小，以及在物理内存变得不使用后已将其
    // 返回给操作系统的虚拟地址空间（请参阅HeapReleased以衡量后者）。
    //
    // HeapSys估计堆具有的最大大小。
	HeapSys uint64

	// HeapIdle is bytes in idle (unused) spans.
	//
	// Idle spans have no objects in them. These spans could be
	// (and may already have been) returned to the OS, or they can
	// be reused for heap allocations, or they can be reused as
	// stack memory.
	//
	// HeapIdle minus HeapReleased estimates the amount of memory
	// that could be returned to the OS, but is being retained by
	// the runtime so it can grow the heap without requesting more
	// memory from the OS. If this difference is significantly
	// larger than the heap size, it indicates there was a recent
	// transient spike in live heap size.
    // HeapIdle是空闲（未使用）跨度中的字节。
    //
    // 空闲跨度中没有对象。这些范围可以（并且可能已经）返回到操作系统，
    // 或者可以重新用于堆分配，或者可以重新用作栈内存。
    //
    // HeapIdle减去HeapReleased估计可以返回给OS的内存量，但是该内存将由运行时保留，
    // 因此它可以增长堆而无需从OS请求更多内存。如果此差异明显大于堆大小，
    // 则表明活动堆大小最近出现了短暂的峰值。
	HeapIdle uint64

	// HeapInuse is bytes in in-use spans.
	//
	// In-use spans have at least one object in them. These spans
	// can only be used for other objects of roughly the same
	// size.
	//
	// HeapInuse minus HeapAlloc estimates the amount of memory
	// that has been dedicated to particular size classes, but is
	// not currently being used. This is an upper bound on
	// fragmentation, but in general this memory can be reused
	// efficiently.
    // HeapInuse是使用中的跨度中的字节。
    //
    // 使用中的跨度中至少包含一个对象。这些跨度只能用于大小大致相同的其他对象。
    //
    // HeapInuse减去HeapAlloc估计专用于特定大小类别的内存量，但当前未使用。这是碎片的上限，但是通常可以有效地重用此内存。
	HeapInuse uint64

	// HeapReleased is bytes of physical memory returned to the OS.
	//
	// This counts heap memory from idle spans that was returned
	// to the OS and has not yet been reacquired for the heap.
    // HeapReleased是返回操作系统的物理内存字节。
    //
    // 这将从返回到OS且尚未为堆重新获取的span范围中计算堆内存。
	HeapReleased uint64

	// HeapObjects is the number of allocated heap objects.
	//
	// Like HeapAlloc, this increases as objects are allocated and
	// decreases as the heap is swept and unreachable objects are
	// freed.
    // HeapObjects是分配的堆对象的数量。
    //
    // 像HeapAlloc一样，随着分配对象的增加，它会增加；而随着堆的清除和无法访问的对象的释放，这会减少。
	HeapObjects uint64

	// Stack memory statistics.
	//
	// Stacks are not considered part of the heap, but the runtime
	// can reuse a span of heap memory for stack memory, and
	// vice-versa.
    // 栈内存统计信息。
    //
    // 栈不被视为堆的一部分，但是运行时可以将堆内存的一部分重用于栈内存，反之亦然。


	// StackInuse is bytes in stack spans.
	//
	// In-use stack spans have at least one stack in them. These
	// spans can only be used for other stacks of the same size.
	//
	// There is no StackIdle because unused stack spans are
	// returned to the heap (and hence counted toward HeapIdle).
    // StackInuse是栈跨度中的字节。
    //
    // 使用中的栈跨度中至少包含一个栈。这些跨度只能用于相同大小的其他栈。
    //
    // 没有StackIdle，因为未使用的栈范围返回到堆中（因此计入HeapIdle）
	StackInuse uint64

	// StackSys is bytes of stack memory obtained from the OS.
	//
	// StackSys is StackInuse, plus any memory obtained directly
	// from the OS for OS thread stacks (which should be minimal).
    // StackSys是从OS获得的栈内存的字节。
    //
    // StackSys是StackInuse，再加上直接从OS获取的用于OS线程栈的任何内存（应该很少）。
	StackSys uint64

	// Off-heap memory statistics.
	//
	// The following statistics measure runtime-internal
	// structures that are not allocated from heap memory (usually
	// because they are part of implementing the heap). Unlike
	// heap or stack memory, any memory allocated to these
	// structures is dedicated to these structures.
	//
	// These are primarily useful for debugging runtime memory
	// overheads.
    // 堆外内存统计信息。
    //
    // 以下统计信息将度量未从堆内存分配的运行时内部结构（通常是因为它们是实现堆的一部分）。
    // 与堆或栈内存不同，分配给这些结构的任何内存都专用于这些结构。
    //
    // 这些主要用于调试运行时内存开销。

	// MSpanInuse is bytes of allocated mspan structures.
    // MSpanInuse是分配的mspan结构的字节。
	MSpanInuse uint64

	// MSpanSys is bytes of memory obtained from the OS for mspan
	// structures.
    // MSpanSys是从操作系统获取的用于mspan结构的内存字节。
	MSpanSys uint64

	// MCacheInuse is bytes of allocated mcache structures.
    // MCacheInuse是分配的mcache结构的字节。
	MCacheInuse uint64

	// MCacheSys is bytes of memory obtained from the OS for
	// mcache structures.
    // MCacheSys是从操作系统获取的用于mcache结构的内存字节。
	MCacheSys uint64

	// BuckHashSys is bytes of memory in profiling bucket hash tables.
    // BuckHashSys是分析存储桶哈希表中的内存字节。
	BuckHashSys uint64

	// GCSys is bytes of memory in garbage collection metadata.
    // GCSys是垃圾回收元数据中的内存字节。
	GCSys uint64

	// OtherSys is bytes of memory in miscellaneous off-heap
	// runtime allocations.
    // OtherSys是其他堆外运行时分配中的内存字节。
	OtherSys uint64

	// Garbage collector statistics.
    // 垃圾收集器统计信息。

	// NextGC is the target heap size of the next GC cycle.
	//
	// The garbage collector's goal is to keep HeapAlloc ≤ NextGC.
	// At the end of each GC cycle, the target for the next cycle
	// is computed based on the amount of reachable data and the
	// value of GOGC.
    // NextGC是下一个GC周期的目标堆大小。
    //
    // 垃圾收集器的目标是保持HeapAlloc≤NextGC。
    // 在每个GC周期结束时，根据可获得的数据量和GOGC的值计算下一个周期的目标。
	NextGC uint64

	// LastGC is the time the last garbage collection finished, as
	// nanoseconds since 1970 (the UNIX epoch).
    // LastGC是最后一次垃圾回收完成的时间，自1970年（UNIX时代）以来，以纳秒为单位。
	LastGC uint64

	// PauseTotalNs is the cumulative nanoseconds in GC
	// stop-the-world pauses since the program started.
	//
	// During a stop-the-world pause, all goroutines are paused
	// and only the garbage collector can run.
    // PauseTotalNs是自程序启动以来，GC全局停机暂停的累积纳秒。
    //
    // 在全局停机暂停期间，所有goroutine都将暂停并且只有垃圾收集器可以运行。
	PauseTotalNs uint64

	// PauseNs is a circular buffer of recent GC stop-the-world
	// pause times in nanoseconds.
	//
	// The most recent pause is at PauseNs[(NumGC+255)%256]. In
	// general, PauseNs[N%256] records the time paused in the most
	// recent N%256th GC cycle. There may be multiple pauses per
	// GC cycle; this is the sum of all pauses during a cycle.
    // PauseNs是最近GC全局停机暂停时间（以纳秒为单位）的循环缓冲区。
    //
    // 最近的暂停是在PauseNs[(NumGC+255)%56]处。通常，PauseNs[N%256]记录最近的第N%256个GC周期中暂停的时间。
    // 每个GC周期可能会有多个暂停；这是一个周期中所有暂停的总和。
	PauseNs [256]uint64

	// PauseEnd is a circular buffer of recent GC pause end times,
	// as nanoseconds since 1970 (the UNIX epoch).
	//
	// This buffer is filled the same way as PauseNs. There may be
	// multiple pauses per GC cycle; this records the end of the
	// last pause in a cycle.
    // PauseEnd是最近的GC暂停结束时间的循环缓冲区，自1970年（UNIX时代）以来为纳秒。
    //
    // 此缓冲区的填充方式与PauseNs相同。每个GC周期可能会有多个暂停；这将记录一个周期中最后一个暂停的结束。
	PauseEnd [256]uint64

	// NumGC is the number of completed GC cycles.
    // NumGC是已完成的GC周期数。
	NumGC uint32

	// NumForcedGC is the number of GC cycles that were forced by
	// the application calling the GC function.
    // NumForcedGC是应用程序调用GC函数强制执行的GC周期数。
	NumForcedGC uint32

	// GCCPUFraction is the fraction of this program's available
	// CPU time used by the GC since the program started.
	//
	// GCCPUFraction is expressed as a number between 0 and 1,
	// where 0 means GC has consumed none of this program's CPU. A
	// program's available CPU time is defined as the integral of
	// GOMAXPROCS since the program started. That is, if
	// GOMAXPROCS is 2 and a program has been running for 10
	// seconds, its "available CPU" is 20 seconds. GCCPUFraction
	// does not include CPU time used for write barrier activity.
	//
	// This is the same as the fraction of CPU reported by
	// GODEBUG=gctrace=1.
    // 自程序启动以来，GCCPUFraction是该程序使用的该程序可用CPU时间的一部分。
    //
    // GCCPUFraction表示为0到1之间的数字，其中0表示GC没有消耗该程序的CPU。
    // 自程序启动以来，程序的可用CPU时间定义为GOMAXPROCS的整数。也就是说，
    // 如果GOMAXPROCS为2且程序已运行10秒钟，则其“可用CPU”为20秒钟。 
    //  GCCPUFraction不包括用于写屏障活动的CPU时间。
    //
    //这与GODEBUG = gctrace = 1报告的CPU分数相同。
	GCCPUFraction float64

	// EnableGC indicates that GC is enabled. It is always true,
	// even if GOGC=off.
    // EnableGC表示已启用GC。即使GOGC = off，也总是如此
	EnableGC bool

	// DebugGC is currently unused.
    // // DebugGC当前未使用。
	DebugGC bool

	// BySize reports per-size class allocation statistics.
	//
	// BySize[N] gives statistics for allocations of size S where
	// BySize[N-1].Size < S ≤ BySize[N].Size.
	//
	// This does not report allocations larger than BySize[60].Size.
    // BySize报告按大小分类分配的统计信息。
    //
    // BySize [N]给出大小为S的分配的统计信息，其中
    // BySize [N-1] .Size <S≤BySize [N] .Size。
    //
    // 这不会报告大于BySize [60] .Size的分配。
	BySize [61]struct {
		// Size is the maximum byte size of an object in this
		// size class.
        // Size是此size类中对象的最大字节大小。
		Size uint32

		// Mallocs is the cumulative count of heap objects
		// allocated in this size class. The cumulative bytes
		// of allocation is Size*Mallocs. The number of live
		// objects in this size class is Mallocs - Frees.
        // Mallocs是在此size类中分配的堆对象的累积计数。分配的累积字节为Size * Mallocs。
        // 此大小类中的活动对象数量为Mallocs-Frees。
		Mallocs uint64

		// Frees is the cumulative count of heap objects freed
		// in this size class.
        // Frees是在此size类中释放的堆对象的累积计数。
		Frees uint64
	}
}

// Size of the trailing by_size array differs between mstats and MemStats,
// and all data after by_size is local to runtime, not exported.
// NumSizeClasses was changed, but we cannot change MemStats because of backward compatibility.
// sizeof_C_MStats is the size of the prefix of mstats that
// corresponds to MemStats. It should match Sizeof(MemStats{}).
// trailing by_size数组的大小在mstats和MemStats之间有所不同，并且by_size之后的所有数据对于运行时都是本地的，而不是导出的。
// NumSizeClasses已更改，但由于向后兼容，我们无法更改MemStats。
// sizeof_C_MStats是与MemStats相对应的mstats前缀的大小。 它应该匹配Sizeof（MemStats {}）。
// memstats.by_size实际是长度为67的数组，这里使用61进行计算，就是兼容
var sizeof_C_MStats = unsafe.Offsetof(memstats.by_size) + 61*unsafe.Sizeof(memstats.by_size[0])

// 初始化，主要做完成
// 1、sizeof_C_MStats和unsafe.Sizeof(memStats)要相等
// 2、unsafe.Offsetof(memstats.heap_live)要8字节对齐
func init() {
	var memStats MemStats
	if sizeof_C_MStats != unsafe.Sizeof(memStats) {
		println(sizeof_C_MStats, unsafe.Sizeof(memStats))
		throw("MStats vs MemStatsType size mismatch")
	}

	if unsafe.Offsetof(memstats.heap_live)%8 != 0 {
		println(unsafe.Offsetof(memstats.heap_live))
		throw("memstats.heap_live not aligned to 8 bytes")
	}
}

// ReadMemStats populates m with memory allocator statistics.
//
// The returned memory allocator statistics are up to date as of the
// call to ReadMemStats. This is in contrast with a heap profile,
// which is a snapshot as of the most recently completed garbage
// collection cycle.
// ReadMemStats使用内存分配器统计信息填充m。
//
// 调用ReadMemStats以来，返回的内存分配器统计信息是最新的。 
// 这与堆概要相反，后者是最新完成的垃圾收集周期的快照。
func ReadMemStats(m *MemStats) {
	stopTheWorld("read mem stats")

	systemstack(func() {
		readmemstats_m(m)
	})

	startTheWorld()
}

// 读取内存统计信息
func readmemstats_m(stats *MemStats) {
	updatememstats()

	// The size of the trailing by_size array differs between
	// mstats and MemStats. NumSizeClasses was changed, but we
	// cannot change MemStats because of backward compatibility.
    // trailing by_size数组的大小在mstats和MemStats之间有所不同。 
    // NumSizeClasses已更改，但由于向后兼容，我们无法更改MemStats。
	memmove(unsafe.Pointer(stats), unsafe.Pointer(&memstats), sizeof_C_MStats)

	// memstats.stacks_sys is only memory mapped directly for OS stacks.
	// Add in heap-allocated stack memory for user consumption.
    // memstats.stacks_sys仅是直接为OS堆栈映射的内存。 添加堆分配的堆栈内存以供用户使用。
	stats.StackSys += stats.StackInuse
}

//go:linkname readGCStats runtime/debug.readGCStats
// 读取GC统计信息
func readGCStats(pauses *[]uint64) {
	systemstack(func() {
		readGCStats_m(pauses)
	})
}

// readGCStats_m must be called on the system stack because it acquires the heap
// lock. See mheap for details.
// 必须在系统堆栈上调用readGCStats_m，因为它获取了堆锁。有关详细信息，请参见mheap。
//go:systemstack
func readGCStats_m(pauses *[]uint64) {
	p := *pauses
	// Calling code in runtime/debug should make the slice large enough.
    // 调用代码在runtime/debug，应当使切片足够大
	if cap(p) < len(memstats.pause_ns)+3 {
		throw("short slice passed to readGCStats")
	}

	// Pass back: pauses, pause ends, last gc (absolute time), number of gc, total pause ns.
    // 回传：暂停，暂停结束，最后gc（绝对时间），gc数量，总暂停ns。
	lock(&mheap_.lock)

	n := memstats.numgc
	if n > uint32(len(memstats.pause_ns)) {
		n = uint32(len(memstats.pause_ns))
	}

	// The pause buffer is circular. The most recent pause is at
	// pause_ns[(numgc-1)%len(pause_ns)], and then backward
	// from there to go back farther in time. We deliver the times
	// most recent first (in p[0]).
    // 暂停缓冲区是循环的。 最近的暂停是在pause_ns[(numgc-1)%len(pause_ns)]处，
    // 然后从那里向后退，以便及时返回。 我们以最近的时间为准(p[0])。
	p = p[:cap(p)]
	for i := uint32(0); i < n; i++ {
		j := (memstats.numgc - 1 - i) % uint32(len(memstats.pause_ns))
		p[i] = memstats.pause_ns[j]
		p[n+i] = memstats.pause_end[j]
	}

	p[n+n] = memstats.last_gc_unix
	p[n+n+1] = uint64(memstats.numgc)
	p[n+n+2] = memstats.pause_total_ns
	unlock(&mheap_.lock)
	*pauses = p[:n+n+3]
}

// 更新内存统计信息
//go:nowritebarrier
func updatememstats() {
	memstats.mcache_inuse = uint64(mheap_.cachealloc.inuse) // mcache使用的字节数
	memstats.mspan_inuse = uint64(mheap_.spanalloc.inuse)   // span使用的字节数
	memstats.sys = memstats.heap_sys + memstats.stacks_sys + memstats.mspan_sys +
		memstats.mcache_sys + memstats.buckhash_sys + memstats.gc_sys + memstats.other_sys // sys使用的字节数

	// We also count stacks_inuse as sys memory.
    // 我们也将stacks_inuse视为系统内存。
	memstats.sys += memstats.stacks_inuse

	// Calculate memory allocator stats.
	// During program execution we only count number of frees and amount of freed memory.
	// Current number of alive object in the heap and amount of alive heap memory
	// are calculated by scanning all spans.
	// Total number of mallocs is calculated as number of frees plus number of alive objects.
	// Similarly, total amount of allocated memory is calculated as amount of freed memory
	// plus amount of alive heap memory.
    // 计算内存分配器统计信息。
    // 在程序执行过程中，我们仅计算可用数量和可用内存量。
    // 通过扫描所有范围来计算堆中当前活动对象的数量和活动堆内存的数量。
    // malloc的总数计算为空闲数加上活动对象数。
    // 类似地，分配的内存总量计算为释放的内存总量加上活动堆内存总量。
	memstats.alloc = 0
	memstats.total_alloc = 0
	memstats.nmalloc = 0
	memstats.nfree = 0
	for i := 0; i < len(memstats.by_size); i++ {
		memstats.by_size[i].nmalloc = 0
		memstats.by_size[i].nfree = 0
	}

	// Flush mcache's to mcentral.
    // 将mcache刷新到mcentral。
	systemstack(flushallmcaches)

	// Aggregate local stats.
    // 汇总本地统计信息。
	cachestats()

	// Collect allocation stats. This is safe and consistent
	// because the world is stopped.
    // 收集分配统计信息。这是安全且一致的，因为已全局停机。
	var smallFree, totalAlloc, totalFree uint64
	// Collect per-spanclass stats.
    // 收集每个跨类的统计信息。
	for spc := range mheap_.central {
		// The mcaches are now empty, so mcentral stats are
		// up-to-date.
        // mcache现在是空的，因此mcentral统计​​信息是最新的。
		c := &mheap_.central[spc].mcentral
		memstats.nmalloc += c.nmalloc
		i := spanClass(spc).sizeclass()
		memstats.by_size[i].nmalloc += c.nmalloc
		totalAlloc += c.nmalloc * uint64(class_to_size[i])
	}
	// Collect per-sizeclass stats.
    // 收集每个大小类别的统计信息。
	for i := 0; i < _NumSizeClasses; i++ {
		if i == 0 {
			memstats.nmalloc += mheap_.nlargealloc
			totalAlloc += mheap_.largealloc
			totalFree += mheap_.largefree
			memstats.nfree += mheap_.nlargefree
			continue
		}

		// The mcache stats have been flushed to mheap_.
        // mcache统计信息已刷新到mheap_。
		memstats.nfree += mheap_.nsmallfree[i]
		memstats.by_size[i].nfree = mheap_.nsmallfree[i]
		smallFree += mheap_.nsmallfree[i] * uint64(class_to_size[i])
	}
	totalFree += smallFree

	memstats.nfree += memstats.tinyallocs
	memstats.nmalloc += memstats.tinyallocs

	// Calculate derived stats.
    // 计算派生的统计信息。
	memstats.total_alloc = totalAlloc
	memstats.alloc = totalAlloc - totalFree
	memstats.heap_alloc = memstats.alloc
	memstats.heap_objects = memstats.nmalloc - memstats.nfree
}

// cachestats flushes all mcache stats.
//
// The world must be stopped.
//
// cachestats刷新所有mcache统计信息。
// 全局必须停机
//
//go:nowritebarrier
func cachestats() {
	for _, p := range allp {
		c := p.mcache
		if c == nil {
			continue
		}
		purgecachedstats(c)
	}
}

// flushmcache flushes the mcache of allp[i].
//
// The world must be stopped.
//
// flushmcache刷新allp [i]的mcache。
// 全局必须停机
//
//go:nowritebarrier
func flushmcache(i int) {
	p := allp[i]
	c := p.mcache
	if c == nil {
		return
	}
	c.releaseAll()
	stackcache_clear(c)
}

// flushallmcaches flushes the mcaches of all Ps.
//
// The world must be stopped.
//
// flushallmcaches刷新所有P的mcache。
//
// 全局必须停机。
//go:nowritebarrier
func flushallmcaches() {
	for i := 0; i < int(gomaxprocs); i++ {
		flushmcache(i)
	}
}

//go:nosplit
func purgecachedstats(c *mcache) {
	// Protected by either heap or GC lock.
    // 受堆或GC锁保护。
	h := &mheap_
	memstats.heap_scan += uint64(c.local_scan)
	c.local_scan = 0
	memstats.tinyallocs += uint64(c.local_tinyallocs)
	c.local_tinyallocs = 0
	h.largefree += uint64(c.local_largefree)
	c.local_largefree = 0
	h.nlargefree += uint64(c.local_nlargefree)
	c.local_nlargefree = 0
	for i := 0; i < len(c.local_nsmallfree); i++ {
		h.nsmallfree[i] += uint64(c.local_nsmallfree[i])
		c.local_nsmallfree[i] = 0
	}
}

// Atomically increases a given *system* memory stat. We are counting on this
// stat never overflowing a uintptr, so this function must only be used for
// system memory stats.
//
// The current implementation for little endian architectures is based on
// xadduintptr(), which is less than ideal: xadd64() should really be used.
// Using xadduintptr() is a stop-gap solution until arm supports xadd64() that
// doesn't use locks.  (Locks are a problem as they require a valid G, which
// restricts their useability.)
//
// A side-effect of using xadduintptr() is that we need to check for
// overflow errors.
// 以原子方式增加给定的*system*内存统计信息。我们指望该统计信息永远不会溢出uintptr，
// 因此该函数只能用于系统内存统计信息。
//
// 小端字节序体系结构的当前实现是基于xadduintptr()的，这并不理想：实际上应该使用xadd64()。
// 使用xadduintptr()是一个权宜之计，直到arm支持不使用锁的xadd64()为止。
// （锁是一个问题，因为它们需要有效的G，这限制了其可用性。）
//
// 使用xadduintptr()的副作用是我们需要检查溢出错误。
//go:nosplit
func mSysStatInc(sysStat *uint64, n uintptr) {
	if sysStat == nil {
		return
	}
	if sys.BigEndian {
		atomic.Xadd64(sysStat, int64(n))
		return
	}
	if val := atomic.Xadduintptr((*uintptr)(unsafe.Pointer(sysStat)), n); val < n {
		print("runtime: stat overflow: val ", val, ", n ", n, "\n")
		exit(2)
	}
}

// Atomically decreases a given *system* memory stat. Same comments as
// mSysStatInc apply.
// 以原子方式减少给定的*system*内存状态。与mSysStatInc的注释相同。
//go:nosplit
func mSysStatDec(sysStat *uint64, n uintptr) {
	if sysStat == nil {
		return
	}
	if sys.BigEndian {
		atomic.Xadd64(sysStat, -int64(n))
		return
	}
	if val := atomic.Xadduintptr((*uintptr)(unsafe.Pointer(sysStat)), uintptr(-int64(n))); val+n < n {
		print("runtime: stat underflow: val ", val, ", n ", n, "\n")
		exit(2)
	}
}
```