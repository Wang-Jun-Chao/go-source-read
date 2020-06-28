```go

// Package list implements a doubly linked list.
//
// To iterate over a list (where l is a *List):
//	for e := l.Front(); e != nil; e = e.Next() {
//		// do something with e.Value
//	}
//
// 包list实现了双向链接列表。
//
// 要遍历list（其中l是* List）：
//  for e := l.Front(); e != nil; e = e.Next() {
//      // 用e.Value做某事
// }

package list

// Element is an element of a linked list.
// Element是链接列表的元素。
type Element struct {
	// Next and previous pointers in the doubly-linked list of elements.
	// To simplify the implementation, internally a list l is implemented
	// as a ring, such that &l.root is both the next element of the last
	// list element (l.Back()) and the previous element of the first list
	// element (l.Front()).
	//
	// 双向链接的元素列表中的下一个和上一个指针。
    // 为了简化实现，在内部实现了列表l
    // 作为一个环，使得＆l.root既是最后一个列表元素（l.Back（））的下一个元素，也是第一个列表元素（l.Front（））的前一个元素。
	next, prev *Element

	// The list to which this element belongs.
	// 此元素所属的链表。
	list *List

	// The value stored with this element.
	// 与此元素一起存储的值。
	Value interface{}
}

// Next returns the next list element or nil.
// Next返回下一个列表元素或nil。
func (e *Element) Next() *Element {
	if p := e.next; e.list != nil && p != &e.list.root {
		return p
	}
	return nil
}

// Prev returns the previous list element or nil.
// Prev返回上一个列表元素或nil。
func (e *Element) Prev() *Element {
	if p := e.prev; e.list != nil && p != &e.list.root {
		return p
	}
	return nil
}

// List represents a doubly linked list.
// The zero value for List is an empty list ready to use.
// List表示一个双向链表。
// List的零值是可以使用的空列表。
type List struct {
    // 哨兵列表元素，仅使用＆root，root.prev和root.next
	root Element // sentinel list element, only &root, root.prev, and root.next are used
	// 当前列表长度，不包括（此）哨兵元素
	len  int     // current list length excluding (this) sentinel element
}

// Init initializes or clears list l.
// Init初始化或清除列表l。
func (l *List) Init() *List {
	l.root.next = &l.root
	l.root.prev = &l.root
	l.len = 0
	return l
}

// New returns an initialized list.
// New返回一个初始化列表。
func New() *List { return new(List).Init() }

// Len returns the number of elements of list l.
// The complexity is O(1).
// Len返回列表l的元素数。
// 复杂度为O（1）。
func (l *List) Len() int { return l.len }

// Front returns the first element of list l or nil if the list is empty.
// 如果列表为空返回nil，否则Front返回列表l第一个元素。
func (l *List) Front() *Element {
	if l.len == 0 {
		return nil
	}
	return l.root.next
}

// Back returns the last element of list l or nil if the list is empty.
// Back返回列表l的最后一个元素；如果列表为空，则返回nil。
func (l *List) Back() *Element {
	if l.len == 0 {
		return nil
	}
	return l.root.prev
}

// lazyInit lazily initializes a zero List value.
// lazyInit延迟初始化一个零列表值。
func (l *List) lazyInit() {
	if l.root.next == nil {
		l.Init()
	}
}

// insert inserts e after at, increments l.len, and returns e.
// insert在at后面插入e，递增l.len，然后返回e。
func (l *List) insert(e, at *Element) *Element {
	n := at.next
	at.next = e
	e.prev = at
	e.next = n
	n.prev = e
	e.list = l
	l.len++
	return e
}

// insertValue is a convenience wrapper for insert(&Element{Value: v}, at).
// insertValue是insert(&Element{Value: v}, at)的便捷包装。
func (l *List) insertValue(v interface{}, at *Element) *Element {
	return l.insert(&Element{Value: v}, at)
}

// remove removes e from its list, decrements l.len, and returns e.
// remove从列表中删除e，递减l.len，然后返回e。
func (l *List) remove(e *Element) *Element {
	e.prev.next = e.next
	e.next.prev = e.prev
	e.next = nil // avoid memory leaks
	e.prev = nil // avoid memory leaks
	e.list = nil
	l.len--
	return e
}

// move moves e to next to at and returns e.
// move将e移至at的后面并返回e。
func (l *List) move(e, at *Element) *Element {
	if e == at {
		return e
	}
	e.prev.next = e.next
	e.next.prev = e.prev

	n := at.next
	at.next = e
	e.prev = at
	e.next = n
	n.prev = e

	return e
}

// Remove removes e from l if e is an element of list l.
// It returns the element value e.Value.
// The element must not be nil.
//
// 如果e是列表l的元素，则Remove从l中删除e。
// 返回元素值e.Value。
// 元素不能为nil。
func (l *List) Remove(e *Element) interface{} {
	if e.list == l {
		// if e.list == l, l must have been initialized when e was inserted
		// in l or l == nil (e is a zero Element) and l.remove will crash
		// 如果e.list == l，则必须在将e插入l或l == nil（e为零元素）时初始化l，否则l.remove将崩溃
		l.remove(e)
	}
	return e.Value
}

// PushFront inserts a new element e with value v at the front of list l and returns e.
// PushFront在列表l的开头插入一个值为v的新元素e并返回e。
func (l *List) PushFront(v interface{}) *Element {
	l.lazyInit()
	return l.insertValue(v, &l.root)
}

// PushBack inserts a new element e with value v at the back of list l and returns e.
// PushBack在列表l的后面插入一个值为v的新元素e并返回e。
func (l *List) PushBack(v interface{}) *Element {
	l.lazyInit()
	return l.insertValue(v, l.root.prev)
}

// InsertBefore inserts a new element e with value v immediately before mark and returns e.
// If mark is not an element of l, the list is not modified.
// The mark must not be nil.
// InsertBefore在紧接mark之前插入一个值为v的新元素e并返回e。
// 如果mark不是l的元素，则列表不会被修改。
// mark不能为nil。
func (l *List) InsertBefore(v interface{}, mark *Element) *Element {
	if mark.list != l {
		return nil
	}
	// see comment in List.Remove about initialization of l
	// 查看List中的注释。Remove有关l的初始化的信息
	return l.insertValue(v, mark.prev)
}

// InsertAfter inserts a new element e with value v immediately after mark and returns e.
// If mark is not an element of l, the list is not modified.
// The mark must not be nil.
// InsertAfter在标记后立即插入一个值为v的新元素e并返回e。
// 如果mark不是l的元素，则列表不会被修改。
// mark不能为nil。
func (l *List) InsertAfter(v interface{}, mark *Element) *Element {
	if mark.list != l {
		return nil
	}
	// see comment in List.Remove about initialization of l
	// 查看List中的注释。Remove有关l的初始化的信息
	return l.insertValue(v, mark)
}

// MoveToFront moves element e to the front of list l.
// If e is not an element of l, the list is not modified.
// The element must not be nil.
// MoveToFront将元素e移动到列表l的前面。
// 如果e不是l的元素，则列表不会被修改。
// e不能为nil。
func (l *List) MoveToFront(e *Element) {
	if e.list != l || l.root.next == e {
		return
	}
	// see comment in List.Remove about initialization of l
	l.move(e, &l.root)
}

// MoveToBack moves element e to the back of list l.
// If e is not an element of l, the list is not modified.
// The element must not be nil.
// MoveToFront将元素e移动到列表l的后面。
// 如果e不是l的元素，则列表不会被修改。
// e不能为nil。
func (l *List) MoveToBack(e *Element) {
	if e.list != l || l.root.prev == e {
		return
	}
	// see comment in List.Remove about initialization of l
	l.move(e, l.root.prev)
}

// MoveBefore moves element e to its new position before mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
// The element and mark must not be nil.
// MoveBefore将元素e移到标记之前的新位置。
// 如果e或mark不是l的元素或e == mark，则不修改列表。
// e和mark不得为nil。
func (l *List) MoveBefore(e, mark *Element) {
	if e.list != l || e == mark || mark.list != l {
		return
	}
	l.move(e, mark.prev)
}

// MoveAfter moves element e to its new position after mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
// The element and mark must not be nil.
// MoveBefore将元素e移到标记之后的新位置。
// 如果e或mark不是l的元素或e == mark，则不修改列表。
// e和mark不得为nil。
func (l *List) MoveAfter(e, mark *Element) {
	if e.list != l || e == mark || mark.list != l {
		return
	}
	l.move(e, mark)
}

// PushBackList inserts a copy of an other list at the back of list l.
// The lists l and other may be the same. They must not be nil.
// PushBackList在列表l的后面插入另一个列表的副本。
// 列表l和其他列表可能相同。 他们一定不能为nil。
func (l *List) PushBackList(other *List) {
	l.lazyInit()
	for i, e := other.Len(), other.Front(); i > 0; i, e = i-1, e.Next() {
		l.insertValue(e.Value, l.root.prev)
	}
}

// PushFrontList inserts a copy of an other list at the front of list l.
// The lists l and other may be the same. They must not be nil.
// PushFrontList在列表l的前面插入另一个列表的副本。
// 列表l和其他列表可能相同。 他们一定不能为nil。
func (l *List) PushFrontList(other *List) {
	l.lazyInit()
	for i, e := other.Len(), other.Back(); i > 0; i, e = i-1, e.Prev() {
		l.insertValue(e.Value, &l.root)
	}
}
```