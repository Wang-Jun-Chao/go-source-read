```go
// Page allocator.
//
// The page allocator manages mapped pages (defined by pageSize, NOT
// physPageSize) for allocation and re-use. It is embedded into mheap.
//
// Pages are managed using a bitmap that is sharded into chunks.
// In the bitmap, 1 means in-use, and 0 means free. The bitmap spans the
// process's address space. Chunks are managed in a sparse-array-style structure
// similar to mheap.arenas, since the bitmap may be large on some systems.
//
// The bitmap is efficiently searched by using a radix tree in combination
// with fast bit-wise intrinsics. Allocation is performed using an address-ordered
// first-fit approach.
//
// Each entry in the radix tree is a summary that describes three properties of
// a particular region of the address space: the number of contiguous free pages
// at the start and end of the region it represents, and the maximum number of
// contiguous free pages found anywhere in that region.
//
// Each level of the radix tree is stored as one contiguous array, which represents
// a different granularity of subdivision of the processes' address space. Thus, this
// radix tree is actually implicit in these large arrays, as opposed to having explicit
// dynamically-allocated pointer-based node structures. Naturally, these arrays may be
// quite large for system with large address spaces, so in these cases they are mapped
// into memory as needed. The leaf summaries of the tree correspond to a bitmap chunk.
//
// The root level (referred to as L0 and index 0 in pageAlloc.summary) has each
// summary represent the largest section of address space (16 GiB on 64-bit systems),
// with each subsequent level representing successively smaller subsections until we
// reach the finest granularity at the leaves, a chunk.
//
// More specifically, each summary in each level (except for leaf summaries)
// represents some number of entries in the following level. For example, each
// summary in the root level may represent a 16 GiB region of address space,
// and in the next level there could be 8 corresponding entries which represent 2
// GiB subsections of that 16 GiB region, each of which could correspond to 8
// entries in the next level which each represent 256 MiB regions, and so on.
//
// Thus, this design only scales to heaps so large, but can always be extended to
// larger heaps by simply adding levels to the radix tree, which mostly costs
// additional virtual address space. The choice of managing large arrays also means
// that a large amount of virtual address space may be reserved by the runtime.
//
// 页面分配器。
//
// 页面分配器管理映射的页面（由pageSize定义，而不是physPageSize定义）以进行分配和重用。它被嵌入到mheap中。
//
// 使用分片的位图管理页面。在位图中，1表示正在使用，0表示空闲。位图跨越进程的地址空间。
// 块以类似于mheap.arenas的稀疏数组样式结构进行管理，因为在某些系统上位图可能很大。
//
// 通过结合使用基数树和快速按位内在函数有效地搜索位图。分配是使用地址顺序优先拟合方法执行的。
//
// 基数树中的每个条目都是一个摘要，描述了地址空间特定区域的三个属性：它所代表的区域的开始和结束处的连续空闲页面数，
// 以及找到的最大连续空闲页面数该地区的任何地方。
//
// 基数树的每一级都存储为一个连续的数组，它代表了进程地址空间细分的不同粒度。因此，与具有显式动态分配的基于指针的节点结构相反，
// 该基数树实际上隐含在这些大数组中。自然，对于具有大地址空间的系统，这些数组可能会很大，因此在这些情况下，它们会根据需要映射到内存中。
// 树的叶子摘要对应于位图块。
//
// 根级别（在pageAlloc.summary中称为L0和index 0）使每个摘要代表地址空间的最大部分（在64位系统上为16 GiB），
// 随后的每个级别代表依次较小的子部分，直到我们到达叶子上最好的粒度，一大块。
//
// 更具体地说，每个级别中的每个摘要（叶子摘要除外）代表下一个级别中的一些条目。
// 例如，根级别中的每个摘要都可以表示地址空间的16 GiB区域，而在下一级别中，可以有8个对应的条目，它们代表该16 GiB区域的2个GiB子节，
// 每个子节可以对应于其中的8个GiB部分。下一级别，每个级别代表256个MiB区域，依此类推。
//
// 因此，此设计仅可扩展到如此大的堆，但始终可以通过简单地向基数树添加级别来扩展到更大的堆，这通常会花费额外的虚拟地址空间。
// 选择管理大型数组还意味着运行时可能会保留大量虚拟地址空间。
package runtime

import (
	"runtime/internal/atomic"
	"unsafe"
)

const (
	// The size of a bitmap chunk, i.e. the amount of bits (that is, pages) to consider
	// in the bitmap at once.
	// 位图块的大小，即一次要在位图中考虑的位数（即页面）。
	pallocChunkPages    = 1 << logPallocChunkPages // 512
	pallocChunkBytes    = pallocChunkPages * pageSize // mac上: pageSize=8192 pallocChunkBytes=256KB
	logPallocChunkPages = 9
	logPallocChunkBytes = logPallocChunkPages + pageShift

	// The number of radix bits for each level.
	//
	// The value of 3 is chosen such that the block of summaries we need to scan at
	// each level fits in 64 bytes (2^3 summaries * 8 bytes per summary), which is
	// close to the L1 cache line width on many systems. Also, a value of 3 fits 4 tree
	// levels perfectly into the 21-bit pallocBits summary field at the root level.
	//
	// The following equation explains how each of the constants relate:
	// summaryL0Bits + (summaryLevels-1)*summaryLevelBits + logPallocChunkBytes = heapAddrBits
	//
	// summaryLevels is an architecture-dependent value defined in mpagealloc_*.go.
	//
	// 每个级别的基数位数。
    //
    // 选择3的值，以便我们需要在每个级别上扫描的摘要块适合64个字节（2^3个摘要*每个摘要8个字节），
    // 这与许多系统上的L1高速缓存行宽度相近。此外，值为3时，根级的21位pallocBits摘要字段完全适合4个树级别。
    //
    // 以下等式说明了每个常数之间的关系：
    // summaryL0Bits +（summaryLevels-1）* summaryLevelBits + logPallocChunkBytes = heapAddrBits
    //
    // summaryLevels是在mpagealloc _ *。go中定义的与体系结构相关的值。
	summaryLevelBits = 3
	summaryL0Bits    = heapAddrBits - logPallocChunkBytes - (summaryLevels-1)*summaryLevelBits

	// pallocChunksL2Bits is the number of bits of the chunk index number
	// covered by the second level of the chunks map.
	//
	// See (*pageAlloc).chunks for more details. Update the documentation
	// there should this change.
	//
	// pallocChunksL2Bits是块映射第二级所覆盖的块索引号的位数。
    //
    // 有关更多详细信息，请参见（* pageAlloc）.chunks。如果有此更改，请更新文档。
	pallocChunksL2Bits  = heapAddrBits - logPallocChunkBytes - pallocChunksL1Bits
	pallocChunksL1Shift = pallocChunksL2Bits

	// Maximum searchAddr value, which indicates that the heap has no free space.
	//
	// We subtract arenaBaseOffset because we want this to represent the maximum
	// value in the shifted address space, but searchAddr is stored as a regular
	// memory address. See arenaBaseOffset for details.
	//
	// 最大searchAddr值，指示堆没有可用空间。
    //
    // 我们减去arenaBaseOffset，因为我们希望它表示移位后的地址空间中的最大值，但是searchAddr被存储为常规内存地址。
    // 有关详细信息，请参见arenaBaseOffset。
	maxSearchAddr = ^uintptr(0) - arenaBaseOffset

	// Minimum scavAddr value, which indicates that the scavenger is done.
	//
	// minScavAddr + arenaBaseOffset == 0
	//
	// 最小scavAddr值，它表示清除程序已完成。
    //
    // minScavAddr + arenaBaseOffset == 0
	minScavAddr = (^arenaBaseOffset + 1) & uintptrMask
)

// Global chunk index.
//
// Represents an index into the leaf level of the radix tree.
// Similar to arenaIndex, except instead of arenas, it divides the address
// space into chunks.
// 全局块索引。
//
// 表示到基数树的叶级的索引。
// 与arenaIndex类似，除了代替arenas之外，它将地址空间划分为多个块。
type chunkIdx uint

// chunkIndex returns the global index of the palloc chunk containing the
// pointer p.
// chunkIndex返回包含指针p的palloc块的全局索引。
func chunkIndex(p uintptr) chunkIdx {
	return chunkIdx((p + arenaBaseOffset) / pallocChunkBytes)
}

// chunkIndex returns the base address of the palloc chunk at index ci.
// chunkIndex返回索引ci处palloc块的基址。
func chunkBase(ci chunkIdx) uintptr {
	return uintptr(ci)*pallocChunkBytes - arenaBaseOffset
}

// chunkPageIndex computes the index of the page that contains p,
// relative to the chunk which contains p.
// chunkPageIndex计算包含p的页面相对于包含p的块的索引。
func chunkPageIndex(p uintptr) uint {
	return uint(p % pallocChunkBytes / pageSize)
}

// l1 returns the index into the first level of (*pageAlloc).chunks.
// l1将索引返回到(*pageAlloc).chunks的第一级。
func (i chunkIdx) l1() uint {
	if pallocChunksL1Bits == 0 {
		// Let the compiler optimize this away if there's no
		// L1 map.
		// 如果没有L1映射，让编译器对其进行优化。
		return 0
	} else {
		return uint(i) >> pallocChunksL1Shift
	}
}

// l2 returns the index into the second level of (*pageAlloc).chunks.
func (i chunkIdx) l2() uint {
	if pallocChunksL1Bits == 0 {
		return uint(i)
	} else {
		return uint(i) & (1<<pallocChunksL2Bits - 1)
	}
}

// addrsToSummaryRange converts base and limit pointers into a range
// of entries for the given summary level.
//
// The returned range is inclusive on the lower bound and exclusive on
// the upper bound.
// addrsToSummaryRange将基本指针和限制指针转换为给定摘要级别的一系列条目。
//
// 返回的范围在下限是包含的，在上限是不包含的。
func addrsToSummaryRange(level int, base, limit uintptr) (lo int, hi int) {
	// This is slightly more nuanced than just a shift for the exclusive
	// upper-bound. Note that the exclusive upper bound may be within a
	// summary at this level, meaning if we just do the obvious computation
	// hi will end up being an inclusive upper bound. Unfortunately, just
	// adding 1 to that is too broad since we might be on the very edge of
	// of a summary's max page count boundary for this level
	// (1 << levelLogPages[level]). So, make limit an inclusive upper bound
	// then shift, then add 1, so we get an exclusive upper bound at the end.
	// 
	// 这比不包含上限的转移稍微有些微妙。请注意，不包含上限可能在此级别的摘要中，这意味着，
	// 如果我们仅进行明显的计算，hi最终将是一个不包含上限。不幸的是，仅添加1太宽了，
	// 因为我们可能处于此级别摘要（1 << levelLogPages [level]）的最大页数边界的边缘。
	// 因此，使limit为一个包含上限，然后移动，然后加1，这样我们最后得到一个不包含上限。
	lo = int((base + arenaBaseOffset) >> levelShift[level])
	hi = int(((limit-1)+arenaBaseOffset)>>levelShift[level]) + 1
	return
}

// blockAlignSummaryRange aligns indices into the given level to that
// level's block width (1 << levelBits[level]). It assumes lo is inclusive
// and hi is exclusive, and so aligns them down and up respectively.
//
// blockAlignSummaryRange将索引对齐到该级别的块宽度（1 << levelBits [level]）。
// 它假定lo是包含的，而hi是不包含的，因此分别将它们上下对齐。
func blockAlignSummaryRange(level int, lo, hi int) (int, int) {
	e := uintptr(1) << levelBits[level]
	return int(alignDown(uintptr(lo), e)), int(alignUp(uintptr(hi), e))
}

type pageAlloc struct {
	// Radix tree of summaries.
	//
	// Each slice's cap represents the whole memory reservation.
	// Each slice's len reflects the allocator's maximum known
	// mapped heap address for that level.
	//
	// The backing store of each summary level is reserved in init
	// and may or may not be committed in grow (small address spaces
	// may commit all the memory in init).
	//
	// The purpose of keeping len <= cap is to enforce bounds checks
	// on the top end of the slice so that instead of an unknown
	// runtime segmentation fault, we get a much friendlier out-of-bounds
	// error.
	//
	// To iterate over a summary level, use inUse to determine which ranges
	// are currently available. Otherwise one might try to access
	// memory which is only Reserved which may result in a hard fault.
	//
	// We may still get segmentation faults < len since some of that
	// memory may not be committed yet.
	//
	// 基数摘要树。
    //
    // 每个切片的上限代表整个内存预留。
    // 每个切片的len反映该级别分配器的最大已知映射堆地址。
    //
    // 每个摘要级别的后备存储都是在init中保留的，并且可能会（也可能不会）在grow中提交（较小的地址空间可能会在init中提交所有内存）。
    //
    // 保持len <= cap的目的是在片的顶端执行边界检查，以便代替未知的运行时分段错误，我们得到更友好的越界错误。
    //
    // 要遍历摘要级别，请使用inUse确定当前可用的范围。否则，可能会尝试访问仅保留的内存，这可能会导致严重故障。
    //
    // 由于某些内存可能尚未提交，因此我们仍然可能遇到分段错误<len。
	summary [summaryLevels][]pallocSum

	// chunks is a slice of bitmap chunks.
	//
	// The total size of chunks is quite large on most 64-bit platforms
	// (O(GiB) or more) if flattened, so rather than making one large mapping
	// (which has problems on some platforms, even when PROT_NONE) we use a
	// two-level sparse array approach similar to the arena index in mheap.
	//
	// To find the chunk containing a memory address `a`, do:
	//   chunkOf(chunkIndex(a))
	//
	// Below is a table describing the configuration for chunks for various
	// heapAddrBits supported by the runtime.
	//
	// heapAddrBits | L1 Bits | L2 Bits | L2 Entry Size
	// ------------------------------------------------
	// 32           | 0       | 10      | 128 KiB
	// 33 (iOS)     | 0       | 11      | 256 KiB
	// 48           | 13      | 13      | 1 MiB
	//
	// There's no reason to use the L1 part of chunks on 32-bit, the
	// address space is small so the L2 is small. For platforms with a
	// 48-bit address space, we pick the L1 such that the L2 is 1 MiB
	// in size, which is a good balance between low granularity without
	// making the impact on BSS too high (note the L1 is stored directly
	// in pageAlloc).
	//
	// To iterate over the bitmap, use inUse to determine which ranges
	// are currently available. Otherwise one might iterate over unused
	// ranges.
	//
	// TODO(mknyszek): Consider changing the definition of the bitmap
	// such that 1 means free and 0 means in-use so that summaries and
	// the bitmaps align better on zero-values.
	//
	// 块是位图块的一部分。
    //
    // 如果扁平化，则在大多数64位平台（O（GiB）或更大）上，块的总大小相当大，因此与其使用一个大的映射（即使在PROT_NONE上也有问题，
    // 在某些平台上还是有问题），我们使用两个级稀疏数组方法类似于mheap中的arena索引。
    //
    // 要查找包含内存地址“a”的块，请执行以下操作：
    //  chunkOf（chunkIndex（a））
    //
    // 下表描述了运行时支持的各种heapAddrBits的块配置
    //
	// heapAddrBits | L1 Bits | L2 Bits | L2 Entry Size
	// ------------------------------------------------
	// 32           | 0       | 10      | 128 KiB
	// 33 (iOS)     | 0       | 11      | 256 KiB
	// 48           | 13      | 13      | 1 MiB
	//
	// 没有理由在32位上使用块的L1部分，地址空间很小，因此L2也很小。对于具有48位地址空间的平台，
	// 我们选择L1使得L2的大小为1 MiB，这在低粒度之间取得了很好的平衡，而又不会对BSS造成太大影响
	// （请注意，L1直接存储在pageAlloc中） 。
    //
    // 要遍历位图，请使用inUse确定当前可用的范围。否则，可能会遍历未使用的范围。
    //
    // TODO（mknyszek）：考虑更改位图的定义，以使1表示空闲，0表示正在使用，以便摘要和位图在零值上更好地对齐。
	chunks [1 << pallocChunksL1Bits]*[1 << pallocChunksL2Bits]pallocData

	// The address to start an allocation search with. It must never
	// point to any memory that is not contained in inUse, i.e.
	// inUse.contains(searchAddr) must always be true.
	//
	// When added with arenaBaseOffset, we guarantee that
	// all valid heap addresses (when also added with
	// arenaBaseOffset) below this value are allocated and
	// not worth searching.
	//
	// Note that adding in arenaBaseOffset transforms addresses
	// to a new address space with a linear view of the full address
	// space on architectures with segmented address spaces.
    //
    // 用于开始分配搜索的地址。它绝不能指向inUse中未包含的任何内存，即inUse.contains（searchAddr）必须始终为true。
    //
    // 当添加了arenaBaseOffset时，我们保证分配了低于此值的所有有效堆地址（当还添加了arenaBaseOffset时），不值得搜索。
    //
    // 注意，在具有分段地址空间的体系结构上，在arenaBaseOffset中添加具有完整地址空间的线性视图的地址，会将地址转换为新的地址空间。
	searchAddr uintptr

	// The address to start a scavenge candidate search with. It
	// need not point to memory contained in inUse.
	// 用于开始搜寻候选地址的地址。它不需要指向inUse中包含的内存。
	scavAddr uintptr

	// The amount of memory scavenged since the last scavtrace print.
	//
	// Read and updated atomically.
	//
	// 自上次scavtrace打印以来清除的内存量。
    //
    // 原子读写。
	scavReleased uintptr

	// start and end represent the chunk indices
	// which pageAlloc knows about. It assumes
	// chunks in the range [start, end) are
	// currently ready to use.
	// 开始和结束表示pageAlloc知道的块索引。它假定[start，end）范围内的块当前准备就绪。
	start, end chunkIdx

	// inUse is a slice of ranges of address space which are
	// known by the page allocator to be currently in-use (passed
	// to grow).
	//
	// This field is currently unused on 32-bit architectures but
	// is harmless to track. We care much more about having a
	// contiguous heap in these cases and take additional measures
	// to ensure that, so in nearly all cases this should have just
	// 1 element.
	//
	// All access is protected by the mheapLock.
	//
	// inUse是地址空间范围的一部分，页面分配器已知该地址空间当前正在使用（传递以增长）。
    //
    // 此字段当前在32位体系结构上未使用，但对其进行跟踪无害。在这种情况下，我们非常关心具有连续堆，
    // 并采取其他措施来确保这一点，因此在几乎所有情况下，该堆都应该只有1个元素。
    //
    // 所有访问均受mheapLock保护。
	inUse addrRanges

	// mheap_.lock. This level of indirection makes it possible
	// to test pageAlloc indepedently of the runtime allocator.
	// // mheap_.lock。这种间接级别使独立于运行时分配器的页面Alloc测试成为可能。
	mheapLock *mutex

	// sysStat is the runtime memstat to update when new system
	// memory is committed by the pageAlloc for allocation metadata.
	//
	// sysStat是pageAlloc为分配元数据提交新的系统内存时要更新的运行时memstat。
	sysStat *uint64

	// Whether or not this struct is being used in tests.
	// 是否在测试中使用此结构。
	test bool
}

func (s *pageAlloc) init(mheapLock *mutex, sysStat *uint64) {
	if levelLogPages[0] > logMaxPackedValue {
		// We can't represent 1<<levelLogPages[0] pages, the maximum number
		// of pages we need to represent at the root level, in a summary, which
		// is a big problem. Throw.
		// 在摘要中，我们不能表示1 << levelLogPages [0]页，这是我们需要在根级别上表示的最大页数，这是一个大问题。
		// 抛出异常。
		print("runtime: root level max pages = ", 1<<levelLogPages[0], "\n")
		print("runtime: summary max pages = ", maxPackedValue, "\n")
		throw("root level max pages doesn't fit in summary")
	}
	s.sysStat = sysStat

	// Initialize s.inUse.
	s.inUse.init(sysStat)

	// System-dependent initialization.
	s.sysInit()

	// Start with the searchAddr in a state indicating there's no free memory.
	// 从状态为searchAddr开始，该状态指示没有可用内存。
	s.searchAddr = maxSearchAddr

	// Start with the scavAddr in a state indicating there's nothing more to do.
	// 从scavAddr开始，该状态指示没有其他事情要做。
	s.scavAddr = minScavAddr

	// Set the mheapLock.
	s.mheapLock = mheapLock
}

// compareSearchAddrTo compares an address against s.searchAddr in a linearized
// view of the address space on systems with discontinuous process address spaces.
// This linearized view is the same one generated by chunkIndex and arenaIndex,
// done by adding arenaBaseOffset.
//
// On systems without a discontinuous address space, it's just a normal comparison.
//
// Returns < 0 if addr is less than s.searchAddr in the linearized address space.
// Returns > 0 if addr is greater than s.searchAddr in the linearized address space.
// Returns 0 if addr and s.searchAddr are equal.
//
// compareSearchAddrTo在具有不连续进程地址空间的系统上，在地址空间的线性化视图中将地址与s.searchAddr进行比较。
// 此线性化视图与通过添加arenaBaseOffset完成的chunkIndex和arenaIndex生成的视图相同。
//
// 在没有不连续地址空间的系统上，这只是正常的比较。
//
// 如果线性化地址空间中的addr小于s.searchAddr，则返回<0。
// 如果线性化地址空间中的addr大于s.searchAddr，则返回> 0。
// 如果addr和s.searchAddr相等，则返回0。
func (s *pageAlloc) compareSearchAddrTo(addr uintptr) int {
	// Compare with arenaBaseOffset added because it gives us a linear, contiguous view
	// of the heap on architectures with signed address spaces.
	// 与添加arenaBaseOffset进行比较，因为它为我们提供了具有符号地址空间的体系结构上堆的线性连续视图。
	lAddr := addr + arenaBaseOffset
	lSearchAddr := s.searchAddr + arenaBaseOffset
	if lAddr < lSearchAddr {
		return -1
	} else if lAddr > lSearchAddr {
		return 1
	}
	return 0
}

// chunkOf returns the chunk at the given chunk index.
// chunkOf返回给定块索引处的块。
func (s *pageAlloc) chunkOf(ci chunkIdx) *pallocData {
	return &s.chunks[ci.l1()][ci.l2()]
}

// grow sets up the metadata for the address range [base, base+size).
// It may allocate metadata, in which case *s.sysStat will be updated.
//
// s.mheapLock must be held.
// grow设置了地址范围[base，base + size）的元数据。
// 它可以分配元数据，在这种情况下* s.sysStat将被更新。
//
// 必须持有s.mheapLock。
func (s *pageAlloc) grow(base, size uintptr) {
	// Round up to chunks, since we can't deal with increments smaller
	// than chunks. Also, sysGrow expects aligned values.
	// 向上舍入块，因为我们无法处理小于大块的增量。此外，sysGrow期望对齐的值。
	limit := alignUp(base+size, pallocChunkBytes)
	base = alignDown(base, pallocChunkBytes)

	// Grow the summary levels in a system-dependent manner.
	// We just update a bunch of additional metadata here.
	// 以系统相关的方式增加摘要级别。我们只是在这里更新了一堆其他的元数据。
	s.sysGrow(base, limit)

	// Update s.start and s.end.
	// If no growth happened yet, start == 0. This is generally
	// safe since the zero page is unmapped.
	// 更新s.start和s.end。如果尚未发生增长，则start==0。这通常是安全的，因为未映射零页面。
	firstGrowth := s.start == 0
	start, end := chunkIndex(base), chunkIndex(limit)
	if firstGrowth || start < s.start {
		s.start = start
	}
	if end > s.end {
		s.end = end
	}
	// Note that [base, limit) will never overlap with any existing
	// range inUse because grow only ever adds never-used memory
	// regions to the page allocator.
	// 请注意，[base，limit）永远不会与inUse中的任何现有范围重叠，因为Growth仅将从未使用的内存区域添加到页面分配器中。
	s.inUse.add(addrRange{base, limit})

	// A grow operation is a lot like a free operation, so if our
	// chunk ends up below the (linearized) s.searchAddr, update
	// s.searchAddr to the new address, just like in free.
	//grow操作非常类似于free操作，因此，如果我们的代码块最终位于（线性化的）s.searchAddr以下，
	// 则将s.searchAddr更新为新地址，就像free操作一样。
	if s.compareSearchAddrTo(base) < 0 {
		s.searchAddr = base
	}

	// Add entries into chunks, which is sparse, if needed. Then,
	// initialize the bitmap.
	//
	// Newly-grown memory is always considered scavenged.
	// Set all the bits in the scavenged bitmaps high.
	// 如果需要，将条目添加到稀疏的块中。然后，初始化位图。
    //
    // 新增长的内存始终被视为清除。
    // 将清除位图中的所有位设置为high。
	for c := chunkIndex(base); c < chunkIndex(limit); c++ {
		if s.chunks[c.l1()] == nil {
			// Create the necessary l2 entry.
			//
			// Store it atomically to avoid races with readers which
			// don't acquire the heap lock.
			//
			// 创建必要的l2条目。
            //
            // 以原子方式存储它，以避免与不获取堆锁的读取器发生竞争。
			r := sysAlloc(unsafe.Sizeof(*s.chunks[0]), s.sysStat)
			atomic.StorepNoWB(unsafe.Pointer(&s.chunks[c.l1()]), r)
		}
		s.chunkOf(c).scavenged.setRange(0, pallocChunkPages)
	}

	// Update summaries accordingly. The grow acts like a free, so
	// we need to ensure this newly-free memory is visible in the
	// summaries.
	// 相应地更新摘要。grow的行为类似于free，因此我们需要确保摘要中可以看到此新释放的内存。
	s.update(base, size/pageSize, true, false)
}

// update updates heap metadata. It must be called each time the bitmap
// is updated.
//
// If contig is true, update does some optimizations assuming that there was
// a contiguous allocation or free between addr and addr+npages. alloc indicates
// whether the operation performed was an allocation or a free.
//
// s.mheapLock must be held.
//
// update更新堆元数据。每次更新位图时都必须调用它。
//
// 如果contig为true，则在addr和addr + npages之间存在连续分配或空闲的情况下，update会进行一些优化。 alloc指示执行的操作是分配还是空闲。
//
// 必须持有s.mheapLock。
func (s *pageAlloc) update(base, npages uintptr, contig, alloc bool) {
	// base, limit, start, and end are inclusive.
	// base, limit, start, 和end都是包含的
	limit := base + npages*pageSize - 1
	sc, ec := chunkIndex(base), chunkIndex(limit)

	// Handle updating the lowest level first.
	// 首先处理最低级别的更新。
	if sc == ec {
		// Fast path: the allocation doesn't span more than one chunk,
		// so update this one and if the summary didn't change, return.
		// 快速路径：分配不会跨越一个以上的块，因此请更新该块，如果摘要未更改，则返回。
		x := s.summary[len(s.summary)-1][sc]
		y := s.chunkOf(sc).summarize()
		if x == y {
			return
		}
		s.summary[len(s.summary)-1][sc] = y
	} else if contig {
		// Slow contiguous path: the allocation spans more than one chunk
		// and at least one summary is guaranteed to change.
		// 缓慢的连续路径：分配跨越一个以上的块，并且保证至少更改一个摘要。
		summary := s.summary[len(s.summary)-1]

		// Update the summary for chunk sc.
		// 更新块sc的摘要。
		summary[sc] = s.chunkOf(sc).summarize()

		// Update the summaries for chunks in between, which are
		// either totally allocated or freed.
		// 更新介于两者之间的块的摘要，这些摘要可以完全分配或释放。
		whole := s.summary[len(s.summary)-1][sc+1 : ec]
		if alloc {
			// Should optimize into a memclr.
			// 应该优化为memclr。
			for i := range whole {
				whole[i] = 0
			}
		} else {
			for i := range whole {
				whole[i] = freeChunkSum
			}
		}

		// Update the summary for chunk ec.
		// 更新块ec的摘要。
		summary[ec] = s.chunkOf(ec).summarize()
	} else {
		// Slow general path: the allocation spans more than one chunk
		// and at least one summary is guaranteed to change.
		//
		// We can't assume a contiguous allocation happened, so walk over
		// every chunk in the range and manually recompute the summary.
		// 缓慢的一般路径：分配跨越一个以上的块，并且保证至少更改一个摘要。
        //
        // 我们不能假设发生了连续分配，因此遍历范围内的每个块并手动重新计算摘要。
		summary := s.summary[len(s.summary)-1]
		for c := sc; c <= ec; c++ {
			summary[c] = s.chunkOf(c).summarize()
		}
	}

	// Walk up the radix tree and update the summaries appropriately.
	// 遍历基数树并适当地更新摘要。
	changed := true
	for l := len(s.summary) - 2; l >= 0 && changed; l-- {
		// Update summaries at level l from summaries at level l+1.
		// 从级别l+1的摘要更新级别l的摘要。
		changed = false

		// "Constants" for the previous level which we
		// need to compute the summary from that level.
		// 上一级别的“常量”，我们需要从该级别计算摘要。
		logEntriesPerBlock := levelBits[l+1]
		logMaxPages := levelLogPages[l+1]

		// lo and hi describe all the parts of the level we need to look at.
		// lo和hi描述了我们需要研究的级别的所有部分。
		lo, hi := addrsToSummaryRange(l, base, limit+1)

		// Iterate over each block, updating the corresponding summary in the less-granular level.
		// 遍历每个块，更新粒度较小的相应摘要。
		for i := lo; i < hi; i++ {
			children := s.summary[l+1][i<<logEntriesPerBlock : (i+1)<<logEntriesPerBlock]
			sum := mergeSummaries(children, logMaxPages)
			old := s.summary[l][i]
			if old != sum {
				changed = true
				s.summary[l][i] = sum
			}
		}
	}
}

// allocRange marks the range of memory [base, base+npages*pageSize) as
// allocated. It also updates the summaries to reflect the newly-updated
// bitmap.
//
// Returns the amount of scavenged memory in bytes present in the
// allocated range.
//
// s.mheapLock must be held.
//
// allocRange标记已分配的内存范围[base，base + npages * pageSize）。它还会更新摘要以反映新更新的位图。
//
// 返回分配范围中存在的清除内存量（以字节为单位）。
//
// 必须持有s.mheapLock。
func (s *pageAlloc) allocRange(base, npages uintptr) uintptr {
	limit := base + npages*pageSize - 1
	sc, ec := chunkIndex(base), chunkIndex(limit)
	si, ei := chunkPageIndex(base), chunkPageIndex(limit)

	scav := uint(0)
	if sc == ec {
		// The range doesn't cross any chunk boundaries.
		// 该范围不跨越任何块边界。
		chunk := s.chunkOf(sc)
		scav += chunk.scavenged.popcntRange(si, ei+1-si)
		chunk.allocRange(si, ei+1-si)
	} else {
		// The range crosses at least one chunk boundary.
		// 该范围至少跨越了一个块边界。
		chunk := s.chunkOf(sc)
		scav += chunk.scavenged.popcntRange(si, pallocChunkPages-si)
		chunk.allocRange(si, pallocChunkPages-si)
		for c := sc + 1; c < ec; c++ {
			chunk := s.chunkOf(c)
			scav += chunk.scavenged.popcntRange(0, pallocChunkPages)
			chunk.allocAll()
		}
		chunk = s.chunkOf(ec)
		scav += chunk.scavenged.popcntRange(0, ei+1)
		chunk.allocRange(0, ei+1)
	}
	s.update(base, npages, true, true)
	return uintptr(scav) * pageSize
}

// find searches for the first (address-ordered) contiguous free region of
// npages in size and returns a base address for that region.
//
// It uses s.searchAddr to prune its search and assumes that no palloc chunks
// below chunkIndex(s.searchAddr) contain any free memory at all.
//
// find also computes and returns a candidate s.searchAddr, which may or
// may not prune more of the address space than s.searchAddr already does.
//
// find represents the slow path and the full radix tree search.
//
// Returns a base address of 0 on failure, in which case the candidate
// searchAddr returned is invalid and must be ignored.
//
// s.mheapLock must be held.
//
// find搜索大小为npages的第一个（地址排序）连续空闲区域，并返回该区域的基址。
//
// 它使用s.searchAddr修剪其搜索，并假定chunkIndex（s.searchAddr）以下的palloc块根本不包含任何可用内存。
//
// find还计算并返回一个候选s.searchAddr，它可能会或可能不会比s.searchAddr已经修剪更多的地址空间。
//
// find表示慢速路径和完整的基数树搜索。
//
// 失败时返回基址0，在这种情况下，返回的候选searchAddr无效，必须忽略。
//
// 必须持有s.mheapLock
func (s *pageAlloc) find(npages uintptr) (uintptr, uintptr) {
	// Search algorithm.
	//
	// This algorithm walks each level l of the radix tree from the root level
	// to the leaf level. It iterates over at most 1 << levelBits[l] of entries
	// in a given level in the radix tree, and uses the summary information to
	// find either:
	//  1) That a given subtree contains a large enough contiguous region, at
	//     which point it continues iterating on the next level, or
	//  2) That there are enough contiguous boundary-crossing bits to satisfy
	//     the allocation, at which point it knows exactly where to start
	//     allocating from.
	//
	// i tracks the index into the current level l's structure for the
	// contiguous 1 << levelBits[l] entries we're actually interested in.
	//
	// NOTE: Technically this search could allocate a region which crosses
	// the arenaBaseOffset boundary, which when arenaBaseOffset != 0, is
	// a discontinuity. However, the only way this could happen is if the
	// page at the zero address is mapped, and this is impossible on
	// every system we support where arenaBaseOffset != 0. So, the
	// discontinuity is already encoded in the fact that the OS will never
	// map the zero page for us, and this function doesn't try to handle
	// this case in any way.

	// i is the beginning of the block of entries we're searching at the
	// current level.
	//
	// 搜索算法。
    //
    // 此算法将基数树的每个级别l从根级别移动到叶子级别。迭代基数树中给定级别的最多1 << levelBits[l]个条目，
    // 并使用摘要信息查找以下任一个：
    // 1）给定的子树包含足够大的连续区域，这时它将继续在下一级进行迭代，或者
    // 2）有足够的连续边界交叉位来满足分配，此时它确切地知道从哪里开始分配。
    //
    // i跟踪我们实际感兴趣的连续1 << levelBits [l]条目的当前级别l结构的索引。
    //
    // 注意：从技术上讲，此搜索可以分配一个跨越arenaBaseOffset边界的区域，当arenaBaseOffset！= 0时，该区域是不连续的。
    // 但是，发生这种情况的唯一方法是映射零地址处的页面，并且这在我们支持arenaBaseOffset！= 0的每个系统上都是不可能的。
    // 因此，不连续性已经被编码，因为OS将永远不会映射零页面，并且此函数不会尝试以任何方式处理这种情况。

    // i是我们正在当前级别搜索的条目块的开头。
	i := 0

	// firstFree is the region of address space that we are certain to
	// find the first free page in the heap. base and bound are the inclusive
	// bounds of this window, and both are addresses in the linearized, contiguous
	// view of the address space (with arenaBaseOffset pre-added). At each level,
	// this window is narrowed as we find the memory region containing the
	// first free page of memory. To begin with, the range reflects the
	// full process address space.
	//
	// firstFree is updated by calling foundFree each time free space in the
	// heap is discovered.
	//
	// At the end of the search, base-arenaBaseOffset is the best new
	// searchAddr we could deduce in this search.
	//
	// firstFree是我们肯定会在堆中找到第一个空闲页的地址空间区域。 base和bound是此窗口的包含边界，
	// 并且都是地址空间的线性连续视图中的地址（预添加了arenaBaseOffset）。在每个级别上，
	// 随着我们发现包含内存的第一个空闲页的内存区域，此窗口都会缩小。首先，该范围反映了整个进程地址空间。
    //
    // 每当发现堆中的可用空间时，通过调用foundFree来更新firstFree。
    //
    // 在搜索结束时，base-arenaBaseOffset是我们可以在此搜索中得出的最佳新searchAddr。
	firstFree := struct {
		base, bound uintptr
	}{
		base:  0,
		bound: (1<<heapAddrBits - 1),
	}
	// foundFree takes the given address range [addr, addr+size) and
	// updates firstFree if it is a narrower range. The input range must
	// either be fully contained within firstFree or not overlap with it
	// at all.
	//
	// This way, we'll record the first summary we find with any free
	// pages on the root level and narrow that down if we descend into
	// that summary. But as soon as we need to iterate beyond that summary
	// in a level to find a large enough range, we'll stop narrowing.
	//
	// foundFree获取给定的地址范围[addr，addr + size），如果范围较小，则更新firstFree。
	// 输入范围必须完全包含在firstFree内或完全不与它重叠。
    //
    // 这样，我们将记录找到的第一个摘要，并在根目录上包含所有可用页面，如果我们进入该摘要，则将其缩小。
    // 但是，只要我们需要在某个摘要上进行迭代以找到足够大的范围，我们就会停止缩小范围。
	foundFree := func(addr, size uintptr) {
		if firstFree.base <= addr && addr+size-1 <= firstFree.bound {
			// This range fits within the current firstFree window, so narrow
			// down the firstFree window to the base and bound of this range.
			// 此范围适合当前的firstFree窗口，因此将firstFree窗口缩小到该范围的底限和边界。
			firstFree.base = addr
			firstFree.bound = addr + size - 1
		} else if !(addr+size-1 < firstFree.base || addr > firstFree.bound) {
			// This range only partially overlaps with the firstFree range,
			// so throw.
			// 此范围仅与firstFree范围部分重叠，因此请抛出异常。
			print("runtime: addr = ", hex(addr), ", size = ", size, "\n")
			print("runtime: base = ", hex(firstFree.base), ", bound = ", hex(firstFree.bound), "\n")
			throw("range partially overlaps")
		}
	}

	// lastSum is the summary which we saw on the previous level that made us
	// move on to the next level. Used to print additional information in the
	// case of a catastrophic failure.
	// lastSumIdx is that summary's index in the previous level.
	// lastSum是上一级别中看到的摘要，使我们可以进入下一个级别。发生灾难性故障时，用于打印其他信息。
	// lastSumIdx是该摘要在上一级中的索引。
	lastSum := packPallocSum(0, 0, 0)
	lastSumIdx := -1

nextLevel:
	for l := 0; l < len(s.summary); l++ {
		// For the root level, entriesPerBlock is the whole level.
		// 对于根级别，entrysPerBlock是整个级别。
		entriesPerBlock := 1 << levelBits[l]
		logMaxPages := levelLogPages[l]

		// We've moved into a new level, so let's update i to our new
		// starting index. This is a no-op for level 0.
		// 我们已经进入了一个新的高度，所以让我们将i更新为新的起始索引。这是0级的无操作。
		i <<= levelBits[l]

		// Slice out the block of entries we care about.
		// 切出我们关心的条目块。
		entries := s.summary[l][i : i+entriesPerBlock]

		// Determine j0, the first index we should start iterating from.
		// The searchAddr may help us eliminate iterations if we followed the
		// searchAddr on the previous level or we're on the root leve, in which
		// case the searchAddr should be the same as i after levelShift.
		// 确定j0，这是我们应该从其开始迭代的第一个索引。
        // 如果我们在上一级遵循searchAddr或位于根目录上，则searchAddr可以帮助我们消除迭代，
        // 在这种情况下，searchAddr应该与levelShift之后的i相同。
		j0 := 0
		if searchIdx := int((s.searchAddr + arenaBaseOffset) >> levelShift[l]); searchIdx&^(entriesPerBlock-1) == i {
			j0 = searchIdx & (entriesPerBlock - 1)
		}

		// Run over the level entries looking for
		// a contiguous run of at least npages either
		// within an entry or across entries.
		//
		// base contains the page index (relative to
		// the first entry's first page) of the currently
		// considered run of consecutive pages.
		//
		// size contains the size of the currently considered
		// run of consecutive pages.
		//
		// 在级别条目上运行，以查找条目内或条目间至少npages的连续内存。
        //
        // base包含当前考虑的连续页面运行的页面索引（相对于第一条目的第一页）。
        //
        // size包含当前考虑的连续页面的大小。
		var base, size uint
		for j := j0; j < len(entries); j++ {
			sum := entries[j]
			if sum == 0 {
				// A full entry means we broke any streak and
				// that we should skip it altogether.
				// 完整的条目意味着我们打破了任何streak，应该完全跳过。
				size = 0
				continue
			}

			// We've encountered a non-zero summary which means
			// free memory, so update firstFree.
			// 我们遇到了一个非零的摘要，这意味着有可用内存，因此请更新firstFree。
			foundFree(uintptr((i+j)<<levelShift[l]), (uintptr(1)<<logMaxPages)*pageSize)

			s := sum.start()
			if size+s >= uint(npages) {
				// If size == 0 we don't have a run yet,
				// which means base isn't valid. So, set
				// base to the first page in this block.
				// 如果size == 0，则我们还没有运行，这意味着基数无效。因此，将base设置为该块的第一页。
				if size == 0 {
					base = uint(j) << logMaxPages
				}
				// We hit npages; we're done!
				// 我们找到了npages；我们完成了！
				size += s
				break
			}
			if sum.max() >= uint(npages) {
				// The entry itself contains npages contiguous
				// free pages, so continue on the next level
				// to find that run.
				// 条目本身包含npages个连续的空闲页面，因此请继续进行下一级查找该内存。
				i += j
				lastSumIdx = i
				lastSum = sum
				continue nextLevel
			}
			if size == 0 || s < 1<<logMaxPages {
				// We either don't have a current run started, or this entry
				// isn't totally free (meaning we can't continue the current
				// one), so try to begin a new run by setting size and base
				// based on sum.end.
				// 我们没有开始当前运行，或者该条目不是完全空闲的（意味着我们无法继续当前运行），
				// 因此请尝试通过基于sum.end设置大小和基准来开始新运行。
				size = sum.end()
				base = uint(j+1)<<logMaxPages - size
				continue
			}
			// The entry is completely free, so continue the run.
			// 该条目是完全免费的，因此继续运行。
			size += 1 << logMaxPages
		}
		if size >= uint(npages) {
			// We found a sufficiently large run of free pages straddling
			// some boundary, so compute the address and return it.
			// 我们发现有足够多的空闲页面跨越某些边界，因此请计算地址并返回它。
			addr := uintptr(i<<levelShift[l]) - arenaBaseOffset + uintptr(base)*pageSize
			return addr, firstFree.base - arenaBaseOffset
		}
		if l == 0 {
			// We're at level zero, so that means we've exhausted our search.
			// 我们处于零级，这意味着我们已经用尽了所有搜索。
			return 0, maxSearchAddr
		}

		// We're not at level zero, and we exhausted the level we were looking in.
		// This means that either our calculations were wrong or the level above
		// lied to us. In either case, dump some useful state and throw.
		// 我们还没有达到零级，我们已经用尽了所寻找的级别。这意味着我们的计算错误或高于我们的级别。
		// 无论哪种情况，都转储一些有用的状态并抛出。
		print("runtime: summary[", l-1, "][", lastSumIdx, "] = ", lastSum.start(), ", ", lastSum.max(), ", ", lastSum.end(), "\n")
		print("runtime: level = ", l, ", npages = ", npages, ", j0 = ", j0, "\n")
		print("runtime: s.searchAddr = ", hex(s.searchAddr), ", i = ", i, "\n")
		print("runtime: levelShift[level] = ", levelShift[l], ", levelBits[level] = ", levelBits[l], "\n")
		for j := 0; j < len(entries); j++ {
			sum := entries[j]
			print("runtime: summary[", l, "][", i+j, "] = (", sum.start(), ", ", sum.max(), ", ", sum.end(), ")\n")
		}
		throw("bad summary data")
	}

	// Since we've gotten to this point, that means we haven't found a
	// sufficiently-sized free region straddling some boundary (chunk or larger).
	// This means the last summary we inspected must have had a large enough "max"
	// value, so look inside the chunk to find a suitable run.
	//
	// After iterating over all levels, i must contain a chunk index which
	// is what the final level represents.
	// 既然到了这一点，那意味着我们还没有找到一个足够大的，跨越某些边界（块或更大）的空闲区域。
    // 这意味着我们检查的最后一个摘要必须具有足够大的“最大值”值，因此请在块中查找合适的内存。
    //
    // 遍历所有级别后，我必须包含一个块索引，这是最终级别表示的内容。
	ci := chunkIdx(i)
	j, searchIdx := s.chunkOf(ci).find(npages, 0)
	if j < 0 {
		// We couldn't find any space in this chunk despite the summaries telling
		// us it should be there. There's likely a bug, so dump some state and throw.
		// 尽管摘要告诉我们应该在其中，但我们在该块中找不到任何空间。可能存在错误，因此请转储一些状态并抛出错误。
		sum := s.summary[len(s.summary)-1][i]
		print("runtime: summary[", len(s.summary)-1, "][", i, "] = (", sum.start(), ", ", sum.max(), ", ", sum.end(), ")\n")
		print("runtime: npages = ", npages, "\n")
		throw("bad summary data")
	}

	// Compute the address at which the free space starts.
	// 计算可用空间开始的地址。
	addr := chunkBase(ci) + uintptr(j)*pageSize

	// Since we actually searched the chunk, we may have
	// found an even narrower free window.
	// 由于实际上我们搜索了该块，因此我们可能找到了更窄的空闲窗口。
	searchAddr := chunkBase(ci) + uintptr(searchIdx)*pageSize
	foundFree(searchAddr+arenaBaseOffset, chunkBase(ci+1)-searchAddr)
	return addr, firstFree.base - arenaBaseOffset
}

// alloc allocates npages worth of memory from the page heap, returning the base
// address for the allocation and the amount of scavenged memory in bytes
// contained in the region [base address, base address + npages*pageSize).
//
// Returns a 0 base address on failure, in which case other returned values
// should be ignored.
//
// s.mheapLock must be held.
//
// alloc从页堆中分配npages的内存，返回分配的基址和区域中包含的字节数的已清除内存量[base address，base address + npages * pageSize）。
//
// 失败时返回0基址，在这种情况下，其他返回值应忽略。
//
// 必须持有s.mheapLock。
func (s *pageAlloc) alloc(npages uintptr) (addr uintptr, scav uintptr) {
	// If the searchAddr refers to a region which has a higher address than
	// any known chunk, then we know we're out of memory.
	// 如果searchAddr所指向的区域的地址比任何已知块的地址高，则表明我们内存不足。
	if chunkIndex(s.searchAddr) >= s.end {
		return 0, 0
	}

	// If npages has a chance of fitting in the chunk where the searchAddr is,
	// search it directly.
	// 如果npages可能适合searchAddr所在的块，请直接搜索它。
	searchAddr := uintptr(0)
	if pallocChunkPages-chunkPageIndex(s.searchAddr) >= uint(npages) {
		// npages is guaranteed to be no greater than pallocChunkPages here.
		// 这里保证npages不大于pallocChunkPages。
		i := chunkIndex(s.searchAddr)
		if max := s.summary[len(s.summary)-1][i].max(); max >= uint(npages) {
			j, searchIdx := s.chunkOf(i).find(npages, chunkPageIndex(s.searchAddr))
			if j < 0 {
				print("runtime: max = ", max, ", npages = ", npages, "\n")
				print("runtime: searchIdx = ", chunkPageIndex(s.searchAddr), ", s.searchAddr = ", hex(s.searchAddr), "\n")
				throw("bad summary data")
			}
			addr = chunkBase(i) + uintptr(j)*pageSize
			searchAddr = chunkBase(i) + uintptr(searchIdx)*pageSize
			goto Found
		}
	}
	// We failed to use a searchAddr for one reason or another, so try
	// the slow path.
	// 由于某种原因，我们未能使用searchAddr，因此请尝试慢速路径。
	addr, searchAddr = s.find(npages)
	if addr == 0 {
		if npages == 1 {
			// We failed to find a single free page, the smallest unit
			// of allocation. This means we know the heap is completely
			// exhausted. Otherwise, the heap still might have free
			// space in it, just not enough contiguous space to
			// accommodate npages.
			// 我们找不到单个空闲页面，即最小的分配单元。这意味着我们知道堆已完全耗尽。
			// 否则，堆中可能仍然有可用空间，只是没有足够的连续空间来容纳npage。
			s.searchAddr = maxSearchAddr
		}
		return 0, 0
	}
Found:
	// Go ahead and actually mark the bits now that we have an address.
	// 现在，我们有了地址，继续实际标记这些位。
	scav = s.allocRange(addr, npages)

	// If we found a higher (linearized) searchAddr, we know that all the
	// heap memory before that searchAddr in a linear address space is
	// allocated, so bump s.searchAddr up to the new one.
	// 如果我们找到了一个更高的（线性化的）searchAddr，我们知道线性地址空间中该searchAddr之前的所有堆内存都已分配，
	// 因此将s.searchAddr扩展到新的。
	if s.compareSearchAddrTo(searchAddr) > 0 {
		s.searchAddr = searchAddr
	}
	return addr, scav
}

// free returns npages worth of memory starting at base back to the page heap.
//
// s.mheapLock must be held.
func (s *pageAlloc) free(base, npages uintptr) {
	// If we're freeing pages below the (linearized) s.searchAddr, update searchAddr.
	if s.compareSearchAddrTo(base) < 0 {
		s.searchAddr = base
	}
	if npages == 1 {
		// Fast path: we're clearing a single bit, and we know exactly
		// where it is, so mark it directly.
		i := chunkIndex(base)
		s.chunkOf(i).free1(chunkPageIndex(base))
	} else {
		// Slow path: we're clearing more bits so we may need to iterate.
		limit := base + npages*pageSize - 1
		sc, ec := chunkIndex(base), chunkIndex(limit)
		si, ei := chunkPageIndex(base), chunkPageIndex(limit)

		if sc == ec {
			// The range doesn't cross any chunk boundaries.
			s.chunkOf(sc).free(si, ei+1-si)
		} else {
			// The range crosses at least one chunk boundary.
			s.chunkOf(sc).free(si, pallocChunkPages-si)
			for c := sc + 1; c < ec; c++ {
				s.chunkOf(c).freeAll()
			}
			s.chunkOf(ec).free(0, ei+1)
		}
	}
	s.update(base, npages, true, false)
}

const (
	pallocSumBytes = unsafe.Sizeof(pallocSum(0))

	// maxPackedValue is the maximum value that any of the three fields in
	// the pallocSum may take on.
	maxPackedValue    = 1 << logMaxPackedValue
	logMaxPackedValue = logPallocChunkPages + (summaryLevels-1)*summaryLevelBits

	freeChunkSum = pallocSum(uint64(pallocChunkPages) |
		uint64(pallocChunkPages<<logMaxPackedValue) |
		uint64(pallocChunkPages<<(2*logMaxPackedValue)))
)

// pallocSum is a packed summary type which packs three numbers: start, max,
// and end into a single 8-byte value. Each of these values are a summary of
// a bitmap and are thus counts, each of which may have a maximum value of
// 2^21 - 1, or all three may be equal to 2^21. The latter case is represented
// by just setting the 64th bit.
type pallocSum uint64

// packPallocSum takes a start, max, and end value and produces a pallocSum.
func packPallocSum(start, max, end uint) pallocSum {
	if max == maxPackedValue {
		return pallocSum(uint64(1 << 63))
	}
	return pallocSum((uint64(start) & (maxPackedValue - 1)) |
		((uint64(max) & (maxPackedValue - 1)) << logMaxPackedValue) |
		((uint64(end) & (maxPackedValue - 1)) << (2 * logMaxPackedValue)))
}

// start extracts the start value from a packed sum.
func (p pallocSum) start() uint {
	if uint64(p)&uint64(1<<63) != 0 {
		return maxPackedValue
	}
	return uint(uint64(p) & (maxPackedValue - 1))
}

// max extracts the max value from a packed sum.
func (p pallocSum) max() uint {
	if uint64(p)&uint64(1<<63) != 0 {
		return maxPackedValue
	}
	return uint((uint64(p) >> logMaxPackedValue) & (maxPackedValue - 1))
}

// end extracts the end value from a packed sum.
func (p pallocSum) end() uint {
	if uint64(p)&uint64(1<<63) != 0 {
		return maxPackedValue
	}
	return uint((uint64(p) >> (2 * logMaxPackedValue)) & (maxPackedValue - 1))
}

// unpack unpacks all three values from the summary.
func (p pallocSum) unpack() (uint, uint, uint) {
	if uint64(p)&uint64(1<<63) != 0 {
		return maxPackedValue, maxPackedValue, maxPackedValue
	}
	return uint(uint64(p) & (maxPackedValue - 1)),
		uint((uint64(p) >> logMaxPackedValue) & (maxPackedValue - 1)),
		uint((uint64(p) >> (2 * logMaxPackedValue)) & (maxPackedValue - 1))
}

// mergeSummaries merges consecutive summaries which may each represent at
// most 1 << logMaxPagesPerSum pages each together into one.
func mergeSummaries(sums []pallocSum, logMaxPagesPerSum uint) pallocSum {
	// Merge the summaries in sums into one.
	//
	// We do this by keeping a running summary representing the merged
	// summaries of sums[:i] in start, max, and end.
	start, max, end := sums[0].unpack()
	for i := 1; i < len(sums); i++ {
		// Merge in sums[i].
		si, mi, ei := sums[i].unpack()

		// Merge in sums[i].start only if the running summary is
		// completely free, otherwise this summary's start
		// plays no role in the combined sum.
		if start == uint(i)<<logMaxPagesPerSum {
			start += si
		}

		// Recompute the max value of the running sum by looking
		// across the boundary between the running sum and sums[i]
		// and at the max sums[i], taking the greatest of those two
		// and the max of the running sum.
		if end+si > max {
			max = end + si
		}
		if mi > max {
			max = mi
		}

		// Merge in end by checking if this new summary is totally
		// free. If it is, then we want to extend the running sum's
		// end by the new summary. If not, then we have some alloc'd
		// pages in there and we just want to take the end value in
		// sums[i].
		if ei == 1<<logMaxPagesPerSum {
			end += 1 << logMaxPagesPerSum
		} else {
			end = ei
		}
	}
	return packPallocSum(start, max, end)
}
```