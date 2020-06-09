```go

//go:generate go run genzfunc.go

// Package sort provides primitives for sorting slices and user-defined
// collections.
// sort包提供用于对切片和用户定义的集合进行排序的原语。
package sort

// A type, typically a collection, that satisfies sort.Interface can be
// sorted by the routines in this package. The methods require that the
// elements of the collection be enumerated by an integer index.
// 一个满足sort的类型，通常是一个collection.Interface可以通过此包中的例程进行排序。 这些方法要求集合的元素由整数索引枚举。
type Interface interface {
	// Len is the number of elements in the collection.
	// Len是集合中元素的数量。
	Len() int
	// Less reports whether the element with
	// index i should sort before the element with index j.
	// Less报告是否将索引为i的元素排在索引为j的元素之前。
	Less(i, j int) bool
	// Swap swaps the elements with indexes i and j.
	// Swap交换索引为i和j的元素。
	Swap(i, j int)
}

// Insertion sort
// 插入排序
func insertionSort(data Interface, a, b int) {
	for i := a + 1; i < b; i++ {
		for j := i; j > a && data.Less(j, j-1); j-- {
			data.Swap(j, j-1)
		}
	}
}

// siftDown implements the heap property on data[lo, hi).
// first is an offset into the array where the root of the heap lies.
// siftDown在data [lo，hi）上实现了heap属性。 第一个是堆根所在的数组的偏移量。
func siftDown(data Interface, lo, hi, first int) {
	root := lo
	for {
		child := 2*root + 1
		if child >= hi {
			break
		}
		if child+1 < hi && data.Less(first+child, first+child+1) {
			child++
		}
		if !data.Less(first+root, first+child) {
			return
		}
		data.Swap(first+root, first+child)
		root = child
	}
}

func heapSort(data Interface, a, b int) {
	first := a
	lo := 0
	hi := b - a

	// Build heap with greatest element at top.
	// 构建堆，最大的元素在顶部。
	for i := (hi - 1) / 2; i >= 0; i-- {
		siftDown(data, i, hi, first)
	}

	// Pop elements, largest first, into end of data.
	// 将元素（从大到大）弹出，插入到数据末尾。
	for i := hi - 1; i >= 0; i-- {
		data.Swap(first, first+i)
		siftDown(data, lo, i, first)
	}
}

// Quicksort, loosely following Bentley and McIlroy,
// ``Engineering a Sort Function,'' SP&E November 1993.

// medianOfThree moves the median of the three values data[m0], data[m1], data[m2] into data[m1].
// meanOfThree将三个值data [m0]，data [m1]，data [m2]的中值移动到data [m1]。
func medianOfThree(data Interface, m1, m0, m2 int) {
	// sort 3 elements // 排序三个元素
	if data.Less(m1, m0) {
		data.Swap(m1, m0)
	}
	// data[m0] <= data[m1]
	if data.Less(m2, m1) {
		data.Swap(m2, m1)
		// data[m0] <= data[m2] && data[m1] < data[m2]
		if data.Less(m1, m0) {
			data.Swap(m1, m0)
		}
	}
	// now data[m0] <= data[m1] <= data[m2]
}

func swapRange(data Interface, a, b, n int) {
	for i := 0; i < n; i++ {
		data.Swap(a+i, b+i)
	}
}

func doPivot(data Interface, lo, hi int) (midlo, midhi int) {
	m := int(uint(lo+hi) >> 1) // Written like this to avoid integer overflow. // 这样写是为了避免整数溢出。
	if hi-lo > 40 {
		// Tukey's ``Ninther,'' median of three medians of three.
		s := (hi - lo) / 8
		medianOfThree(data, lo, lo+s, lo+2*s)
		medianOfThree(data, m, m-s, m+s)
		medianOfThree(data, hi-1, hi-1-s, hi-1-2*s)
	}
	medianOfThree(data, lo, m, hi-1)

	// Invariants are:  // 不变量条件：
	//	data[lo] = pivot (set up by ChoosePivot)
	//	data[lo < i < a] < pivot
	//	data[a <= i < b] <= pivot
	//	data[b <= i < c] unexamined
	//	data[c <= i < hi-1] > pivot
	//	data[hi-1] >= pivot
	pivot := lo
	a, c := lo+1, hi-1

	for ; a < c && data.Less(a, pivot); a++ { // data[a] < pivot
	}
	b := a
	for {
		for ; b < c && !data.Less(pivot, b); b++ { // data[b] <= pivot
		}
		for ; b < c && data.Less(pivot, c-1); c-- { // data[c-1] > pivot
		}
		if b >= c {
			break
		}
		// data[b] > pivot; data[c-1] <= pivot; b < c; 要交换
		data.Swap(b, c-1)
		b++
		c--
	}
	// If hi-c<3 then there are duplicates (by property of median of nine).
	// Let's be a bit more conservative, and set border to 5.
	// 如果hi-c<3，则存在重复项（按中位数9的属性）。 让我们更加保守一些，将border设置为5。
	protect := hi-c < 5
	if !protect && hi-c < (hi-lo)/4 {
		// Lets test some points for equality to pivot
		// 让我们测试一些要点是否与pivot相等
		dups := 0
		if !data.Less(pivot, hi-1) { // data[hi-1] = pivot
			data.Swap(c, hi-1)
			c++
			dups++
		}
		if !data.Less(b-1, pivot) { // data[b-1] = pivot
			b--
			dups++
		}
		// m-lo = (hi-lo)/2 > 6
		// b-lo > (hi-lo)*3/4-1 > 8
		// ==> m < b ==> data[m] <= pivot
		if !data.Less(m, pivot) { // data[m] = pivot
			data.Swap(m, b-1)
			b--
			dups++
		}
		// if at least 2 points are equal to pivot, assume skewed distribution
		protect = dups > 1
	}
	if protect {
		// Protect against a lot of duplicates
		// Add invariant:
		//	data[a <= i < b] unexamined
		//	data[b <= i < c] = pivot
		for {
			for ; a < b && !data.Less(b-1, pivot); b-- { // data[b] == pivot
			}
			for ; a < b && data.Less(a, pivot); a++ { // data[a] < pivot
			}
			if a >= b {
				break
			}
			// data[a] == pivot; data[b-1] < pivot
			data.Swap(a, b-1)
			a++
			b--
		}
	}
	// Swap pivot into middle
	data.Swap(pivot, b-1)
	return b - 1, c
}

func quickSort(data Interface, a, b, maxDepth int) {
	for b-a > 12 { // Use ShellSort for slices <= 12 elements // 将ShellSort用于切片 <= 12个元素
		if maxDepth == 0 {
			heapSort(data, a, b)
			return
		}
		maxDepth--
		mlo, mhi := doPivot(data, a, b)
		// Avoiding recursion on the larger subproblem guarantees
		// a stack depth of at most lg(b-a).
		// 避免在较大的子问题上进行递归可确保堆栈深度最多为lg(b-a)。
		if mlo-a < b-mhi {
			quickSort(data, a, mlo, maxDepth)
			a = mhi // i.e., quickSort(data, mhi, b)
		} else {
			quickSort(data, mhi, b, maxDepth)
			b = mlo // i.e., quickSort(data, a, mlo)
		}
	}
	if b-a > 1 {
		// Do ShellSort pass with gap 6
		// It could be written in this simplified form cause b-a <= 12
		// 以间隙6进行ShellSort排序。可以用这种简化形式编写，因为b-a <= 12
		for i := a + 6; i < b; i++ {
			if data.Less(i, i-6) {
				data.Swap(i, i-6)
			}
		}
		insertionSort(data, a, b)
	}
}

// Sort sorts data.
// It makes one call to data.Len to determine n, and O(n*log(n)) calls to
// data.Less and data.Swap. The sort is not guaranteed to be stable.
// Sort对数据进行排序。一次调用data.Len确定n，然后调用O（n * log（n））调用data.Less和data.Swap。不能保证排序是稳定的。
func Sort(data Interface) {
	n := data.Len()
	quickSort(data, 0, n, maxDepth(n))
}

// maxDepth returns a threshold at which quicksort should switch
// to heapsort. It returns 2*ceil(lg(n+1)).
// maxDepth返回一个阈值，在该阈值处，快速排序应切换为堆排序。 它返回 2*ceil(lg(n+1))。
func maxDepth(n int) int {
	var depth int
	for i := n; i > 0; i >>= 1 {
		depth++
	}
	return depth * 2
}

// lessSwap is a pair of Less and Swap function for use with the
// auto-generated func-optimized variant of sort.go in
// zfuncversion.go.
// lessSwap是一对Less and Swap函数，可与zfuncversion.go中自动生成的func优化的sort.go变体一起使用。
type lessSwap struct {
	Less func(i, j int) bool
	Swap func(i, j int)
}

type reverse struct {
	// This embedded Interface permits Reverse to use the methods of
	// another Interface implementation.
	// 此嵌入式接口允许Reverse使用另一个Interface实现的方法。
	Interface
}

// Less returns the opposite of the embedded implementation's Less method.
// Less返回与嵌入式实现的Less方法相反的方法。
func (r reverse) Less(i, j int) bool {
	return r.Interface.Less(j, i)
}

// Reverse returns the reverse order for data.
// Reverse返回数据的反向顺序。
func Reverse(data Interface) Interface {
	return &reverse{data}
}

// IsSorted reports whether data is sorted.
// IsSorted报告是否对数据进行排序。
func IsSorted(data Interface) bool {
	n := data.Len()
	for i := n - 1; i > 0; i-- {
		if data.Less(i, i-1) {
			return false
		}
	}
	return true
}

// Convenience types for common cases //常见情况下的便利类型

// IntSlice attaches the methods of Interface to []int, sorting in increasing order.
// IntSlice将Interface的方法附加到[]int上，并按升序排序。
type IntSlice []int

func (p IntSlice) Len() int           { return len(p) }
func (p IntSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p IntSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Sort is a convenience method.
// Sort是一种方便的方法。
func (p IntSlice) Sort() { Sort(p) }

// Float64Slice attaches the methods of Interface to []float64, sorting in increasing order
// (not-a-number values are treated as less than other values).
type Float64Slice []float64

func (p Float64Slice) Len() int           { return len(p) }
func (p Float64Slice) Less(i, j int) bool { return p[i] < p[j] || isNaN(p[i]) && !isNaN(p[j]) }
func (p Float64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// isNaN is a copy of math.IsNaN to avoid a dependency on the math package.
func isNaN(f float64) bool {
	return f != f
}

// Sort is a convenience method.
func (p Float64Slice) Sort() { Sort(p) }

// StringSlice attaches the methods of Interface to []string, sorting in increasing order.
type StringSlice []string

func (p StringSlice) Len() int           { return len(p) }
func (p StringSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p StringSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Sort is a convenience method.
func (p StringSlice) Sort() { Sort(p) }

// Convenience wrappers for common cases

// Ints sorts a slice of ints in increasing order.
func Ints(a []int) { Sort(IntSlice(a)) }

// Float64s sorts a slice of float64s in increasing order
// (not-a-number values are treated as less than other values).
func Float64s(a []float64) { Sort(Float64Slice(a)) }

// Strings sorts a slice of strings in increasing order.
func Strings(a []string) { Sort(StringSlice(a)) }

// IntsAreSorted tests whether a slice of ints is sorted in increasing order.
func IntsAreSorted(a []int) bool { return IsSorted(IntSlice(a)) }

// Float64sAreSorted tests whether a slice of float64s is sorted in increasing order
// (not-a-number values are treated as less than other values).
func Float64sAreSorted(a []float64) bool { return IsSorted(Float64Slice(a)) }

// StringsAreSorted tests whether a slice of strings is sorted in increasing order.
func StringsAreSorted(a []string) bool { return IsSorted(StringSlice(a)) }

// Notes on stable sorting:
// The used algorithms are simple and provable correct on all input and use
// only logarithmic additional stack space. They perform well if compared
// experimentally to other stable in-place sorting algorithms.
//
// Remarks on other algorithms evaluated:
//  - GCC's 4.6.3 stable_sort with merge_without_buffer from libstdc++:
//    Not faster.
//  - GCC's __rotate for block rotations: Not faster.
//  - "Practical in-place mergesort" from  Jyrki Katajainen, Tomi A. Pasanen
//    and Jukka Teuhola; Nordic Journal of Computing 3,1 (1996), 27-40:
//    The given algorithms are in-place, number of Swap and Assignments
//    grow as n log n but the algorithm is not stable.
//  - "Fast Stable In-Place Sorting with O(n) Data Moves" J.I. Munro and
//    V. Raman in Algorithmica (1996) 16, 115-160:
//    This algorithm either needs additional 2n bits or works only if there
//    are enough different elements available to encode some permutations
//    which have to be undone later (so not stable on any input).
//  - All the optimal in-place sorting/merging algorithms I found are either
//    unstable or rely on enough different elements in each step to encode the
//    performed block rearrangements. See also "In-Place Merging Algorithms",
//    Denham Coates-Evely, Department of Computer Science, Kings College,
//    January 2004 and the references in there.
//  - Often "optimal" algorithms are optimal in the number of assignments
//    but Interface has only Swap as operation.

// Stable sorts data while keeping the original order of equal elements.
//
// It makes one call to data.Len to determine n, O(n*log(n)) calls to
// data.Less and O(n*log(n)*log(n)) calls to data.Swap.
//
// 关于稳定排序的注意事项：
// 使用的算法在所有输入上都是简单且可证明正确的，并且仅使用对数的额外栈空间。
// 如果将其与其他稳定的就地排序算法进行实验比较，它们的性能会很好。
//
// 关于其他评估算法的说明：
//  - 来自libstdc ++的GCC 4.6.3 stable_sort与merge_without_buffer：不快。
//  - GCC的__rotate进行块旋转：不快。
//  - 来自Jyrki Katajainen，Tomi A. Pasanen和Jukka Teuhola的“实用就地合并排序”；
//    Nordic Journal of Computing 3,1（1996），27-40：
//    给定的算法就地，交换和分配的数量随着n log n的增加而增加，但是算法不稳定。
//  - “通过O（n）数据移动实现快速稳定的就地排序” Munro和V.Raman in Algorithmica（1996）16，115-160：
//    此算法要么需要额外的2n位，要么仅在有足够的可用元素来编码某些置换而后需要撤消时才起作用（因此对任何输入均不稳定））。
//  - 我发现所有最佳的就地排序/合并算法都是不稳定的，或者在每个步骤中都依赖于足够不同的元素来对执行的块重排进行编码。
//    另请参见2004年1月，金斯学院计算机科学系Denham Coates-Evely的“就地合并算法”及其中的参考文献。
//  - 通常，“最优”算法在分配数量上是最佳的，但是Interface仅将Swap作为操作。

// 稳定排序数据，同时保持相等元素的原始顺序。
//
// 一次调用data.Len确定n，对数据调用O(n*log(n))。Less和O(n*log(n)*log(n))调用data.Swap。
func Stable(data Interface) {
	stable(data, data.Len())
}

func stable(data Interface, n int) {
	blockSize := 20 // must be > 0
	a, b := 0, blockSize
	for b <= n { // 元素个数不大于20
		insertionSort(data, a, b)
		a = b
		b += blockSize
	}
	insertionSort(data, a, n)

	for blockSize < n { // 元素个数大于20
		a, b = 0, 2*blockSize
		for b <= n {
			symMerge(data, a, a+blockSize, b)
			a = b
			b += 2 * blockSize
		}
		if m := a + blockSize; m < n {
			symMerge(data, a, m, n)
		}
		blockSize *= 2
	}
}

// SymMerge merges the two sorted subsequences data[a:m] and data[m:b] using
// the SymMerge algorithm from Pok-Son Kim and Arne Kutzner, "Stable Minimum
// Storage Merging by Symmetric Comparisons", in Susanne Albers and Tomasz
// Radzik, editors, Algorithms - ESA 2004, volume 3221 of Lecture Notes in
// Computer Science, pages 714-723. Springer, 2004.
//
// Let M = m-a and N = b-n. Wolog M < N.
// The recursion depth is bound by ceil(log(N+M)).
// The algorithm needs O(M*log(N/M + 1)) calls to data.Less.
// The algorithm needs O((M+N)*log(M)) calls to data.Swap.
//
// The paper gives O((M+N)*log(M)) as the number of assignments assuming a
// rotation algorithm which uses O(M+N+gcd(M+N)) assignments. The argumentation
// in the paper carries through for Swap operations, especially as the block
// swapping rotate uses only O(M+N) Swaps.
//
// symMerge assumes non-degenerate arguments: a < m && m < b.
// Having the caller check this condition eliminates many leaf recursion calls,
// which improves performance.
//
// SymMerge使用来自Susanne Albers和Tomasz Radzik的Pok-Son Kim和Arne Kutzner的
// SymMerge算法合并了两个排序的子序列数据[a：m]和data [m：b]，“通过对称比较进行稳定的最小存储合并” ，
// 编辑，算法-ESA 2004，计算机科学讲义第3221卷，第714-723页。施普林格，2004年。
//
// 令M = m-a和N = b-n。 Wolog M <N。
// 递归深度受ceil（log（N + M））约束。
// 该算法需要对数据进行O（M * log（N / M + 1））次调用。
// 该算法需要O（（M + N）* log（M））调用data.Swap。
//
// 假设使用O（M + N + gcd（M + N））分配的旋转算法，本文给出O（（M + N）* log（M））作为分配数量。
// 本文中的论点适用于交换操作，尤其是当块交换旋转仅使用O（M + N）交换时。
//
// symMerge假定非简并的参数：a <m && m <b。
// 让调用者检查此条件可以消除许多叶递归调用，从而提高了性能。
func symMerge(data Interface, a, m, b int) {
	// Avoid unnecessary recursions of symMerge
	// by direct insertion of data[a] into data[m:b]
	// if data[a:m] only contains one element.
	// 如果data[a:m]仅包含一个元素，则通过将data[a]直接插入data[m:b]来避免symMerge的不必要的递归。
	if m-a == 1 {
		// Use binary search to find the lowest index i
		// such that data[i] >= data[a] for m <= i < b.
		// Exit the search loop with i == b in case no such index exists.
		// 使用二分搜索找到最低的索引i，以使data [i]> = data [a]的m <= i <b。 如果不存在这样的索引，请使用i == b退出搜索循环。
		i := m
		j := b
		for i < j {
			h := int(uint(i+j) >> 1)
			if data.Less(h, a) {
				i = h + 1
			} else {
				j = h
			}
		}
		// Swap values until data[a] reaches the position before i.
		// 交换值，直到data[a]到达i之前的位置。
		for k := a; k < i-1; k++ {
			data.Swap(k, k+1)
		}
		return
	}

	// Avoid unnecessary recursions of symMerge
	// by direct insertion of data[m] into data[a:m]
	// if data[m:b] only contains one element.
	// 如果data[a:m]仅包含一个元素，则通过将data[a]直接插入data[m:b]来避免symMerge的不必要的递归。
	if b-m == 1 {
		// Use binary search to find the lowest index i
		// such that data[i] > data[m] for a <= i < m.
		// Exit the search loop with i == m in case no such index exists.
		// 使用二分搜索找到最低的索引i，对于a <= i < m，以使data[i]>data[m]。 如果没有这样的索引，则以i == m 退出搜索循环。
		i := a
		j := m
		for i < j {
			h := int(uint(i+j) >> 1)
			if !data.Less(m, h) {
				i = h + 1
			} else {
				j = h
			}
		}
		// Swap values until data[m] reaches the position i.
		// 交换值，直到data [m]到达位置i。
		for k := m; k > i; k-- {
			data.Swap(k, k-1)
		}
		return
	}

	mid := int(uint(a+b) >> 1)
	n := mid + m
	var start, r int
	if m > mid {
		start = n - b
		r = mid
	} else {
		start = a
		r = m
	}
	p := n - 1

	for start < r {
		c := int(uint(start+r) >> 1)
		if !data.Less(p-c, c) {
			start = c + 1
		} else {
			r = c
		}
	}

	end := n - start
	if start < m && m < end {
		rotate(data, start, m, end)
	}
	if a < start && start < mid {
		symMerge(data, a, start, mid)
	}
	if mid < end && end < b {
		symMerge(data, mid, end, b)
	}
}

// Rotate two consecutive blocks u = data[a:m] and v = data[m:b] in data:
// Data of the form 'x u v y' is changed to 'x v u y'.
// Rotate performs at most b-a many calls to data.Swap.
// Rotate assumes non-degenerate arguments: a < m && m < b.
// 旋转数据中的两个连续块u = data [a：m]和v = data [m：b]：
// 格式为'x u v y'的数据更改为'x v u y'。
// Rotate最多执行b-a次调用data.Swap。
// Rotate假定非简并参数：a <m && m <b。
func rotate(data Interface, a, m, b int) {
	i := m - a
	j := b - m

	for i != j {
		if i > j {
			swapRange(data, m-i, m, j)
			i -= j
		} else {
			swapRange(data, m-i, m+j-i, i)
			j -= i
		}
	}
	// i == j
	swapRange(data, m-i, m, i)
}

/*
Complexity of Stable Sorting


Complexity of block swapping rotation

Each Swap puts one new element into its correct, final position.
Elements which reach their final position are no longer moved.
Thus block swapping rotation needs |u|+|v| calls to Swaps.
This is best possible as each element might need a move.

Pay attention when comparing to other optimal algorithms which
typically count the number of assignments instead of swaps:
E.g. the optimal algorithm of Dudzinski and Dydek for in-place
rotations uses O(u + v + gcd(u,v)) assignments which is
better than our O(3 * (u+v)) as gcd(u,v) <= u.


Stable sorting by SymMerge and BlockSwap rotations

SymMerg complexity for same size input M = N:
Calls to Less:  O(M*log(N/M+1)) = O(N*log(2)) = O(N)
Calls to Swap:  O((M+N)*log(M)) = O(2*N*log(N)) = O(N*log(N))

(The following argument does not fuzz over a missing -1 or
other stuff which does not impact the final result).

Let n = data.Len(). Assume n = 2^k.

Plain merge sort performs log(n) = k iterations.
On iteration i the algorithm merges 2^(k-i) blocks, each of size 2^i.

Thus iteration i of merge sort performs:
Calls to Less  O(2^(k-i) * 2^i) = O(2^k) = O(2^log(n)) = O(n)
Calls to Swap  O(2^(k-i) * 2^i * log(2^i)) = O(2^k * i) = O(n*i)

In total k = log(n) iterations are performed; so in total:
Calls to Less O(log(n) * n)
Calls to Swap O(n + 2*n + 3*n + ... + (k-1)*n + k*n)
   = O((k/2) * k * n) = O(n * k^2) = O(n * log^2(n))


Above results should generalize to arbitrary n = 2^k + p
and should not be influenced by the initial insertion sort phase:
Insertion sort is O(n^2) on Swap and Less, thus O(bs^2) per block of
size bs at n/bs blocks:  O(bs*n) Swaps and Less during insertion sort.
Merge sort iterations start at i = log(bs). With t = log(bs) constant:
Calls to Less O((log(n)-t) * n + bs*n) = O(log(n)*n + (bs-t)*n)
   = O(n * log(n))
Calls to Swap O(n * log^2(n) - (t^2+t)/2*n) = O(n * log^2(n))

稳定排序的复杂性


块交换旋转的复杂性

每个Swap将一个新元素放入其正确的最终位置。
达到最终位置的元素不再移动。
因此，块交换旋转需要| u | + | v |次调用Swap。
这是最好的可能，因为每个元素可能都需要移动。

与其他通常计算分配次数而不是交换次数的最佳算法进行比较时，请注意：
例如。 Dudzinski和Dydek用于原位旋转的最佳算法使用O（u + v + gcd（u，v））分配，这优于我们的O（3 *（u + v））作为gcd（u，v）<= u


通过SymMerge和BlockSwap旋转稳定排序

对于相同大小的输入，SymMerg复杂度M = N：
调用Less：O（M * log（N / M + 1））= O（N * log（2））= O（N）
调用Swap：O（（M + N）* log（M））= O（2 * N * log（N））= O（N * log（N））

（以下参数不会模糊missing-1或其他不会影响最终结果的东西）。

令n = data.Len（）。假设n = 2 ^ k。

普通合并排序执行log（n）= k次迭代。
在迭代i时，算法合并2 ^（k-i）个块，每个块的大小为2 ^ i。

因此，合并排序的迭代i执行：
调用Less O（2 ^（k-i）* 2 ^ i）= O（2 ^ k）= O（2 ^ log（n））= O（n）
调用Swap O（2 ^（k-i）* 2 ^ i * log（2 ^ i））= O（2 ^ k * i）= O（n * i）

总共执行k = log（n）次迭代；因此总计：
调用Less O（log（n）* n）
调用Swap O（n + 2 * n + 3 * n + ... +（k-1）* n + k * n）= O（（k / 2）* k * n）= O（n * k ^ 2）= O（n * log ^ 2（n））


以上结果应推广为任意n = 2 ^ k + p
并且不受初始插入排序阶段的影响：
插入排序在Swap和Less上为O（n ^ 2），因此在n/bs块上每bs块的大小为O（bs ^ 2）：O（bs * n）在插入排序过程中掉期并减少。
合并排序迭代从i = log（bs）开始。用t = log（bs）常数：
调用Less O（（log（n）-t）* n + bs * n）= O（log（n）* n +（bs-t）* n）= O（n * log（n））
调用Swap O（n * log ^ 2（n）-（t ^ 2 + t）/ 2 * n）= O（n * log ^ 2（n））

*/
```