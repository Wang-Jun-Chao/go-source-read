```go

package sync

import (
	"sync/atomic"
	"unsafe"
)

// 参考：https://zhuanlan.zhihu.com/p/99710992

// poolDequeue is a lock-free fixed-size single-producer,
// multi-consumer queue. The single producer can both push and pop
// from the head, and consumers can pop from the tail.
//
// It has the added feature that it nils out unused slots to avoid
// unnecessary retention of objects. This is important for sync.Pool,
// but not typically a property considered in the literature.
// poolDequeue是一个无锁的固定大小的单生产者，多消费者队列。 单个生产者可以从头顶推和弹出，而消费者可以从尾巴弹出。
//
// 它具有增加的功能，它可以清除未使用的插槽，以避免不必要的对象保留。 这对于sync.Pool很重要，但通常不是文献中考虑的属性。
type poolDequeue struct {
	// headTail packs together a 32-bit head index and a 32-bit
	// tail index. Both are indexes into vals modulo len(vals)-1.
	//
	// tail = index of oldest data in queue
	// head = index of next slot to fill
	//
	// Slots in the range [tail, head) are owned by consumers.
	// A consumer continues to own a slot outside this range until
	// it nils the slot, at which point ownership passes to the
	// producer.
	//
	// The head index is stored in the most-significant bits so
	// that we can atomically add to it and the overflow is
	// harmless.
	//
	// headTail将32位的head索引和32位的tail索引打包在一起。 两者都是以len（vals）-1为模的vals的索引。
    //
    // tail = 队列中最旧数据的索引
    // head = 要填充的下一个插槽的索引
    //
    // 范围为[tail，head]的插槽归消费者所有。
    // 消费者继续拥有超出此范围的插槽，直到该插槽无效为止，此时所有权转移给了生产者。
    //
    // 头索引存储在最高有效位中，以便我们可以原子地添加到头索引中，并且溢出是无害的。
	headTail uint64

	// vals is a ring buffer of interface{} values stored in this
	// dequeue. The size of this must be a power of 2.
	//
	// vals[i].typ is nil if the slot is empty and non-nil
	// otherwise. A slot is still in use until *both* the tail
	// index has moved beyond it and typ has been set to nil. This
	// is set to nil atomically by the consumer and read
	// atomically by the producer.
	//
	// vals是存储在此出队中的interface {}值的环形缓冲区。 此大小必须是2的幂。
    //
    // 如果插槽为空，则vals[i].typ为零，否则为非零。 插槽仍在使用中，直到both和tail索引都超出了它，并且typ被设置为nil。 消费者原子地将其设置为零，生产者原子地将其设置值。
	vals []eface
}

type eface struct {
	typ, val unsafe.Pointer
}

const dequeueBits = 32

// dequeueLimit is the maximum size of a poolDequeue.
//
// This must be at most (1<<dequeueBits)/2 because detecting fullness
// depends on wrapping around the ring buffer without wrapping around
// the index. We divide by 4 so this fits in an int on 32-bit.
//
// dequeueLimit是poolDequeue的最大大小。
//
// 此值最多为（1 << dequeueBits）/ 2，因为检测是否填充取决于环绕环形缓冲区而不环绕索引。 我们除以4，这样就可以容纳32位整数。
const dequeueLimit = (1 << dequeueBits) / 4 // 0B01000000_0000000_0000000_0000000_0000000

// dequeueNil is used in poolDeqeue to represent interface{}(nil).
// Since we use nil to represent empty slots, we need a sentinel value
// to represent nil.
// dequeueNil在poolDeqeue中用于表示interface{}(nil)。 由于我们使用nil表示空插槽，因此我们需要一个哨兵值来表示nil。
type dequeueNil *struct{}

// 解head和tail的位置
func (d *poolDequeue) unpack(ptrs uint64) (head, tail uint32) {
	const mask = 1<<dequeueBits - 1
	head = uint32((ptrs >> dequeueBits) & mask)
	tail = uint32(ptrs & mask)
	return
}

// 打包head和tail
func (d *poolDequeue) pack(head, tail uint32) uint64 {
	const mask = 1<<dequeueBits - 1
	return (uint64(head) << dequeueBits) |
		uint64(tail&mask)
}

// pushHead adds val at the head of the queue. It returns false if the
// queue is full. It must only be called by a single producer.
// pushead在队列的开头添加val。 如果队列已满，则返回false。 它只能由单个生产者调用。
func (d *poolDequeue) pushHead(val interface{}) bool {
	ptrs := atomic.LoadUint64(&d.headTail)
	head, tail := d.unpack(ptrs)
	// 1<<dequeueBits-1 ==> 0b11111111_11111111_11111111_11111111
	// ---> tail ---> head  // [tail, head]之前存放数据
	// └     <----      ┘
	if (tail+uint32(len(d.vals)))&(1<<dequeueBits-1) == head {
		// Queue is full.
		return false
	}
	slot := &d.vals[head&uint32(len(d.vals)-1)]

	// Check if the head slot has been released by popTail.
	// 检查头部是否已被popTail释放。
	typ := atomic.LoadPointer(&slot.typ)
	if typ != nil {
		// Another goroutine is still cleaning up the tail, so
		// the queue is actually still full.
		// 另一个goroutine仍在清理tail，因此队列实际上仍然满了。
		return false
	}

	// The head slot is free, so we own it.
	// 头部是空闲的，因此我们拥有它，可以插入数据了。
	if val == nil {
		val = dequeueNil(nil)
	}
	*(*interface{})(unsafe.Pointer(slot)) = val

	// Increment head. This passes ownership of slot to popTail
	// and acts as a store barrier for writing the slot.
	// 增加head。 这将槽位的所有权传递给popTail并充当写入槽位的存储屏障。
	atomic.AddUint64(&d.headTail, 1<<dequeueBits) // 在head+1，head在高32位，所以要1<<dequeueBits
	return true
}

// popHead removes and returns the element at the head of the queue.
// It returns false if the queue is empty. It must only be called by a
// single producer.
// popHead删除并返回队列开头的元素。 如果队列为空，则返回false。 它只能由单个生产者调用。
func (d *poolDequeue) popHead() (interface{}, bool) {
	var slot *eface
	for {
		ptrs := atomic.LoadUint64(&d.headTail)
		head, tail := d.unpack(ptrs)
		if tail == head { // 头尾相等表示队列为空
			// Queue is empty.
			return nil, false
		}

		// Confirm tail and decrement head. We do this before
		// reading the value to take back ownership of this
		// slot.
		// 确认队尾和递减头部。 我们在读取值之前取回该槽位的所有权。
		head--
		ptrs2 := d.pack(head, tail)
		if atomic.CompareAndSwapUint64(&d.headTail, ptrs, ptrs2) {
			// We successfully took back slot.
			// 我们成功后退了槽位。
			slot = &d.vals[head&uint32(len(d.vals)-1)]
			break
		}
	}

	val := *(*interface{})(unsafe.Pointer(slot))
	if val == dequeueNil(nil) {
		val = nil
	}
	// Zero the slot. Unlike popTail, this isn't racing with
	// pushHead, so we don't need to be careful here.
	// 将槽位清零。 与popTail不同，这不是与pushHead竞争的，所以我们在这里不需要小心。
	*slot = eface{}
	return val, true
}

// popTail removes and returns the element at the tail of the queue.
// It returns false if the queue is empty. It may be called by any
// number of consumers.
// popTail删除并返回队列尾部的元素。 如果队列为空，则返回false。 任何数量的消费者都可以调用它。
func (d *poolDequeue) popTail() (interface{}, bool) {
	var slot *eface
	for {
		ptrs := atomic.LoadUint64(&d.headTail)
		head, tail := d.unpack(ptrs)
		if tail == head {
			// Queue is empty.
			return nil, false
		}

		// Confirm head and tail (for our speculative check
		// above) and increment tail. If this succeeds, then
		// we own the slot at tail.
		// 确认队头和队尾（用于上面的推测性检查）并增加尾位置。 如果成功，则我们在尾部拥有插槽。
		ptrs2 := d.pack(head, tail+1)
		if atomic.CompareAndSwapUint64(&d.headTail, ptrs, ptrs2) {
			// Success.
			slot = &d.vals[tail&uint32(len(d.vals)-1)]
			break
		}
	}

	// We now own slot. // 现在，我们拥有槽位。
	val := *(*interface{})(unsafe.Pointer(slot))
	if val == dequeueNil(nil) {
		val = nil
	}

	// Tell pushHead that we're done with this slot. Zeroing the
	// slot is also important so we don't leave behind references
	// that could keep this object live longer than necessary.
	//
	// We write to val first and then publish that we're done with
	// this slot by atomically writing to typ.
	//
	// 告诉pushHead我们已经完成了此槽位。 将槽位清零也很重要，因此我们不会留下会使该对象寿命超出必要时间的引用。
    //
    // 我们先写入val，然后通过原子写入typ来发布已完成此插槽的操作。
	slot.val = nil
	atomic.StorePointer(&slot.typ, nil)
	// At this point pushHead owns the slot. // 此时pushHead拥有此槽。

	return val, true
}

// poolChain is a dynamically-sized version of poolDequeue.
//
// This is implemented as a doubly-linked list queue of poolDequeues
// where each dequeue is double the size of the previous one. Once a
// dequeue fills up, this allocates a new one and only ever pushes to
// the latest dequeue. Pops happen from the other end of the list and
// once a dequeue is exhausted, it gets removed from the list.
//
// poolChain是poolDequeue的动态大小版本。
//
// 这是作为poolDequeues的双向链列表队列实现的，其中每个队列是上一个队列的大小的两倍。
// 一旦双向队列填满，就会分配一个新的双向队列，并且只会推送到最新的双向队列。
// 弹出声从双向附表的另一端发生，一旦双向队列耗尽，它就会从列表中删除。
type poolChain struct {
	// head is the poolDequeue to push to. This is only accessed
	// by the producer, so doesn't need to be synchronized.
	// head是poolDequeue要推送的。 这仅由生产者访问，因此不需要同步。
	head *poolChainElt

	// tail is the poolDequeue to popTail from. This is accessed
	// by consumers, so reads and writes must be atomic.
	// tail是poolDequeue使用popTail弹出的。 消费者可访问，因此读写必须是原子的。
	tail *poolChainElt
}

type poolChainElt struct {
	poolDequeue

	// next and prev link to the adjacent poolChainElts in this
	// poolChain.
	//
	// next is written atomically by the producer and read
	// atomically by the consumer. It only transitions from nil to
	// non-nil.
	//
	// prev is written atomically by the consumer and read
	// atomically by the producer. It only transitions from
	// non-nil to nil.
	//
	// next和prev链接到此poolChain中的相邻poolChainElts。
    //
    // next由生产者自动写入，由消费者自动读取。 它仅从nil过渡到non-nil。
    //
    // prev由消费者自动写入，由生产者自动编写。 它仅从非零过渡到零。
	next, prev *poolChainElt
}

// 存储PoolChainElt元素
func storePoolChainElt(pp **poolChainElt, v *poolChainElt) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(pp)), unsafe.Pointer(v))
}

// 取PoolChainElt元素值
func loadPoolChainElt(pp **poolChainElt) *poolChainElt {
	return (*poolChainElt)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(pp))))
}

// 向队头添加元素
func (c *poolChain) pushHead(val interface{}) {
	d := c.head
	if d == nil {
		// Initialize the chain. //初始化链表。
		const initSize = 8 // Must be a power of 2，必须是2的指数次方
		d = new(poolChainElt)
		d.vals = make([]eface, initSize)
		c.head = d
		storePoolChainElt(&c.tail, d)
	}

	if d.pushHead(val) { // 插入元素
		return
	}

	// The current dequeue is full. Allocate a new one of twice
	// the size.
	// 当前出队已满。 分配两倍大小的新链表。
	newSize := len(d.vals) * 2
	if newSize >= dequeueLimit { // dequeueLimit = 0B01000000_0000000_0000000_0000000_0000000
		// Can't make it any bigger. // 无法将其放大。
		newSize = dequeueLimit
	}

	d2 := &poolChainElt{prev: d}
	d2.vals = make([]eface, newSize)
	c.head = d2
	storePoolChainElt(&d.next, d2)
	d2.pushHead(val) // 使用d2存储元素
}

// 从头部取元素
func (c *poolChain) popHead() (interface{}, bool) {
	d := c.head
	for d != nil {
		if val, ok := d.popHead(); ok { // 从当前头部中取元素
			return val, ok
		}
		// There may still be unconsumed elements in the
		// previous dequeue, so try backing up.
		// 上一个双向队列中可能仍然有未消耗的元素，因此请尝试备份。
		d = loadPoolChainElt(&d.prev)
	}
	return nil, false
}

// 从尾部取元素
func (c *poolChain) popTail() (interface{}, bool) {
	d := loadPoolChainElt(&c.tail)
	if d == nil {
		return nil, false
	}

	for {
		// It's important that we load the next pointer
		// *before* popping the tail. In general, d may be
		// transiently empty, but if next is non-nil before
		// the pop and the pop fails, then d is permanently
		// empty, which is the only condition under which it's
		// safe to drop d from the chain.
		//
		// 我们在弹出尾部之前加载下一个指针非常重要。 通常，d可能会短暂地为空，但如果pop且pop失败之前next不为nil，则d永久为空，这是唯一可以安全地从链中删除d的条件。
		d2 := loadPoolChainElt(&d.next)

		if val, ok := d.popTail(); ok {
			return val, ok
		}

		if d2 == nil {
			// This is the only dequeue. It's empty right
			// now, but could be pushed to in the future.
			// 这是唯一的双向队列。 它现在是空的，但将来可能会被推入元素。
			return nil, false
		}

		// The tail of the chain has been drained, so move on
		// to the next dequeue. Try to drop it from the chain
		// so the next pop doesn't have to look at the empty
		// dequeue again.
		// 链的尾部已排空，因此继续进行下一个出队。 尝试将其从链中删除，以便下一个弹出窗口不必再次查看空的出队。
		if atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&c.tail)), unsafe.Pointer(d), unsafe.Pointer(d2)) {
			// We won the race. Clear the prev pointer so
			// the garbage collector can collect the empty
			// dequeue and so popHead doesn't back up
			// further than necessary.
			// 我们赢得了竞争。 清除prev指针，以便垃圾收集器可以收集空的出队，因此popHead不会备份超出必要的范围。
			storePoolChainElt(&d2.prev, nil)
		}
		d = d2
	}
}
```