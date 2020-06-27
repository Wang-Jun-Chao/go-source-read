```go
// Package heap provides heap operations for any type that implements
// heap.Interface. A heap is a tree with the property that each node is the
// minimum-valued node in its subtree.
//
// The minimum element in the tree is the root, at index 0.
//
// A heap is a common way to implement a priority queue. To build a priority
// queue, implement the Heap interface with the (negative) priority as the
// ordering for the Less method, so Push adds items while Pop removes the
// highest-priority item from the queue. The Examples include such an
// implementation; the file example_pq_test.go has the complete source.
//
// 包heap为实现了heap.Interface的任何类型提供堆操作。 堆是一棵树，其属性为每个节点是其子树中的最小值节点。
//
// 树中的最小元素是根，索引为0。
//
// 堆是实现优先级队列的常用方法。 要构建优先级队列，请以（负）优先级实现Heap接口，
// 作为Less方法的顺序，因此Push添加项，而Pop删除队列中优先级最高的项。
// 示例包括这样的实现； 文件example_pq_test.go包含完整的源代码。
package heap

import "sort"

// The Interface type describes the requirements
// for a type using the routines in this package.
// Any type that implements it may be used as a
// min-heap with the following invariants (established after
// Init has been called or if the data is empty or sorted):
//
//	!h.Less(j, i) for 0 <= i < h.Len() and 2*i+1 <= j <= 2*i+2 and j < h.Len()
//
// Note that Push and Pop in this interface are for package heap's
// implementation to call. To add and remove things from the heap,
// use heap.Push and heap.Pop.
//
// Interface类型使用此包中的例程描述对类型的要求。
// 任何实现它的类型都可以用作带有以下不变量的最小堆（在调用Init或数据为空或排序后建立）。
//
// !h.Less（j，i）为0 <= i <h.Len（）和2 * i + 1 <= j <= 2 * i + 2和j <h.Len（）
//
// 注意，此接口中的Push和Pop用于包堆的实现调用。 要从堆中添加和删除内容，请使用heap.Push和heap.Pop。
type Interface interface {
	sort.Interface
	Push(x interface{}) // add x as element Len() // 将x添加为元素Len()
	Pop() interface{}   // remove and return element Len() - 1. // 删除并返回元素Len()-1。
}

// Init establishes the heap invariants required by the other routines in this package.
// Init is idempotent with respect to the heap invariants
// and may be called whenever the heap invariants may have been invalidated.
// The complexity is O(n) where n = h.Len().
//
// Init建立此程序包中其他例程所需的堆不变式。
// 相对于堆不变式，Init是幂等的，只要使堆不变式无效，就可以调用它。
// 复杂度为O(n)，其中n = h.Len()。
func Init(h Interface) {
	// heapify // 创建堆
	n := h.Len()
	for i := n/2 - 1; i >= 0; i-- {
		down(h, i, n)
	}
}

// Push pushes the element x onto the heap.
// The complexity is O(log n) where n = h.Len().
// Push将元素x推入堆中。
// 复杂度为O(log n)，其中n = h.Len()。
func Push(h Interface, x interface{}) {
	h.Push(x)
	up(h, h.Len()-1)
}

// Pop removes and returns the minimum element (according to Less) from the heap.
// The complexity is O(log n) where n = h.Len().
// Pop is equivalent to Remove(h, 0).
//
// Pop从堆中删除并返回最小元素（根据Less）。
// 复杂度为O（log n），其中n = h.Len（）。
// Pop等效于Remove（h，0）。
func Pop(h Interface) interface{} {
	n := h.Len() - 1
	h.Swap(0, n)
	down(h, 0, n)
	return h.Pop()
}

// Remove removes and returns the element at index i from the heap.
// The complexity is O(log n) where n = h.Len().
//
// Remove从堆中删除并返回索引i处的元素。
// 复杂度为O（log n），其中n = h.Len（）。
func Remove(h Interface, i int) interface{} {
	n := h.Len() - 1
	if n != i {
		h.Swap(i, n)
		if !down(h, i, n) {
			up(h, i)
		}
	}
	return h.Pop()
}

// Fix re-establishes the heap ordering after the element at index i has changed its value.
// Changing the value of the element at index i and then calling Fix is equivalent to,
// but less expensive than, calling Remove(h, i) followed by a Push of the new value.
// The complexity is O(log n) where n = h.Len().
//
// Fix在索引i的元素更改其值后重新建立堆顺序。
// 更改索引i处元素的值，然后调用Fix等效于调用，Remove（h，i）紧随其后的是推入新值，但代价更低。
// 复杂度为O（log n），其中n = h.Len（）。
func Fix(h Interface, i int) {
	if !down(h, i, h.Len()) {
		up(h, i)
	}
}

// 向上冒泡，通过Less比较，小的向上冒泡
func up(h Interface, j int) {
	for {
		i := (j - 1) / 2 // parent // 父元素
		if i == j || !h.Less(j, i) { // 子元素大于等于父元素，退出
			break
		}
		h.Swap(i, j) // 交换父亲和孩子
		j = i
	}
}

// 向下沉，通过Less比较，大的向下沉
func down(h Interface, i0, n int) bool {
	i := i0
	for {
		j1 := 2*i + 1 // 左孩子
		if j1 >= n || j1 < 0 { // j1 < 0 after int overflow // j1 < 0 表示整数溢出，j1 >= n 表示已经处理完
			break
		}
		j := j1 // left child // j暂时为左孩子
		if j2 := j1 + 1; j2 < n && h.Less(j2, j1) { // 如果i有右孩子，并且右孩子比左孩子小，j取右孩子
			j = j2 // = 2*i + 2  // right child // 右孩子
		}
		if !h.Less(j, i) { // 孩子大于等于父亲，交换结束
			break
		}
		h.Swap(i, j) // 进行交换
		i = j // 更新新的处理节点
	}
	return i > i0 // 表示有交换操作
}
```