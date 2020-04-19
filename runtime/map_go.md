
```go
package runtime

// This file contains the implementation of Go's map type.
//
// A map is just a hash table. The data is arranged
// into an array of buckets. Each bucket contains up to
// 8 key/elem pairs. The low-order bits of the hash are
// used to select a bucket. Each bucket contains a few
// high-order bits of each hash to distinguish the entries
// within a single bucket.
//
// If more than 8 keys hash to a bucket, we chain on
// extra buckets.
//
// When the hashtable grows, we allocate a new array
// of buckets twice as big. Buckets are incrementally
// copied from the old bucket array to the new bucket array.
//
// Map iterators walk through the array of buckets and
// return the keys in walk order (bucket #, then overflow
// chain order, then bucket index).  To maintain iteration
// semantics, we never move keys within their bucket (if
// we did, keys might be returned 0 or 2 times).  When
// growing the table, iterators remain iterating through the
// old table and must check the new table if the bucket
// they are iterating through has been moved ("evacuated")
// to the new table.

// Picking loadFactor: too large and we have lots of overflow
// buckets, too small and we waste a lot of space. I wrote
// a simple program to check some stats for different loads:
// (64-bit, 8 byte keys and elems)
//  loadFactor    %overflow  bytes/entry     hitprobe    missprobe
//        4.00         2.13        20.77         3.00         4.00
//        4.50         4.05        17.30         3.25         4.50
//        5.00         6.85        14.77         3.50         5.00
//        5.50        10.55        12.94         3.75         5.50
//        6.00        15.27        11.67         4.00         6.00
//        6.50        20.90        10.79         4.25         6.50
//        7.00        27.14        10.15         4.50         7.00
//        7.50        34.03         9.73         4.75         7.50
//        8.00        41.10         9.40         5.00         8.00
//
// %overflow   = percentage of buckets which have an overflow bucket
// bytes/entry = overhead bytes used per key/elem pair
// hitprobe    = # of entries to check when looking up a present key
// missprobe   = # of entries to check when looking up an absent key
//
// Keep in mind this data is for maximally loaded tables, i.e. just
// before the table grows. Typical tables will be somewhat less loaded.
/**
 * map.go文件包含Go的映射类型的实现。
 *
 * 映射只是一个哈希表。数据被安排在一系列存储桶中。每个存储桶最多包含8个键/元素对。
 * 哈希的低位用于选择存储桶。每个存储桶包含每个哈希的一些高阶位，以区分单个存储桶中的条目。
 *
 * 如果有8个以上的键散列到存储桶中，则我们会链接到其他存储桶。
 *
 * 当散列表增加时，我们将分配一个两倍大数组作为新的存储桶。
 * 将存储桶以增量方式从旧存储桶阵列复制到新存储桶阵列。
 *
 * 映射迭代器遍历存储桶数组，并按遍历顺序返回键（存储桶#，然后是溢出链顺序，然后是存储桶索引）。
 * 为了维持迭代语义，我们绝不会在键的存储桶中移动键（如果这样做，键可能会返回0或2次）。
 * 在扩展表时，迭代器将保持对旧表的迭代，并且必须检查新表是否将要迭代的存储桶（“撤离”）到新表中。
 *
 * 选择loadFactor：太大了，我们有很多溢出桶，太小了，我们浪费了很多空间。
 * 我编写了一个简单的程序来检查一些不同负载的统计信息：（64位，8字节键和值）
 *   loadFactor    %overflow  bytes/entry     hitprobe    missprobe
 *         4.00         2.13        20.77         3.00         4.00
 *         4.50         4.05        17.30         3.25         4.50
 *         5.00         6.85        14.77         3.50         5.00
 *         5.50        10.55        12.94         3.75         5.50
 *         6.00        15.27        11.67         4.00         6.00
 *         6.50        20.90        10.79         4.25         6.50
 *         7.00        27.14        10.15         4.50         7.00
 *         7.50        34.03         9.73         4.75         7.50
 *         8.00        41.10         9.40         5.00         8.00
 * %overflow = 具有溢出桶的桶的百分比
 * bytes/entry = 每个键值对使用的字节数
 * hitprobe = 查找存在的key时要检查的条目数
 * missprobe = 查找不存在的key要检查的条目数
 *
 * 请记住，此数据用于最大加载的表，即表增长之前。 典型的表将少加载。
 **/
import (
	"runtime/internal/atomic"
	"runtime/internal/math"
	"runtime/internal/sys"
	"unsafe"
)

const (
	// Maximum number of key/elem pairs a bucket can hold.
	// 桶可以容纳的最大键/值对数量。
	bucketCntBits = 3
	bucketCnt     = 1 << bucketCntBits

	// Maximum average load of a bucket that triggers growth is 6.5.
	// Represent as loadFactorNum/loadFactDen, to allow integer math.
	// 触发增长的存储桶的最大平均负载为6.5。表示为loadFactorNum/loadFactDen，以允许整数数学运算。
	loadFactorNum = 13
	loadFactorDen = 2

	// Maximum key or elem size to keep inline (instead of mallocing per element).
	// Must fit in a uint8.
	// Fast versions cannot handle big elems - the cutoff size for
	// fast versions in cmd/compile/internal/gc/walk.go must be at most this elem.
	// 保持内联的最大键或elem大小（而不是每个元素的malloc分配）。
    // 必须适合uint8。
    // 快速版本不能处理大问题 - cmd/compile/internal/gc/walk.go中快速版本的临界大小最多必须是这个元素。
	maxKeySize  = 128
	maxElemSize = 128

	// data offset should be the size of the bmap struct, but needs to be
	// aligned correctly. For amd64p32 this means 64-bit alignment
	// even though pointers are 32 bit.
	// 数据偏移量应为bmap结构的大小，但需要正确对齐。对于amd64p32，
	// 即使指针是32位，这也意味着64位对齐。
	dataOffset = unsafe.Offsetof(struct {
		b bmap
		v int64
	}{}.v)

	// Possible tophash values. We reserve a few possibilities for special marks.
	// Each bucket (including its overflow buckets, if any) will have either all or none of its
	// entries in the evacuated* states (except during the evacuate() method, which only happens
	// during map writes and thus no one else can observe the map during that time).
	// 可能的tophash值。我们为特殊标记保留一些可能性。
    // 每个存储桶（包括其溢出存储桶，如果有的话）在evacuated*状态下将具有全部或没有条目
    //（除了evacuate()方法期间，该方法仅在映射写入期间发生，因此在此期间没有其他人可以观察该映射）。
    // 此单元格是空的，并且不再有更高索引或溢出的非空单元格。
	emptyRest      = 0 // this cell is empty, and there are no more non-empty cells at higher indexes or overflows.
	// 这个单元格是空的
	emptyOne       = 1 // this cell is empty
	// 键/元素有效。条目已被分散到大表的前半部分。
	evacuatedX     = 2 // key/elem is valid.  Entry has been evacuated to first half of larger table.
	// 与上述相同，但分散到大表的后半部分。
	evacuatedY     = 3 // same as above, but evacuated to second half of larger table.
	// 单元格是空的，桶已已经被分散。
	evacuatedEmpty = 4 // cell is empty, bucket is evacuated.
	// 一个正常填充的单元格的最小tophash
	minTopHash     = 5 // minimum tophash for a normal filled cell.

	// flags
	// 标志位
	iterator     = 1 // there may be an iterator using buckets // 可能有一个使用桶的迭代器
	oldIterator  = 2 // there may be an iterator using oldbuckets // 可能有一个使用oldbuckets的迭代器
	hashWriting  = 4 // a goroutine is writing to the map // 一个goroutine正在写映射
	sameSizeGrow = 8 // the current map growth is to a new map of the same size // 当前的映射增长是到一个相同大小的新映射

	// sentinel bucket ID for iterator checks
	// 用于迭代器检查的哨兵桶ID
	noCheck = 1<<(8*sys.PtrSize) - 1
)

// isEmpty reports whether the given tophash array entry represents an empty bucket entry.
/**
 * isEmpty报告给定的tophash数组条目是否表示一个空存储桶条目。
 * @param x bmpa中tophash数组中的元素
 * @return
 **/
func isEmpty(x uint8) bool {
	return x <= emptyOne
}

// A header for a Go map.
/**
 * go map的头部
 **/
type hmap struct {
	// Note: the format of the hmap is also encoded in cmd/compile/internal/gc/reflect.go.
	// Make sure this stays in sync with the compiler's definition.
	// 注意：hmap的格式也编码在cmd/compile/internal/gc/reflect.go中。确保这与编译器的定义保持同步。
	// #存活元素==映射的大小。必须是第一个（内置len（）使用）
	count     int // # live cells == size of map.  Must be first (used by len() builtin)
	flags     uint8
	// 桶数的log_2（最多可容纳loadFactor * 2 ^ B个项目）
	B         uint8  // log_2 of # of buckets (can hold up to loadFactor * 2^B items)
	// 溢出桶的大概数量；有关详细信息，请参见incrnoverflow
	noverflow uint16 // approximate number of overflow buckets; see incrnoverflow for details
	// 哈希种子
	hash0     uint32 // hash seed

    // 2^B个桶的数组。如果count == 0，则可能为nil。
	buckets    unsafe.Pointer // array of 2^B Buckets. may be nil if count==0.
	// 上一存储桶数组，只有当前桶的一半大小，只有在增长时才为非nil
	oldbuckets unsafe.Pointer // previous bucket array of half the size, non-nil only when growing
	// 迁移进度计数器（小于此的桶表明已被迁移）
	nevacuate  uintptr        // progress counter for evacuation (buckets less than this have been evacuated)

    // 可选择字段
	extra *mapextra // optional fields
}

// mapextra holds fields that are not present on all maps.
/**
 * mapextra包含并非在所有map上都存在的字段。
 **/
type mapextra struct {
	// If both key and elem do not contain pointers and are inline, then we mark bucket
	// type as containing no pointers. This avoids scanning such maps.
	// However, bmap.overflow is a pointer. In order to keep overflow buckets
	// alive, we store pointers to all overflow buckets in hmap.extra.overflow and hmap.extra.oldoverflow.
	// overflow and oldoverflow are only used if key and elem do not contain pointers.
	// overflow contains overflow buckets for hmap.buckets.
	// oldoverflow contains overflow buckets for hmap.oldbuckets.
	// The indirection allows to store a pointer to the slice in hiter.
	// 如果key和elem都不包含指针并且是内联的，则我们将存储桶类型标记为不包含指针。这样可以避免扫描此类映射。
    // 但是，bmap.overflow是一个指针。为了使溢出桶保持活动状态，我们将指向所有溢出桶的指针存储在hmap.extra.overflow
    // 和hmap.extra.oldoverflow中。仅当key和elem不包含指针时，才使用overflow和oldoverflow。
    // overflow包含hmap.buckets的溢出桶。 oldoverflow包含hmap.oldbuckets的溢出存储桶。
    // 间接允许在Hiter中存储指向切片的指针。
	overflow    *[]*bmap
	oldoverflow *[]*bmap

	// nextOverflow holds a pointer to a free overflow bucket.
	// nextOverflow拥有一个指向空闲溢出桶的指针。
	nextOverflow *bmap
}

// A bucket for a Go map.
/**
 * go映射的桶结构
 **/
type bmap struct {
	// tophash generally contains the top byte of the hash value
	// for each key in this bucket. If tophash[0] < minTopHash,
	// tophash[0] is a bucket evacuation state instead.
	// tophash通常包含此存储桶中每个键的哈希值的最高字节。如果tophash[0] < minTopHash，
	// 则tophash[0]是桶迁移状态。
	tophash [bucketCnt]uint8
	// Followed by bucketCnt keys and then bucketCnt elems.
	// NOTE: packing all the keys together and then all the elems together makes the
	// code a bit more complicated than alternating key/elem/key/elem/... but it allows
	// us to eliminate padding which would be needed for, e.g., map[int64]int8.
	// Followed by an overflow pointer.
	// 随后是bucketCnt键，再后是bucketCnt元素。
    // 注意：将所有键打包在一起，然后将所有elems打包在一起，使代码比交替key/elem/key/elem/...复杂一些，
    // 但是它使我们可以省去填充，例如，映射[int64] int8。后跟一个溢出指针。
}

// A hash iteration structure.
// If you modify hiter, also change cmd/compile/internal/gc/reflect.go to indicate
// the layout of this structure.
/**
 * 哈希迭代结构。
 * 如果修改了hiter，还请更改cmd/compile/internal/gc/reflect.go来指示此结构的布局。
 **/
type hiter struct {
    // 必须处于第一位置。写nil表示迭代结束（请参阅cmd/internal/gc/range.go）。
	key         unsafe.Pointer // Must be in first position.  Write nil to indicate iteration end (see cmd/internal/gc/range.go).
	// 必须位于第二位置（请参阅cmd/internal/gc/range.go）。
	elem        unsafe.Pointer // Must be in second position (see cmd/internal/gc/range.go).
	t           *maptype // map类型
	h           *hmap
	// hash_iter初始化时的bucket指针
	buckets     unsafe.Pointer // bucket ptr at hash_iter initialization time
	// current bucket
	bptr        *bmap          // current bucket
	// 使hmap.buckets溢出桶保持活动状态
	overflow    *[]*bmap       // keeps overflow buckets of hmap.buckets alive
	// 使hmap.oldbuckets溢出桶保持活动状态
	oldoverflow *[]*bmap       // keeps overflow buckets of hmap.oldbuckets alive
	// 存储桶迭代始于指针位置
	startBucket uintptr        // bucket iteration started at
	// 从迭代期间开始的桶内距离start位置的偏移量（应该足够大以容纳bucketCnt-1）
	offset      uint8          // intra-bucket offset to start from during iteration (should be big enough to hold bucketCnt-1)
	// 已经从存储桶数组的末尾到开头缠绕了
	wrapped     bool           // already wrapped around from end of bucket array to beginning
	B           uint8
	i           uint8
	bucket      uintptr
	checkBucket uintptr
}

// bucketShift returns 1<<b, optimized for code generation.
/**
 * bucketShift返回1<<b，已针对代码生成进行了优化。
 * @param
 * @return
 **/
func bucketShift(b uint8) uintptr {
	// Masking the shift amount allows overflow checks to be elided.
	// 掩盖移位量可以消除溢出检查。
	return uintptr(1) << (b & (sys.PtrSize*8 - 1))
}

// bucketMask returns 1<<b - 1, optimized for code generation.
/**
 * bucketMask返回1<<b - 1，已针对代码生成进行了优化。
 * @param
 * @return
 **/
func bucketMask(b uint8) uintptr {
	return bucketShift(b) - 1
}

// tophash calculates the tophash value for hash.
/**
 * tophash计算哈希的tophash值。
 * @param
 * @return
 **/
func tophash(hash uintptr) uint8 {
    // sys.PtrSize为4或者8
    // 取最高位字节
	top := uint8(hash >> (sys.PtrSize*8 - 8))
	if top < minTopHash {
		top += minTopHash
	}
	return top
}

/**
 * 判断b是否被迁移新的map中
 * @param
 * @return
 **/
func evacuated(b *bmap) bool {
	h := b.tophash[0]
	return h > emptyOne && h < minTopHash
}

/**
 * TODO
 **/
func (b *bmap) overflow(t *maptype) *bmap {
	return *(**bmap)(add(unsafe.Pointer(b), uintptr(t.bucketsize)-sys.PtrSize))
}

/**
 * 设置b的溢出bmap指针
 * @param t map类型指针
 * @param 溢出bmap指针
 * @return
 **/
func (b *bmap) setoverflow(t *maptype, ovf *bmap) {
	*(**bmap)(add(unsafe.Pointer(b), uintptr(t.bucketsize)-sys.PtrSize)) = ovf
}

/**
 * 获取b中的所有key的起始指针
 * @param
 * @return
 **/
func (b *bmap) keys() unsafe.Pointer {
	return add(unsafe.Pointer(b), dataOffset)
}

// incrnoverflow increments h.noverflow.
// noverflow counts the number of overflow buckets.
// This is used to trigger same-size map growth.
// See also tooManyOverflowBuckets.
// To keep hmap small, noverflow is a uint16.
// When there are few buckets, noverflow is an exact count.
// When there are many buckets, noverflow is an approximate count.
/**
 * incrnoverflow递增h.noverflow。
 * noverflow计算溢出桶的数量。
 * 这用于触发相同大小的map增长。
 * 另请参见tooManyOverflowBuckets。
 * 为了使hmap保持较小，noverflow是一个uint16。
 * 当存储桶很少时，noverflow是一个精确的计数。
 * 如果有很多存储桶，则noverflow是一个近似计数。
 * @param
 * @return
 **/
func (h *hmap) incrnoverflow() {
	// We trigger same-size map growth if there are
	// as many overflow buckets as buckets.
	// We need to be able to count to 1<<h.B.
	// 如果溢出存储桶的数量与存储桶的数量相同，则会触发相同尺寸的map增长。
    // 我们需要能够计数到1<<h.B。
	if h.B < 16 { // 说是map中的元素比较少，少于（2^h.B）个
		h.noverflow++
		return
	}
	// Increment with probability 1/(1<<(h.B-15)).
	// When we reach 1<<15 - 1, we will have approximately
	// as many overflow buckets as buckets.
	// 以概率1/(1<<(h.B-15))递增。
    //当我们达到1 << 15-1时，我们将拥有大约与桶一样多的溢出桶。
	mask := uint32(1)<<(h.B-15) - 1
	// Example: if h.B == 18, then mask == 7,
	// and fastrand & 7 == 0 with probability 1/8.
	// 例如：如果h.B == 18，则mask == 7，fastrand&7 == 0，概率为1/8。
	if fastrand()&mask == 0 {
		h.noverflow++
	}
}

/**
 * 创建新的溢出桶
 * @param
 * @return 新的溢出桶指针
 **/
func (h *hmap) newoverflow(t *maptype, b *bmap) *bmap {
	var ovf *bmap
	// 已经有额外数据，并且额外数据的nextOverflow不为空，
	if h.extra != nil && h.extra.nextOverflow != nil {
		// We have preallocated overflow buckets available.
		// See makeBucketArray for more details.
		// 我们有预分配的溢出桶可用。有关更多详细信息，请参见makeBucketArray。
		ovf = h.extra.nextOverflow
		if ovf.overflow(t) == nil {
			// We're not at the end of the preallocated overflow buckets. Bump the pointer.
			// 我们不在预分配的溢出桶的尽头。撞到指针。
			h.extra.nextOverflow = (*bmap)(add(unsafe.Pointer(ovf), uintptr(t.bucketsize)))
		} else {
			// This is the last preallocated overflow bucket.
			// Reset the overflow pointer on this bucket,
			// which was set to a non-nil sentinel value.
			// 这是最后一个预分配的溢出存储桶。重置此存储桶上的溢出指针，该指针已设置为非nil标记值，现在要设置成nil
			ovf.setoverflow(t, nil)
			h.extra.nextOverflow = nil
		}
	} else {
	    // 没有额外数据，创建新的溢出桶
		ovf = (*bmap)(newobject(t.bucket))
	}
	// 增加溢出桶计数
	h.incrnoverflow()
	if t.bucket.ptrdata == 0 { // 如果没有指针数据
		h.createOverflow() // 创建额外的溢出数据
		*h.extra.overflow = append(*h.extra.overflow, ovf) // 将溢出桶添加到溢出数组中
	}
	b.setoverflow(t, ovf)
	return ovf
}

/**
 * 创建h的溢出桶
 * @param
 * @return
 **/
func (h *hmap) createOverflow() {
	if h.extra == nil {
		h.extra = new(mapextra)
	}
	if h.extra.overflow == nil {
		h.extra.overflow = new([]*bmap)
	}
}

/**
 * 创建hmap，主要是对hint参数进行判定，不超出int可以表示的值
 * @param
 * @return
 **/
func makemap64(t *maptype, hint int64, h *hmap) *hmap {
	if int64(int(hint)) != hint {
		hint = 0
	}
	return makemap(t, int(hint), h)
}

// makemap_small implements Go map creation for make(map[k]v) and
// make(map[k]v, hint) when hint is known to be at most bucketCnt
// at compile time and the map needs to be allocated on the heap.
/**
 * 当在编译时已知hint最多为bucketCnt并且需要在堆上分配映射时。
 * makemap_small实现了make(map[k]v)和make(map[k]v, hint)的Go映射创建，
 * @param
 * @return
 **/
func makemap_small() *hmap {
	h := new(hmap)
	h.hash0 = fastrand()
	return h
}

// makemap implements Go map creation for make(map[k]v, hint).
// If the compiler has determined that the map or the first bucket
// can be created on the stack, h and/or bucket may be non-nil.
// If h != nil, the map can be created directly in h.
// If h.buckets != nil, bucket pointed to can be used as the first bucket.
/**
 * makemap为make(map[k]v, hint)实现Go map创建。
 * 如果编译器确定可以在堆栈上创建映射或第一个存储桶，则h和/或存储桶可能为非nil。
 * 如果h!= nil，则可以直接在h中创建地图。
 * 如果h.buckets != nil，则指向的存储桶可以用作第一个存储桶。
 * @param
 * @return
 **/
func makemap(t *maptype, hint int, h *hmap) *hmap {
    // 计算所需要的内存空间，并且判断是是否会有溢出
	mem, overflow := math.MulUintptr(uintptr(hint), t.bucket.size)
	if overflow || mem > maxAlloc { // 有溢出或者分配的内存大于最大分配内存
		hint = 0
	}

	// initialize Hmap
	// 初始化hmap
	if h == nil {
		h = new(hmap)
	}
	h.hash0 = fastrand() // 设置随机数

	// Find the size parameter B which will hold the requested # of elements.
	// For hint < 0 overLoadFactor returns false since hint < bucketCnt.
	// 找到用于保存请求的元素数的大小参数B。
    // 对于hint<0，由于hint < bucketCnt，overLoadFactor返回false。
	B := uint8(0)
	// 判断是否过载，过载B就增加
	for overLoadFactor(hint, B) {
		B++
	}
	h.B = B

	// allocate initial hash table
	// if B == 0, the buckets field is allocated lazily later (in mapassign)
	// If hint is large zeroing this memory could take a while.
	// 如果B == 0，则分配初始哈希表，则稍后（在mapassign中）延迟分配buckets字段。
	// 如果hint为零，则此内存可能需要一段时间。
	if h.B != 0 {
		var nextOverflow *bmap
		// 创建数据桶和溢出桶
		h.buckets, nextOverflow = makeBucketArray(t, h.B, nil)
		if nextOverflow != nil { // 溢出桶不为空就将溢出桶挂到附加数据上
			h.extra = new(mapextra)
			h.extra.nextOverflow = nextOverflow
		}
	}

	return h
}

// makeBucketArray initializes a backing array for map buckets.
// 1<<b is the minimum number of buckets to allocate.
// dirtyalloc should either be nil or a bucket array previously
// allocated by makeBucketArray with the same t and b parameters.
// If dirtyalloc is nil a new backing array will be alloced and
// otherwise dirtyalloc will be cleared and reused as backing array.
/**
 * makeBucketArray为map数据桶初始化底层数组。
 * 1<<b 是要分配的最小存储桶数。
 * dirtyalloc应该为nil或由makeBucketArray先前使用相同的t和b参数分配的bucket数组。
 * 如果dirtyalloc为nil，则将分配一个新的后备数组，否则，dirtyalloc将被清除并重新用作后备数组。
 * @param
 * @return
 **/
func makeBucketArray(t *maptype, b uint8, dirtyalloc unsafe.Pointer) (buckets unsafe.Pointer, nextOverflow *bmap) {
	base := bucketShift(b)
	nbuckets := base
	// For small b, overflow buckets are unlikely.
	// Avoid the overhead of the calculation.
	// 对于小b，溢出桶不太可能出现。避免计算的开销。
	if b >= 4 {
		// Add on the estimated number of overflow buckets
		// required to insert the median number of elements
		// used with this value of b.
		// 加上所需的溢流桶的估计数量，以插入使用此值b的元素的中位数。
		nbuckets += bucketShift(b - 4)
		sz := t.bucket.size * nbuckets
		up := roundupsize(sz) // 计算mallocgc分配的内存
		if up != sz {
			nbuckets = up / t.bucket.size // 计算每个桶的内存大小
		}
	}

	if dirtyalloc == nil {
	    // 直接创建数组
		buckets = newarray(t.bucket, int(nbuckets))
	} else {
		// dirtyalloc was previously generated by
		// the above newarray(t.bucket, int(nbuckets))
		// but may not be empty.
		// dirtyalloc先前是由上述newarray(t.bucket, int(nbuckets))生成的，但可能不为空。
		buckets = dirtyalloc
		size := t.bucket.size * nbuckets
		// 进行内存清零
		if t.bucket.ptrdata != 0 {
			memclrHasPointers(buckets, size)
		} else {
			memclrNoHeapPointers(buckets, size)
		}
	}

    // 实际计算的桶数据和最初的桶数不一样
	if base != nbuckets {
		// We preallocated some overflow buckets.
		// To keep the overhead of tracking these overflow buckets to a minimum,
		// we use the convention that if a preallocated overflow bucket's overflow
		// pointer is nil, then there are more available by bumping the pointer.
		// We need a safe non-nil pointer for the last overflow bucket; just use buckets.
		// 我们预先分配了一些溢出桶。
        // 为了使跟踪这些溢出桶的开销降到最低，我们使用以下约定：如果预分配的溢出桶的溢出指针为nil，则通过碰撞指针还有更多可用空间。
        // 对于最后一个溢出存储区，我们需要一个安全的非nil指针；使用buckets。
		nextOverflow = (*bmap)(add(buckets, base*uintptr(t.bucketsize))) // 计算下一个溢出桶
		last := (*bmap)(add(buckets, (nbuckets-1)*uintptr(t.bucketsize))) // 计算最后一个溢出桶
		last.setoverflow(t, (*bmap)(buckets))
	}
	return buckets, nextOverflow
}

// mapaccess1 returns a pointer to h[key].  Never returns nil, instead
// it will return a reference to the zero object for the elem type if
// the key is not in the map.
// NOTE: The returned pointer may keep the whole map live, so don't
// hold onto it for very long.
/**
 * mapaccess1返回指向h[key]的指针。从不返回nil，如果键不在映射中，它将返回对elem类型的零对象的引用。
 * 注意：返回的指针可能会使整个映射保持活动状态，因此不要在很长一段时间内都持有它。
 * @param
 * @return
 **/
func mapaccess1(t *maptype, h *hmap, key unsafe.Pointer) unsafe.Pointer {
	if raceenabled && h != nil {
		callerpc := getcallerpc()
		pc := funcPC(mapaccess1)
		racereadpc(unsafe.Pointer(h), callerpc, pc)
		raceReadObjectPC(t.key, key, callerpc, pc)
	}
	if msanenabled && h != nil {
		msanread(key, t.key.size)
	}
	if h == nil || h.count == 0 {
		if t.hashMightPanic() { // 如果h的hasher函数可能会painc，则调用hasher函数
			t.hasher(key, 0) // see issue 23734
		}
		return unsafe.Pointer(&zeroVal[0]) // 返回0值
	}
	if h.flags&hashWriting != 0 { // 写map的时候不能读
		throw("concurrent map read and map write")
	}
	hash := t.hasher(key, uintptr(h.hash0)) // 计算key的hash值
	m := bucketMask(h.B)
	b := (*bmap)(add(h.buckets, (hash&m)*uintptr(t.bucketsize))) // 计算桶的位置
	if c := h.oldbuckets; c != nil {
		if !h.sameSizeGrow() {
			// There used to be half as many buckets; mask down one more power of two.
			// 曾经有一半的桶。右移一位。
			m >>= 1
		}
		oldb := (*bmap)(add(c, (hash&m)*uintptr(t.bucketsize))) // 计算旧桶的位置
		if !evacuated(oldb) { // 旧桶的元素没有被迁移，取旧桶的值
			b = oldb
		}
	}
	top := tophash(hash)
bucketloop:
	for ; b != nil; b = b.overflow(t) { // 遍历每一个桶，包括溢出桶
		for i := uintptr(0); i < bucketCnt; i++ {
			if b.tophash[i] != top {
				if b.tophash[i] == emptyRest { // 元素为空，表示没有找到
					break bucketloop
				}
				continue
			}
			k := add(unsafe.Pointer(b), dataOffset+i*uintptr(t.keysize)) // 计算数据指针位置
			if t.indirectkey() { // 如果是非直接key，再取一次地址
				k = *((*unsafe.Pointer)(k))
			}
			if t.key.equal(key, k) { // key相等
				e := add(unsafe.Pointer(b), dataOffset+bucketCnt*uintptr(t.keysize)+i*uintptr(t.elemsize))
				if t.indirectelem() { // 如果是非直接elem，对e再取一次地址
					e = *((*unsafe.Pointer)(e))
				}
				return e // 返回找到的元素
			}
		}
	}
	return unsafe.Pointer(&zeroVal[0]) // 返回空元素
}

/**
 * 方法同mapaccess1，仅多返回一个值用于表示是否找到对应元素
 * @param
 * @return
 **/
func mapaccess2(t *maptype, h *hmap, key unsafe.Pointer) (unsafe.Pointer, bool) {
	if raceenabled && h != nil {
		callerpc := getcallerpc()
		pc := funcPC(mapaccess2)
		racereadpc(unsafe.Pointer(h), callerpc, pc)
		raceReadObjectPC(t.key, key, callerpc, pc)
	}
	if msanenabled && h != nil {
		msanread(key, t.key.size)
	}
	if h == nil || h.count == 0 {
		if t.hashMightPanic() {
			t.hasher(key, 0) // see issue 23734
		}
		return unsafe.Pointer(&zeroVal[0]), false
	}
	if h.flags&hashWriting != 0 {
		throw("concurrent map read and map write")
	}
	hash := t.hasher(key, uintptr(h.hash0))
	m := bucketMask(h.B)
	b := (*bmap)(unsafe.Pointer(uintptr(h.buckets) + (hash&m)*uintptr(t.bucketsize)))
	if c := h.oldbuckets; c != nil {
		if !h.sameSizeGrow() {
			// There used to be half as many buckets; mask down one more power of two.
			m >>= 1
		}
		oldb := (*bmap)(unsafe.Pointer(uintptr(c) + (hash&m)*uintptr(t.bucketsize)))
		if !evacuated(oldb) {
			b = oldb
		}
	}
	top := tophash(hash)
bucketloop:
	for ; b != nil; b = b.overflow(t) {
		for i := uintptr(0); i < bucketCnt; i++ {
			if b.tophash[i] != top {
				if b.tophash[i] == emptyRest {
					break bucketloop
				}
				continue
			}
			k := add(unsafe.Pointer(b), dataOffset+i*uintptr(t.keysize))
			if t.indirectkey() {
				k = *((*unsafe.Pointer)(k))
			}
			if t.key.equal(key, k) {
				e := add(unsafe.Pointer(b), dataOffset+bucketCnt*uintptr(t.keysize)+i*uintptr(t.elemsize))
				if t.indirectelem() {
					e = *((*unsafe.Pointer)(e))
				}
				return e, true
			}
		}
	}
	return unsafe.Pointer(&zeroVal[0]), false
}

// returns both key and elem. Used by map iterator
/**
 * 返回key和elem。由map迭代器使用，与mapaccess1相类似，只多返回了一个key
 * @param
 * @return
 **/
func mapaccessK(t *maptype, h *hmap, key unsafe.Pointer) (unsafe.Pointer, unsafe.Pointer) {
	if h == nil || h.count == 0 {
		return nil, nil
	}
	hash := t.hasher(key, uintptr(h.hash0))
	m := bucketMask(h.B)
	b := (*bmap)(unsafe.Pointer(uintptr(h.buckets) + (hash&m)*uintptr(t.bucketsize)))
	if c := h.oldbuckets; c != nil {
		if !h.sameSizeGrow() {
			// There used to be half as many buckets; mask down one more power of two.
			m >>= 1
		}
		oldb := (*bmap)(unsafe.Pointer(uintptr(c) + (hash&m)*uintptr(t.bucketsize)))
		if !evacuated(oldb) {
			b = oldb
		}
	}
	top := tophash(hash)
bucketloop:
	for ; b != nil; b = b.overflow(t) {
		for i := uintptr(0); i < bucketCnt; i++ {
			if b.tophash[i] != top {
				if b.tophash[i] == emptyRest {
					break bucketloop
				}
				continue
			}
			k := add(unsafe.Pointer(b), dataOffset+i*uintptr(t.keysize))
			if t.indirectkey() {
				k = *((*unsafe.Pointer)(k))
			}
			if t.key.equal(key, k) {
				e := add(unsafe.Pointer(b), dataOffset+bucketCnt*uintptr(t.keysize)+i*uintptr(t.elemsize))
				if t.indirectelem() {
					e = *((*unsafe.Pointer)(e))
				}
				return k, e
			}
		}
	}
	return nil, nil
}

/**
 * 获取map中key对应的值，如果没有找到就返回zero
 * @param
 * @return
 **/
func mapaccess1_fat(t *maptype, h *hmap, key, zero unsafe.Pointer) unsafe.Pointer {
	e := mapaccess1(t, h, key)
	if e == unsafe.Pointer(&zeroVal[0]) {
		return zero
	}
	return e
}

/**
 * 获取map中key对应的值，如果没有找到就返回zero，并返回是否找到标记
 * @param
 * @return
 **/
func mapaccess2_fat(t *maptype, h *hmap, key, zero unsafe.Pointer) (unsafe.Pointer, bool) {
	e := mapaccess1(t, h, key)
	if e == unsafe.Pointer(&zeroVal[0]) {
		return zero, false
	}
	return e, true
}

// Like mapaccess, but allocates a slot for the key if it is not present in the map.
/**
 * 与mapaccess类似，但是如果map中不存在key，则为该key分配一个位置。
 * @param
 * @return key对应elem的插入位置指针
 **/
func mapassign(t *maptype, h *hmap, key unsafe.Pointer) unsafe.Pointer {
	if h == nil {
		panic(plainError("assignment to entry in nil map"))
	}
	if raceenabled {
		callerpc := getcallerpc()
		pc := funcPC(mapassign)
		racewritepc(unsafe.Pointer(h), callerpc, pc)
		raceReadObjectPC(t.key, key, callerpc, pc)
	}
	if msanenabled {
		msanread(key, t.key.size)
	}
	if h.flags&hashWriting != 0 { // 不能并发读写
		throw("concurrent map writes")
	}
	hash := t.hasher(key, uintptr(h.hash0)) // 计算hash值

	// Set hashWriting after calling t.hasher, since t.hasher may panic,
	// in which case we have not actually done a write.
	// 在调用t.hasher之后设置hashWriting，因为t.hasher可能会出现panic情况，在这种情况下，我们实际上并未执行写入操作。
	h.flags ^= hashWriting

	if h.buckets == nil { // 如果桶为空，就创建大小为1的桶
		h.buckets = newobject(t.bucket) // newarray(t.bucket, 1)
	}

again:
	bucket := hash & bucketMask(h.B)
	if h.growing() { // 是否在扩容
		growWork(t, h, bucket) // 进行扩容处理
	}
	b := (*bmap)(unsafe.Pointer(uintptr(h.buckets) + bucket*uintptr(t.bucketsize))) // 计算桶的位置
	top := tophash(hash)

	var inserti *uint8
	var insertk unsafe.Pointer
	var elem unsafe.Pointer
bucketloop:
	for {
		for i := uintptr(0); i < bucketCnt; i++ { // 遍历对应桶中的元素
			if b.tophash[i] != top {
				if isEmpty(b.tophash[i]) && inserti == nil {
					inserti = &b.tophash[i]
					insertk = add(unsafe.Pointer(b), dataOffset+i*uintptr(t.keysize))
					elem = add(unsafe.Pointer(b), dataOffset+bucketCnt*uintptr(t.keysize)+i*uintptr(t.elemsize))
				}
				if b.tophash[i] == emptyRest {
					break bucketloop
				}
				continue
			}
			k := add(unsafe.Pointer(b), dataOffset+i*uintptr(t.keysize))
			if t.indirectkey() {
				k = *((*unsafe.Pointer)(k))
			}
			if !t.key.equal(key, k) {
				continue
			}
			// already have a mapping for key. Update it.
			// 已经有一个key映射。更新它。
			if t.needkeyupdate() {
				typedmemmove(t.key, k, key)
			}
			elem = add(unsafe.Pointer(b), dataOffset+bucketCnt*uintptr(t.keysize)+i*uintptr(t.elemsize))
			goto done
		}
		ovf := b.overflow(t) // 找下一个溢出桶进行处理
		if ovf == nil {
			break
		}
		b = ovf
	}

	// Did not find mapping for key. Allocate new cell & add entry.
	// 找不到键的映射。分配新单元格并添加条目。

	// If we hit the max load factor or we have too many overflow buckets,
	// and we're not already in the middle of growing, start growing.
	// 如果我们达到最大负载因子，或者我们有太多的溢出桶，而我们还没有处于增长过程，那就开始增长。
	if !h.growing() && (overLoadFactor(h.count+1, h.B) || tooManyOverflowBuckets(h.noverflow, h.B)) {
		hashGrow(t, h)
		goto again // Growing the table invalidates everything, so try again // 扩容表格会使所有内容失效，因此请重试
	}

	if inserti == nil {
		// all current buckets are full, allocate a new one.
		// 当前所有存储桶已满，请分配一个新的存储桶。
		newb := h.newoverflow(t, b)
		inserti = &newb.tophash[0]
		insertk = add(unsafe.Pointer(newb), dataOffset)
		elem = add(insertk, bucketCnt*uintptr(t.keysize))
	}

	// store new key/elem at insert position
	// 在插入位置存储新的key/elem
	if t.indirectkey() { // 插入key
		kmem := newobject(t.key)
		*(*unsafe.Pointer)(insertk) = kmem
		insertk = kmem
	}
	if t.indirectelem() { // 插入elem
		vmem := newobject(t.elem)
		*(*unsafe.Pointer)(elem) = vmem
	}
	typedmemmove(t.key, insertk, key)
	*inserti = top
	h.count++

done:
	if h.flags&hashWriting == 0 {
		throw("concurrent map writes")
	}
	h.flags &^= hashWriting
	if t.indirectelem() {
		elem = *((*unsafe.Pointer)(elem))
	}
	return elem
}

/**
 * 删除key
 * @param
 * @return
 **/
func mapdelete(t *maptype, h *hmap, key unsafe.Pointer) {
	if raceenabled && h != nil {
		callerpc := getcallerpc()
		pc := funcPC(mapdelete)
		racewritepc(unsafe.Pointer(h), callerpc, pc)
		raceReadObjectPC(t.key, key, callerpc, pc)
	}
	if msanenabled && h != nil {
		msanread(key, t.key.size)
	}
	if h == nil || h.count == 0 {
		if t.hashMightPanic() {
			t.hasher(key, 0) // see issue 23734
		}
		return
	}
	if h.flags&hashWriting != 0 { // 当前map正在被写，不能再写
		throw("concurrent map writes")
	}

	hash := t.hasher(key, uintptr(h.hash0))

	// Set hashWriting after calling t.hasher, since t.hasher may panic,
	// in which case we have not actually done a write (delete).
	// 在调用t.hasher之后设置hashWriting，因为t.hasher可能会出现panic情况，
	// 在这种情况下，我们实际上并未执行写入（删除）操作。
	h.flags ^= hashWriting

	bucket := hash & bucketMask(h.B)
	if h.growing() {
		growWork(t, h, bucket)
	}
	b := (*bmap)(add(h.buckets, bucket*uintptr(t.bucketsize)))
	bOrig := b
	top := tophash(hash)
search:
	for ; b != nil; b = b.overflow(t) {
		for i := uintptr(0); i < bucketCnt; i++ {
			if b.tophash[i] != top {
				if b.tophash[i] == emptyRest {
					break search
				}
				continue
			}
			k := add(unsafe.Pointer(b), dataOffset+i*uintptr(t.keysize))
			k2 := k
			if t.indirectkey() {
				k2 = *((*unsafe.Pointer)(k2))
			}
			if !t.key.equal(key, k2) { // 两个key不相等
				continue
			}
			// Only clear key if there are pointers in it.
			// 如果其中有指针，则仅清除键。
			if t.indirectkey() {
				*(*unsafe.Pointer)(k) = nil
			} else if t.key.ptrdata != 0 {
				memclrHasPointers(k, t.key.size)
			}
			e := add(unsafe.Pointer(b), dataOffset+bucketCnt*uintptr(t.keysize)+i*uintptr(t.elemsize))
			if t.indirectelem() { // elem是间接指针，将指针赋空
				*(*unsafe.Pointer)(e) = nil
			} else if t.elem.ptrdata != 0 { // 元素有指针数据，将清除指针数据
				memclrHasPointers(e, t.elem.size)
			} else { // 清除e的数据
				memclrNoHeapPointers(e, t.elem.size)
			}
			b.tophash[i] = emptyOne // 标记桶为空
			// If the bucket now ends in a bunch of emptyOne states,
			// change those to emptyRest states.
			// It would be nice to make this a separate function, but
			// for loops are not currently inlineable.
			// 如果存储桶现在以一堆emptyOne状态结束，则将其更改为emptyRest状态。
			// 将此功能设为单独的函数会很好，但是for循环当前不可内联。
			if i == bucketCnt-1 {
			    // 有下一个溢出桶，并且溢出桶有值
				if b.overflow(t) != nil && b.overflow(t).tophash[0] != emptyRest {
					goto notLast
				}
			} else { // 没有溢出桶了，但是当前位置之后的位置还有值
				if b.tophash[i+1] != emptyRest {
					goto notLast
				}
			}
			for { // 当前桶的当前位置之后都没有值了
				b.tophash[i] = emptyRest // 标记当前位置已经空
				if i == 0 {
					if b == bOrig {
					    // 从初始存储桶开始，我们已经处理完了。
						break // beginning of initial bucket, we're done.
					}
					// Find previous bucket, continue at its last entry.
					// 查找上一个存储桶，直到最后一个。
					c := b
					for b = bOrig; b.overflow(t) != c; b = b.overflow(t) {
					}
					i = bucketCnt - 1
				} else {
					i--
				}
				if b.tophash[i] != emptyOne {
					break
				}
			}
		notLast:
			h.count--
			break search
		}
	}

	if h.flags&hashWriting == 0 {
		throw("concurrent map writes")
	}
	h.flags &^= hashWriting
}

// mapiterinit initializes the hiter struct used for ranging over maps.
// The hiter struct pointed to by 'it' is allocated on the stack
// by the compilers order pass or on the heap by reflect_mapiterinit.
// Both need to have zeroed hiter since the struct contains pointers.
/**
 * mapiterinit初始化用于在map上进行遍历的hiter结构。
 * “it”所指向的hiter结构是由编译器顺序传递在堆栈上分配的，或者由reflect_mapiterinit在堆上分配的。
 * 由于结构包含指针，因此两者都需要将hiter归零。
 * @param t 映射类型
 * @param h 要遍历的map
 * @param 迭代器对象
 **/
func mapiterinit(t *maptype, h *hmap, it *hiter) {
	if raceenabled && h != nil {
		callerpc := getcallerpc()
		racereadpc(unsafe.Pointer(h), callerpc, funcPC(mapiterinit))
	}

	if h == nil || h.count == 0 {
		return
	}

	if unsafe.Sizeof(hiter{})/sys.PtrSize != 12 { // hiter{}的大小必须是指针大小的12倍
		throw("hash_iter size incorrect") // see cmd/compile/internal/gc/reflect.go
	}
	it.t = t
	it.h = h

	// grab snapshot of bucket state
	// 抓取桶状态快照
	it.B = h.B
	it.buckets = h.buckets
	if t.bucket.ptrdata == 0 {
		// Allocate the current slice and remember pointers to both current and old.
		// This preserves all relevant overflow buckets alive even if
		// the table grows and/or overflow buckets are added to the table
		// while we are iterating.
		// 分配当前切片，并记住指向当前和旧指针。
        // 这样即使表不断增长和/或在我们进行迭代时将溢出桶添加到表中，
        // 也可以使所有相关的溢出桶保持活动状态。
		h.createOverflow()
		it.overflow = h.extra.overflow
		it.oldoverflow = h.extra.oldoverflow
	}

	// decide where to start
	// 决定从哪里开始
	r := uintptr(fastrand())
	// fastrand()只产生32位hash，当hash表过大时需要生产新的高位hash值
	if h.B > 31-bucketCntBits {
		r += uintptr(fastrand()) << 31 // 对于超级大的桶，产生高位hash值
	}
	it.startBucket = r & bucketMask(h.B) // 记录桶起始位置
	it.offset = uint8(r >> h.B & (bucketCnt - 1)) // 记录桶内偏移量

	// iterator state
	// 迭代器状态，即记录当前迭代器所在的桶
	it.bucket = it.startBucket

	// Remember we have an iterator.
	// Can run concurrently with another mapiterinit().
	// 记住我们有一个迭代器。可以与另一个mapiterinit（）同时运行。
	if old := h.flags; old&(iterator|oldIterator) != iterator|oldIterator {
		atomic.Or8(&h.flags, iterator|oldIterator)
	}

    // 开始迭代
	mapiternext(it)
}

/**
 * 进行map迭代
 * @param
 * @return
 **/
func mapiternext(it *hiter) {
	h := it.h
	if raceenabled {
		callerpc := getcallerpc()
		racereadpc(unsafe.Pointer(h), callerpc, funcPC(mapiternext))
	}
	if h.flags&hashWriting != 0 {
		throw("concurrent map iteration and map write")
	}
	t := it.t
	bucket := it.bucket
	b := it.bptr // 当前桶
	i := it.i
	checkBucket := it.checkBucket

next:
	if b == nil { // 当前桶为空
		if bucket == it.startBucket && it.wrapped { // 再次到达起始位置，说明迭代已经结束
			// end of iteration
			it.key = nil
			it.elem = nil
			return
		}
		if h.growing() && it.B == h.B {
			// Iterator was started in the middle of a grow, and the grow isn't done yet.
			// If the bucket we're looking at hasn't been filled in yet (i.e. the old
			// bucket hasn't been evacuated) then we need to iterate through the old
			// bucket and only return the ones that will be migrated to this bucket.
			// 迭代器是在增长过程中启动的，尚未完成增长。如果我们要查看的存储桶尚未填充（即尚未迁移旧存储桶），
			// 则我们需要遍历旧存储桶，只返回将要迁移到该存储桶的存储桶。
			oldbucket := bucket & it.h.oldbucketmask()
			b = (*bmap)(add(h.oldbuckets, oldbucket*uintptr(t.bucketsize)))
			if !evacuated(b) { // 桶中的数据还未迁移
				checkBucket = bucket
			} else {
				b = (*bmap)(add(it.buckets, bucket*uintptr(t.bucketsize)))
				checkBucket = noCheck
			}
		} else {
			b = (*bmap)(add(it.buckets, bucket*uintptr(t.bucketsize)))
			checkBucket = noCheck
		}
		// 移动到下一个桶
		bucket++
		if bucket == bucketShift(it.B) { // 已经到达最后一个桶了
			bucket = 0
			it.wrapped = true // 标记，用于指标下一次迭代就是重新开始了
		}
		i = 0
	}
	for ; i < bucketCnt; i++ { // 迭代桶中的元素
		offi := (i + it.offset) & (bucketCnt - 1)
		if isEmpty(b.tophash[offi]) || b.tophash[offi] == evacuatedEmpty { // 如果是空桶，或者桶中的无素已经迁移，跳过
			// TODO: emptyRest is hard to use here, as we start iterating
			// in the middle of a bucket. It's feasible, just tricky.
		    // TODO: 当我们开始在存储桶中间进行迭代时，emptyRest很难用。这是可行的，只是棘手。
			continue
		}
		// 找key
		k := add(unsafe.Pointer(b), dataOffset+uintptr(offi)*uintptr(t.keysize))
		if t.indirectkey() {
			k = *((*unsafe.Pointer)(k))
		}
		// 找elem
		e := add(unsafe.Pointer(b), dataOffset+bucketCnt*uintptr(t.keysize)+uintptr(offi)*uintptr(t.elemsize))
		if checkBucket != noCheck && !h.sameSizeGrow() {
			// Special case: iterator was started during a grow to a larger size
			// and the grow is not done yet. We're working on a bucket whose
			// oldbucket has not been evacuated yet. Or at least, it wasn't
			// evacuated when we started the bucket. So we're iterating
			// through the oldbucket, skipping any keys that will go
			// to the other new bucket (each oldbucket expands to two
			// buckets during a grow).
			// 特殊情况：迭代器是在增长到更大的大小期间启动的，尚未完成增长。
			// 我们正在处理尚未迁移oldbucket的存储桶。至少，当我们启动存储桶时，它并没有被迁移。
			// 因此，我们正在遍历oldbucket，跳过将要转到另一个新bucket的所有key
			// （在增长过程中，每个oldbucket会扩展为两个bucket）。
			if t.reflexivekey() || t.key.equal(k, k) {
				// If the item in the oldbucket is not destined for
				// the current new bucket in the iteration, skip it.
				// 如果oldbucket中的条目不是发往迭代中的当前新bucket，请跳过它。
				hash := t.hasher(k, uintptr(h.hash0))
				if hash&bucketMask(it.B) != checkBucket {
					continue
				}
			} else {
				// Hash isn't repeatable if k != k (NaNs).  We need a
				// repeatable and randomish choice of which direction
				// to send NaNs during evacuation. We'll use the low
				// bit of tophash to decide which way NaNs go.
				// NOTE: this case is why we need two evacuate tophash
				// values, evacuatedX and evacuatedY, that differ in
				// their low bit.
				// 如果k！= k（NaNs），则哈希不可重复。我们需要对迁移期间发送NaN的方向进行可重复且随机的选择。
				// 我们将使用低位的Tophash来决定NaN的走法。
                // 注意：这种情况就是为什么我们需要两个迁移值，即evacuatedX和evacuatedY，它们的低位不同。
				if checkBucket>>(it.B-1) != uintptr(b.tophash[offi]&1) {
					continue
				}
			}
		}
		// 无素没有被迁移
		if (b.tophash[offi] != evacuatedX && b.tophash[offi] != evacuatedY) ||
			!(t.reflexivekey() || t.key.equal(k, k)) {
			// This is the golden data, we can return it.
			// OR
			// key!=key, so the entry can't be deleted or updated, so we can just return it.
			// That's lucky for us because when key!=key we can't look it up successfully.
			// 这是黄金数据（golden data），我们可以将其返回。
            // 要么
            // key!= key，因此无法删除或更新该条目，因此我们可以将其返回。
            // 这对我们来说是幸运的，因为当key!= key时，我们无法成功查找它。
			it.key = k
			if t.indirectelem() {
				e = *((*unsafe.Pointer)(e))
			}
			it.elem = e
		} else {
			// The hash table has grown since the iterator was started.
			// The golden data for this key is now somewhere else.
			// Check the current hash table for the data.
			// This code handles the case where the key
			// has been deleted, updated, or deleted and reinserted.
			// NOTE: we need to regrab the key as it has potentially been
			// updated to an equal() but not identical key (e.g. +0.0 vs -0.0).
			// 自从启动迭代器以来，哈希表已经增长。该key的黄金数据现在位于其他位置。
			// 检查当前哈希表中的数据。此代码处理key已被删除，更新，或删除并重新插入的情况。
            // 注意：我们需要重新注册key，因为它可能已更新为equal()，但key不相同（例如+0.0 vs -0.0）。
			rk, re := mapaccessK(t, h, k)
			if rk == nil {
				continue // key has been deleted // key已被删除
			}
			// 记录key、elem
			it.key = rk
			it.elem = re
		}
		it.bucket = bucket
		// 避免不必要的写屏障
		if it.bptr != b { // avoid unnecessary write barrier; see issue 14921
			it.bptr = b
		}
		it.i = i + 1 // 移动到桶的下一个位置
		it.checkBucket = checkBucket
		return
	}
	b = b.overflow(t) // 找溢出桶，重新开始处理
	i = 0
	goto next
}

// mapclear deletes all keys from a map.
/**
 * 清除map中的所有数据
 * @param
 * @return
 **/
func mapclear(t *maptype, h *hmap) {
	if raceenabled && h != nil {
		callerpc := getcallerpc()
		pc := funcPC(mapclear)
		racewritepc(unsafe.Pointer(h), callerpc, pc)
	}

	if h == nil || h.count == 0 {
		return
	}

	if h.flags&hashWriting != 0 {
		throw("concurrent map writes")
	}

	h.flags ^= hashWriting // 标记当前协程正在写map

	h.flags &^= sameSizeGrow // 标记当前协程正在创建一个相当大小的map
	h.oldbuckets = nil
	h.nevacuate = 0
	h.noverflow = 0
	h.count = 0

	// Keep the mapextra allocation but clear any extra information.
	// 保留mapextra分配，但清除所有其他信息。
	if h.extra != nil {
		*h.extra = mapextra{}
	}

	// makeBucketArray clears the memory pointed to by h.buckets
	// and recovers any overflow buckets by generating them
	// as if h.buckets was newly alloced.
	// makeBucketArray清除h.buckets指向的内存，并通过生成溢出桶来恢复所有溢出桶，
	// 就好像重新分配了h.buckets一样。
	_, nextOverflow := makeBucketArray(t, h.B, h.buckets)
	if nextOverflow != nil {
		// If overflow buckets are created then h.extra
		// will have been allocated during initial bucket creation.
		// 如果创建了溢出存储桶，则将在初始存储桶创建过程中分配h.extra。
		h.extra.nextOverflow = nextOverflow
	}

	if h.flags&hashWriting == 0 {
		throw("concurrent map writes")
	}
	h.flags &^= hashWriting // 位清空
}

/**
 * map扩容
 * @param
 * @return
 **/
func hashGrow(t *maptype, h *hmap) {
	// If we've hit the load factor, get bigger.
	// Otherwise, there are too many overflow buckets,
	// so keep the same number of buckets and "grow" laterally.
	// 如果我们达到了负载因子，请扩容。
	// 否则，溢出桶过多，因此保持相同数量的桶并横向“增长”。
	bigger := uint8(1)
	if !overLoadFactor(h.count+1, h.B) { // 增加一个元素，没有超过负载因子
		bigger = 0
		h.flags |= sameSizeGrow
	}
	oldbuckets := h.buckets
	// 创建新桶
	newbuckets, nextOverflow := makeBucketArray(t, h.B+bigger, nil)

	flags := h.flags &^ (iterator | oldIterator)
	if h.flags&iterator != 0 {
		flags |= oldIterator
	}
	// commit the grow (atomic wrt gc)
	// 提交增长（atomic wrt gc）
	h.B += bigger
	h.flags = flags
	h.oldbuckets = oldbuckets
	h.buckets = newbuckets
	h.nevacuate = 0
	h.noverflow = 0

	if h.extra != nil && h.extra.overflow != nil {
		// Promote current overflow buckets to the old generation.
		// 将当前的溢出桶提升到老一代。
		if h.extra.oldoverflow != nil {
			throw("oldoverflow is not nil")
		}
		h.extra.oldoverflow = h.extra.overflow
		h.extra.overflow = nil
	}
	if nextOverflow != nil {
		if h.extra == nil {
			h.extra = new(mapextra)
		}
		h.extra.nextOverflow = nextOverflow
	}

	// the actual copying of the hash table data is done incrementally
	// by growWork() and evacuate().
	// 哈希表数据的实际复制是通过growWork()和evacuate()增量完成的。
}

// overLoadFactor reports whether count items placed in 1<<B buckets is over loadFactor.
/**
 * overLoadFactor报告放置在1<<B个存储桶中的计数项是否超过loadFactor。
 * @param
 * @return
 **/
func overLoadFactor(count int, B uint8) bool {
	return count > bucketCnt && uintptr(count) > loadFactorNum*(bucketShift(B)/loadFactorDen)
}

// tooManyOverflowBuckets reports whether noverflow buckets is too many for a map with 1<<B buckets.
// Note that most of these overflow buckets must be in sparse use;
// if use was dense, then we'd have already triggered regular map growth.
/**
 * tooManyOverflowBuckets报告noverflow存储桶对于具有1<<B存储桶的map是否过多。
 * 注意，这些溢出桶中的大多数必须处于稀疏状态；如果使用密集，那么我们已经触发了常规地图增长。
 * @param
 * @return
 **/
func tooManyOverflowBuckets(noverflow uint16, B uint8) bool {
	// If the threshold is too low, we do extraneous work.
	// If the threshold is too high, maps that grow and shrink can hold on to lots of unused memory.
	// "too many" means (approximately) as many overflow buckets as regular buckets.
	// See incrnoverflow for more details.
	// 如果阈值太低，我们会进行多余的工作。如果阈值太高，则增大和缩小的映射可能会保留大量未使用的内存。
    // “太多”意味着（大约）溢出桶与常规桶一样多。有关更多详细信息，请参见incrnoverflow。
	if B > 15 {
		B = 15
	}
	// The compiler doesn't see here that B < 16; mask B to generate shorter shift code.
	// 编译器在这里看不到B <16;掩码B生成较短的移位码。
	return noverflow >= uint16(1)<<(B&15)
}

// growing reports whether h is growing. The growth may be to the same size or bigger.
/**
 * growing报告h是否在增长。增长可能达到相同大小或更大。
 * @param
 * @return
 **/
func (h *hmap) growing() bool {
	return h.oldbuckets != nil
}

// sameSizeGrow reports whether the current growth is to a map of the same size.
/**
 * sameSizeGrow报告当前的增长是否针对当前map相同大小增长。
 * @param
 * @return
 **/
func (h *hmap) sameSizeGrow() bool {
	return h.flags&sameSizeGrow != 0
}

// noldbuckets calculates the number of buckets prior to the current map growth.
/**
 * noldbuckets计算当前map增长之前的存储桶数。
 * @param
 * @return
 **/
func (h *hmap) noldbuckets() uintptr {
	oldB := h.B
	if !h.sameSizeGrow() {
		oldB--
	}
	return bucketShift(oldB)
}

// oldbucketmask provides a mask that can be applied to calculate n % noldbuckets().
/**
 * oldbucketmask提供了可用于计算n%noldbuckets()的掩码。
 * @param
 * @return
 **/
func (h *hmap) oldbucketmask() uintptr {
	return h.noldbuckets() - 1
}

/**
 * 桶增长
 * @param
 * @return
 **/
func growWork(t *maptype, h *hmap, bucket uintptr) {
	// make sure we evacuate the oldbucket corresponding
	// to the bucket we're about to use
	// 确保我们迁移将要使用的存储桶对应的旧存储桶
	evacuate(t, h, bucket&h.oldbucketmask())

	// evacuate one more oldbucket to make progress on growing
    // 迁移一个旧桶，会使过程标记在growing
	if h.growing() {
		evacuate(t, h, h.nevacuate)
	}
}

/**
 * 桶是否已经迁移过
 * @param
 * @return
 **/
func bucketEvacuated(t *maptype, h *hmap, bucket uintptr) bool {
	b := (*bmap)(add(h.oldbuckets, bucket*uintptr(t.bucketsize)))
	return evacuated(b)
}

// evacDst is an evacuation destination.
/**
 * evacDst是迁移目的地。
 **/
type evacDst struct {
	b *bmap          // current destination bucket // 当前目标桶
	i int            // key/elem index into b // b的key/elem索引
	k unsafe.Pointer // pointer to current key storage // 指向当前k的存储位置
	e unsafe.Pointer // pointer to current elem storage // 指向当前elem的存储位置
}

/**
 * 进行迁移
 * @param
 * @param
 * @param oldbucket 需要迁移的桶
 * @return
 **/
func evacuate(t *maptype, h *hmap, oldbucket uintptr) {
	b := (*bmap)(add(h.oldbuckets, oldbucket*uintptr(t.bucketsize)))
	newbit := h.noldbuckets()
	if !evacuated(b) {
		// TODO: reuse overflow buckets instead of using new ones, if there
		// is no iterator using the old buckets.  (If !oldIterator.)
		// TODO：如果没有迭代器使用旧的存储桶，则重用溢出存储桶而不是使用新的存储桶。（如果为!oldIterator。）

		// xy contains the x and y (low and high) evacuation destinations.
		// xy包含x和y（低和高）迁移目的地。
		var xy [2]evacDst
		x := &xy[0]
		x.b = (*bmap)(add(h.buckets, oldbucket*uintptr(t.bucketsize)))
		x.k = add(unsafe.Pointer(x.b), dataOffset)
		x.e = add(x.k, bucketCnt*uintptr(t.keysize))

		if !h.sameSizeGrow() { // 非同大小增长
			// Only calculate y pointers if we're growing bigger.
			// Otherwise GC can see bad pointers.
			// 仅当我们变得更大时才计算y指针。否则GC可能会看到错误的指针。
			y := &xy[1]
			y.b = (*bmap)(add(h.buckets, (oldbucket+newbit)*uintptr(t.bucketsize)))
			y.k = add(unsafe.Pointer(y.b), dataOffset)
			y.e = add(y.k, bucketCnt*uintptr(t.keysize))
		}

		for ; b != nil; b = b.overflow(t) {
			k := add(unsafe.Pointer(b), dataOffset)
			e := add(k, bucketCnt*uintptr(t.keysize))
			for i := 0; i < bucketCnt; i, k, e = i+1, add(k, uintptr(t.keysize)), add(e, uintptr(t.elemsize)) {
				top := b.tophash[i]
				if isEmpty(top) {
					b.tophash[i] = evacuatedEmpty
					continue
				}
				if top < minTopHash {
					throw("bad map state")
				}
				k2 := k
				if t.indirectkey() {
					k2 = *((*unsafe.Pointer)(k2))
				}
				var useY uint8
				if !h.sameSizeGrow() {
					// Compute hash to make our evacuation decision (whether we need
					// to send this key/elem to bucket x or bucket y).
					// 计算散列值以做出迁移决定（是否需要将此key/elem发送到存储桶x或存储桶y）。
					hash := t.hasher(k2, uintptr(h.hash0))
					if h.flags&iterator != 0 && !t.reflexivekey() && !t.key.equal(k2, k2) {
						// If key != key (NaNs), then the hash could be (and probably
						// will be) entirely different from the old hash. Moreover,
						// it isn't reproducible. Reproducibility is required in the
						// presence of iterators, as our evacuation decision must
						// match whatever decision the iterator made.
						// Fortunately, we have the freedom to send these keys either
						// way. Also, tophash is meaningless for these kinds of keys.
						// We let the low bit of tophash drive the evacuation decision.
						// We recompute a new random tophash for the next level so
						// these keys will get evenly distributed across all buckets
						// after multiple grows.
						// 如果key != key(NaNs)，则哈希可能（可能会）与旧哈希完全不同。而且，它是不可复制的。
						// 在存在迭代器的情况下，要求具有可重复性，因为我们的迁移决策必须与迭代器所做的任何决策相匹配。
						// 幸运的是，我们可以自由发送这些key。同样，tophash对于这些key也没有意义。
						// 我们让低位的hophash决定迁移。我们为下一个级别重新计算了一个新的随机tophash，以便在多次增长之后，
						// 这些key将在所有存储桶中平均分配
						useY = top & 1
						top = tophash(hash)
					} else {
						if hash&newbit != 0 {
							useY = 1
						}
					}
				}

				if evacuatedX+1 != evacuatedY || evacuatedX^1 != evacuatedY {
					throw("bad evacuatedN")
				}

				b.tophash[i] = evacuatedX + useY // evacuatedX + 1 == evacuatedY
				dst := &xy[useY]                 // evacuation destination // 迁移目的地

				if dst.i == bucketCnt {
					dst.b = h.newoverflow(t, dst.b)
					dst.i = 0
					dst.k = add(unsafe.Pointer(dst.b), dataOffset)
					dst.e = add(dst.k, bucketCnt*uintptr(t.keysize))
				}
				// 掩码dst.i作为优化，以避免边界检查
				dst.b.tophash[dst.i&(bucketCnt-1)] = top // mask dst.i as an optimization, to avoid a bounds check
				if t.indirectkey() {
					*(*unsafe.Pointer)(dst.k) = k2 // copy pointer // 拷贝指针
				} else {
					typedmemmove(t.key, dst.k, k) // copy elem // 拷贝元素
				}
				if t.indirectelem() {
					*(*unsafe.Pointer)(dst.e) = *(*unsafe.Pointer)(e)
				} else {
					typedmemmove(t.elem, dst.e, e)
				}
				dst.i++
				// These updates might push these pointers past the end of the
				// key or elem arrays.  That's ok, as we have the overflow pointer
				// at the end of the bucket to protect against pointing past the
				// end of the bucket.
				// 这些更新可能会将这些指针推到key或elem数组的末尾。
				// 没关系，因为我们在存储桶的末尾有溢出指针，以防止指向存储桶的末尾。
				dst.k = add(dst.k, uintptr(t.keysize))
				dst.e = add(dst.e, uintptr(t.elemsize))
			}
		}
		// Unlink the overflow buckets & clear key/elem to help GC.
		// 取消链接溢出桶并清除key/elem，以帮助GC。
		if h.flags&oldIterator == 0 && t.bucket.ptrdata != 0 {
			b := add(h.oldbuckets, oldbucket*uintptr(t.bucketsize))
			// Preserve b.tophash because the evacuation
			// state is maintained there.
			// 因为疏散状态一直保持在那里，所以要保留b.tophash。
			ptr := add(b, dataOffset)
			n := uintptr(t.bucketsize) - dataOffset
			memclrHasPointers(ptr, n)
		}
	}

	if oldbucket == h.nevacuate {
		advanceEvacuationMark(h, t, newbit)
	}
}

/**
 *
 * @param
 * @return
 **/
func advanceEvacuationMark(h *hmap, t *maptype, newbit uintptr) {
	h.nevacuate++
	// Experiments suggest that 1024 is overkill by at least an order of magnitude.
	// Put it in there as a safeguard anyway, to ensure O(1) behavior.
	// 实验表明，1024的杀伤力至少高出一个数量级。无论如何都要将其放在其中以确保O（1）行为。
	stop := h.nevacuate + 1024
	if stop > newbit {
		stop = newbit
	}
	// 迁移直到不成功或者等于stop
	for h.nevacuate != stop && bucketEvacuated(t, h, h.nevacuate) {
		h.nevacuate++
	}
	if h.nevacuate == newbit { // newbit == # of oldbuckets
		// Growing is all done. Free old main bucket array.
		// 增长长已经完成。自由使用旧的主存储桶数组。
		h.oldbuckets = nil
		// Can discard old overflow buckets as well.
		// If they are still referenced by an iterator,
		// then the iterator holds a pointers to the slice.
		// 也可以丢弃旧的溢出桶。
        // 如果迭代器仍在引用它们，则迭代器将保留指向切片的指针。
		if h.extra != nil {
			h.extra.oldoverflow = nil
		}
		h.flags &^= sameSizeGrow
	}
}

// Reflect stubs. Called from ../reflect/asm_*.s
// 以下方法是反射调用，对应../reflect/asm_*.s中的方法

//go:linkname reflect_makemap reflect.makemap
func reflect_makemap(t *maptype, cap int) *hmap {
	// Check invariants and reflects math.
	if t.key.equal == nil {
		throw("runtime.reflect_makemap: unsupported map key type")
	}
	if t.key.size > maxKeySize && (!t.indirectkey() || t.keysize != uint8(sys.PtrSize)) ||
		t.key.size <= maxKeySize && (t.indirectkey() || t.keysize != uint8(t.key.size)) {
		throw("key size wrong")
	}
	if t.elem.size > maxElemSize && (!t.indirectelem() || t.elemsize != uint8(sys.PtrSize)) ||
		t.elem.size <= maxElemSize && (t.indirectelem() || t.elemsize != uint8(t.elem.size)) {
		throw("elem size wrong")
	}
	if t.key.align > bucketCnt {
		throw("key align too big")
	}
	if t.elem.align > bucketCnt {
		throw("elem align too big")
	}
	if t.key.size%uintptr(t.key.align) != 0 {
		throw("key size not a multiple of key align")
	}
	if t.elem.size%uintptr(t.elem.align) != 0 {
		throw("elem size not a multiple of elem align")
	}
	if bucketCnt < 8 {
		throw("bucketsize too small for proper alignment")
	}
	if dataOffset%uintptr(t.key.align) != 0 {
		throw("need padding in bucket (key)")
	}
	if dataOffset%uintptr(t.elem.align) != 0 {
		throw("need padding in bucket (elem)")
	}

	return makemap(t, cap, nil)
}

//go:linkname reflect_mapaccess reflect.mapaccess
func reflect_mapaccess(t *maptype, h *hmap, key unsafe.Pointer) unsafe.Pointer {
	elem, ok := mapaccess2(t, h, key)
	if !ok {
		// reflect wants nil for a missing element
		elem = nil
	}
	return elem
}

//go:linkname reflect_mapassign reflect.mapassign
func reflect_mapassign(t *maptype, h *hmap, key unsafe.Pointer, elem unsafe.Pointer) {
	p := mapassign(t, h, key)
	typedmemmove(t.elem, p, elem)
}

//go:linkname reflect_mapdelete reflect.mapdelete
func reflect_mapdelete(t *maptype, h *hmap, key unsafe.Pointer) {
	mapdelete(t, h, key)
}

//go:linkname reflect_mapiterinit reflect.mapiterinit
func reflect_mapiterinit(t *maptype, h *hmap) *hiter {
	it := new(hiter)
	mapiterinit(t, h, it)
	return it
}

//go:linkname reflect_mapiternext reflect.mapiternext
func reflect_mapiternext(it *hiter) {
	mapiternext(it)
}

//go:linkname reflect_mapiterkey reflect.mapiterkey
func reflect_mapiterkey(it *hiter) unsafe.Pointer {
	return it.key
}

//go:linkname reflect_mapiterelem reflect.mapiterelem
func reflect_mapiterelem(it *hiter) unsafe.Pointer {
	return it.elem
}

//go:linkname reflect_maplen reflect.maplen
func reflect_maplen(h *hmap) int {
	if h == nil {
		return 0
	}
	if raceenabled {
		callerpc := getcallerpc()
		racereadpc(unsafe.Pointer(h), callerpc, funcPC(reflect_maplen))
	}
	return h.count
}

//go:linkname reflectlite_maplen internal/reflectlite.maplen
func reflectlite_maplen(h *hmap) int {
	if h == nil {
		return 0
	}
	if raceenabled {
		callerpc := getcallerpc()
		racereadpc(unsafe.Pointer(h), callerpc, funcPC(reflect_maplen))
	}
	return h.count
}

const maxZero = 1024 // must match value in cmd/compile/internal/gc/walk.go:zeroValSize
var zeroVal [maxZero]byte
```