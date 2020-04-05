chan.go是go语言通道实现，通道结构的定义，接收和发送的操作都此文件中实现。
# 通道的结构
hchan是通道表示的基本结构，其内容表示如下：
一些特殊情况
- 当dataqsiz=0时：说明这是一个无缓冲对列
- 当dataqsiz>0时，说明是一个缓冲对列
```go
type hchan struct {
    qcount   uint               // 队列中的数据总数
    dataqsiz uint               // 循环队列的大小
    buf      unsafe.Pointer     // 指向dataqsiz数组中的一个元素
    elemsize uint16
    closed   uint32             // 非0表示通道已经关闭
    elemtype *_type             // 元素类型
    sendx    uint               // 发送索引
    recvx    uint               // 接收索引
    recvq    waitq              // 所有等待的接收者
    sendq    waitq              // 所有等待的发送者

    // 锁保护hchan中的所有字段，以及此通道上阻止的sudogs中的多个字段。
    // 保持该锁状态时，请勿更改另一个G的状态（特别是不要准备好G），因为这会因堆栈收缩而死锁。
    // Question: sudogs？参见runtime/runtime2.go中的sudog结构
    lock mutex
}
```
# 通道的发送和接收
通道的发送和接收都是使用了一个名为waitq的等待对例，每个接收者和发送者都是一个sudog结构，此构在runtiem/runtime2.go文件中定义，在之后的源码分析中会仔细说明。
```go
type waitq struct {
	first *sudog    // 队列头
	last  *sudog    // 队列尾
}
```

# 通道的对齐方法式
从文件的源码中可以知道，在内存中通道是以8字节的方式进行对齐的，内存分配不是8个字节会自动对对齐
```go
const (
	maxAlign  = 8 // 8字节对齐
	hchanSize = unsafe.Sizeof(hchan{}) + uintptr(-int(unsafe.Sizeof(hchan{}))&(maxAlign-1))
	debugChan = false
)
```

# 通道的使用
## 通道的创建的方法
在源码中通道的创建有三个实现方法
- func reflect_makechan(t *chantype, size int) *hchan {...}
- func makechan64(t *chantype, size int64) *hchan {...}
- func makechan(t *chantype, size int) *hchan {...}
其本质是调用最后一个通道的创建方法。
## 通道的创建过程
方法`func makechan(t *chantype, size int) *hchan {...}`说明了通道的整个创建过程。
```go
/**
 * 创建通道
 * @param t 通道类型指针
 * @param size 通道大小，0表示无缓冲通道
 * @return
 **/
func makechan(t *chantype, size int) *hchan {...}
```
通道创建要通过以过几个步骤。
- 1、检查通道元素类型的size，如果`elem.size >= 1<<16`，则创建失败。
- 2、检查通道和元素的对齐，如果`hchanSize%maxAlign != 0 || elem.align > maxAlign`，则通道创建失败
- 3、计算通道需要分配的内存，如果`overflow || mem > maxAlloc-hchanSize || size < 0`，则通道创建失败
    - 3.1、overflow：表示内存内存计算有溢出
    - 3.2、mem > maxAlloc-hchanSize：表示内存分配置超出了限制
    - 3.3、size < 0：表示通道缓冲为负数
- 4、根据不同条件进行内存分配
    - 4.1、mem == 0：队列或者元素大小为0，这是表示元素或者队列大小为0，直接分配hchanSize大小的内存，缓冲地址向自身
    - 4.2、elem.ptrdata == 0：元素不包含指针，直接分配hchanSize+mem大小的内存，并且缓冲地址向自身+hchanSize
    - 4.3、其他情况（元素包含指针）：使用new的方法创建一个hchan，分配mem大小的内存，并且缓冲地址向内存分配的地址
- 5、最后设置通道结构的其他值

## 向通道中发送数据
向通道中发送数据的方法一共有四个，如下所示
- func chansend1(c *hchan, elem unsafe.Pointer) {...}
- func chansend(c *hchan, ep unsafe.Pointer, block bool, callerpc uintptr) bool {...}
- func send(c *hchan, sg *sudog, ep unsafe.Pointer, unlockf func(), skip int) {...}
- func sendDirect(t *_type, sg *sudog, src unsafe.Pointer) {...}
chansend1方法是go编译代码中c <- x的入口点，即当我们编写代码 c <- x时，就是调用此方法。chansend1方法本质还是调用chansend方法进行处理
这四个方法的调用如下图所示
```plantuml
digraph send_call {
    chansend1 -> chansend
    chansend -> send
    send -> sendDirect
}
```

chansend方法的执行代表了事个数据的发送过程，方法签名如下：
```go
/**
  * 通用单通道发送/接收
  * 如果block不为nil，则协议将不会休眠，但如果无法完成则返回。
  *
  * 当涉及休眠的通道已关闭时，可以使用g.param == nil唤醒休眠。
  * 最容易循环并重新运行该操作； 我们将看到它现已关闭。
  * @param c 通道对象
  * @param ep 元素指针
  * @param block 是否阻塞
  * @param callerpc 调用者指针
  * @return bool true：表示发送成功
  **/
func chansend(c *hchan, ep unsafe.Pointer, block bool, callerpc uintptr) bool {...}
```
通道数据发送的处理过程：
- 1、如果通道为空
    - 1.1、通道是非阻塞的，则返回false，表示发送失败，方法结束
    - 1.2、通道是阻塞的，则会阻塞当前goroutine，同时会执行一个throw方法，方法结束
- 2、检查通道和数据状态，2.1，2.2，2.3同时满足时发关失败，方法结束
    - 2.1、!block：通道非阻塞
    - 2.2、c.closed == 0：通道未关闭
    - 2.3、((c.dataqsiz == 0 && c.recvq.first == nil) || (c.dataqsiz > 0 && c.qcount == c.dataqsiz))
        - 2.3.1、(c.dataqsiz == 0 && c.recvq.first == nil)：通道中没有数据，并且没有接收者
        - 2.3.2、(c.dataqsiz > 0 && c.qcount == c.dataqsiz)：通道中有数据，并且已经满了
- 3、对通道进行锁定
- 4、判断通道是否已经关闭，如果已经关闭，就解锁通道，并且抛出panic。方法结束
- 5、在接收者队列中找出一个最先入队的接收者，如果有，就调用send方法进行发送，返回true，方法结束
- 6、如果没有找到接收者，并且c.qcount < c.dataqsiz，即通道的发送缓冲区未满，将要发送的数据拷贝到通道缓冲区，更新相关的计数据信息，并释放锁，返回true，方法结束
- 7、没有找到接收都，并且没有缓冲可用，非阻塞方式，则解锁通道，返回false，发送失败
- 8、没有找到接收都，并且没有缓冲可用，阻塞方式。获取gp(g)和mysg(sudog)，并且将发送数据挂到mysg上，将mysg加到发送队列。调用gopark访求将当前goroutine阻塞，直到被恢复。
- 9、在恢复后我们还要将发送数据保活，以确保数据正确被接收者复制出去了。
- 10、检查goroutine状态，如果mysg != gp.waiting说明被破坏了，执行throw，方法结束
- 11、如果gp.param == nil，说明唤醒有问题，
    - 11.1、如果通道未关闭，则说明是伪唤醒，执行throw方法结束
    - 11.2、如果通道关闭，则panic，在关闭的通道中进行了发送消息。
- 12、最后是清理数据，并且释放mysg




# 源码
```go
package runtime

// This file contains the implementation of Go channels.

// Invariants:
//  At least one of c.sendq and c.recvq is empty,
//  except for the case of an unbuffered channel with a single goroutine
//  blocked on it for both sending and receiving using a select statement,
//  in which case the length of c.sendq and c.recvq is limited only by the
//  size of the select statement.
//
// For buffered channels, also:
//  c.qcount > 0 implies that c.recvq is empty.
//  c.qcount < c.dataqsiz implies that c.sendq is empty.
/**
 * 此文件包含Go渠道的实现。此包中所使用的类型大部分定义在：runtime/type.go文件中
 *
 * 不变量：
 * c.sendq和c.recvq中的至少一个为空，但在无缓冲通道上阻塞了单个goroutine以便使用select语句发送和接收的情况除外，在这种情况下，
 * c.sendq的长度 而c.recvq仅受select语句的大小限制。
 *
 * 对于缓冲通道，还：
 * c.qcount> 0表示c.recvq为空。
 * c.qcount <c.dataqsiz表示c.sendq为空。
 */
import (
	"runtime/internal/atomic"
	"runtime/internal/math"
	"unsafe"
)

const (
	maxAlign  = 8 // 8字节对齐
	hchanSize = unsafe.Sizeof(hchan{}) + uintptr(-int(unsafe.Sizeof(hchan{}))&(maxAlign-1))
	debugChan = false
)

type hchan struct {
	qcount   uint           // total data in the queue // 队列中的数据总数
	dataqsiz uint           // size of the circular queue   // 循环队列的大小
	buf      unsafe.Pointer // points to an array of dataqsiz elements // 指向dataqsiz数组中的一个元素
	elemsize uint16
	closed   uint32  // 非0表示通道已经关闭
	elemtype *_type // element type // 元素类型
	sendx    uint   // send index   // 发送索引
	recvx    uint   // receive index // 接收索引
	recvq    waitq  // list of recv waiters // 所有等待的接收者
	sendq    waitq  // list of send waiters // 所有等待的发送者

	// lock protects all fields in hchan, as well as several
	// fields in sudogs blocked on this channel.
	//
	// Do not change another G's status while holding this lock
	// (in particular, do not ready a G), as this can deadlock
	// with stack shrinking.
	// 锁保护hchan中的所有字段，以及此通道上阻止的sudogs中的多个字段。
    // 保持该锁状态时，请勿更改另一个G的状态（特别是不要准备好G），因为这会因堆栈收缩而死锁。
    // Question: sudogs？参见runtime/runtime2.go中的sudog结构
	lock mutex
}

/**
 * 等待队列数据结棍
 */
type waitq struct {
	first *sudog    // 队列头
	last  *sudog    // 队列尾
}

//go:linkname reflect_makechan reflect.makechan
/**
 * 通过反射创建通道，
 * @param
 * @return
 **/
func reflect_makechan(t *chantype, size int) *hchan {
	return makechan(t, size) // 在reflect/value.go/makechan方法中实现
}

/**
 * 创建通道
 * @param t 通道类型
 * @param size 看上去是支持64位int，本质上只支持int类型
 * @return
 **/
func makechan64(t *chantype, size int64) *hchan {
	if int64(int(size)) != size { // 说明有溢出
		panic(plainError("makechan: size out of range"))
	}

	return makechan(t, int(size))
}

/**
 * 创建通道
 * @param t 通道类型指针
 * @param size 通道大小，0表示无缓冲通道
 * @return
 **/
func makechan(t *chantype, size int) *hchan {
	elem := t.elem

	// compiler checks this but be safe.
	// 编译器对此进行检查，但很安全。
	if elem.size >= 1<<16 {
		throw("makechan: invalid channel element type")
	}
	// 非8字节对齐
	if hchanSize%maxAlign != 0 || elem.align > maxAlign {
		throw("makechan: bad alignment")
	}

    // MulUintptr返回a * b以及乘法是否溢出。 在受支持的平台上，这是编译器固有的功能。
	mem, overflow := math.MulUintptr(elem.size, uintptr(size))
	// 发生溢出，或者分配内存超限制，或者size<0
	if overflow || mem > maxAlloc-hchanSize || size < 0 {
		panic(plainError("makechan: size out of range"))
	}

	// Hchan does not contain pointers interesting for GC when elements stored in buf do not contain pointers.
	// buf points into the same allocation, elemtype is persistent.
	// SudoG's are referenced from their owning thread so they can't be collected.
	// TODO(dvyukov,rlh): Rethink when collector can move allocated objects.
	// 当存储在buf中的元素不包含指针时，Hchan不包含GC感兴趣的指针。 buf指向相同的分配，
	// elemtype是持久的。 SudoG是从它们自己的线程中引用的，因此无法收集它们。
    // TODO（dvyukov，rlh）：重新考虑何时收集器可以移动分配的对象。
	var c *hchan
	switch {
	case mem == 0: // 不需要分配置内存空间
		// Queue or element size is zero.
	    // 队列或元素大小为零。
	    // mallocgc分配一个大小为size字节的对象。 小对象是从每个P缓存的空闲列表中分配的。
	    // 大对象（> 32 kB）直接从堆中分配。
		c = (*hchan)(mallocgc(hchanSize, nil, true))
		// Race detector uses this location for synchronization.
		// 竞态探测器使用此位置进行同步。
		c.buf = c.raceaddr()
	case elem.ptrdata == 0: // 无指针数据
		// Elements do not contain pointers.
		// Allocate hchan and buf in one call.
		// 元素不包含指针。 在一次调用中分配hchan和buf。
		c = (*hchan)(mallocgc(hchanSize+mem, nil, true))
		c.buf = add(unsafe.Pointer(c), hchanSize) // 修改数据指针地址
	default:
		// Elements contain pointers.
	    // 元素包含指针。
		c = new(hchan)
		c.buf = mallocgc(mem, elem, true)
	}

	c.elemsize = uint16(elem.size) // 设置元素大小
	c.elemtype = elem   // 设置元素类型
	c.dataqsiz = uint(size) // 设置通道大小

	if debugChan {
		print("makechan: chan=", c, "; elemsize=", elem.size, "; dataqsiz=", size, "\n")
	}
	return c
}

// chanbuf(c, i) is pointer to the i'th slot in the buffer.
/**
 * chanbuf(c, i) 返回指向缓冲区中第i个槽值的指针。
 * @param c 通道对象
 * @return i 第i个槽位
 **/
func chanbuf(c *hchan, i uint) unsafe.Pointer {
	return add(c.buf, uintptr(i)*uintptr(c.elemsize))
}

// entry point for c <- x from compiled code
//go:nosplit
/**
 * 编译代码中c <- x的入口点，即当我们编写代码 c <- x时，就是调用此方法
 * @param c 通道对象
 * @param elem 需要发送的元素
 * @return
 **/
func chansend1(c *hchan, elem unsafe.Pointer) {
	chansend(c, elem, true, getcallerpc())
}

/*
 * generic single channel send/recv
 * If block is not nil,
 * then the protocol will not
 * sleep but return if it could
 * not complete.
 *
 * sleep can wake up with g.param == nil
 * when a channel involved in the sleep has
 * been closed.  it is easiest to loop and re-run
 * the operation; we'll see that it's now closed.
 */
 /**
  * 通用单通道发送/接收
  * 如果block不为nil，则协议将不会休眠，但如果无法完成则返回。
  *
  * 当涉及休眠的通道已关闭时，可以使用g.param == nil唤醒休眠。
  * 最容易循环并重新运行该操作； 我们将看到它现已关闭。
  * @param c 通道对象
  * @param ep 元素指针
  * @param block 是否阻塞
  * @param callerpc 调用者指针
  * @return bool true：表示发送成功
  **/
func chansend(c *hchan, ep unsafe.Pointer, block bool, callerpc uintptr) bool {
	if c == nil { // 通道已为空
		if !block { // 非阻塞
			return false
		}
		// 将当前goroutine置于等待状态并调用unlockf。 如果unlockf返回false，则继续执行goroutine程序。
		// unlockf一定不能访问此G的堆栈，因为它可能在调用gopark和调用unlockf之间移动。
        // waitReason参数说明了goroutine已停止的原因。
        // 它显示在堆栈跟踪和堆转储中。
        // waitReason应具有唯一性和描述性。
        // 不要重复使用waitReason，请添加新的waitReason。
        // 更详细的说明参见：runtime/proc.go和runtime/runtime2.go
		gopark(nil, nil, waitReasonChanSendNilChan, traceEvGoStop, 2)
		throw("unreachable")
	}

	if debugChan { // 当前已经为false
		print("chansend: chan=", c, "\n")
	}

	if raceenabled { // 当前已经为false
		racereadpc(c.raceaddr(), callerpc, funcPC(chansend))
	}

	// Fast path: check for failed non-blocking operation without acquiring the lock.
	//
	// After observing that the channel is not closed, we observe that the channel is
	// not ready for sending. Each of these observations is a single word-sized read
	// (first c.closed and second c.recvq.first or c.qcount depending on kind of channel).
	// Because a closed channel cannot transition from 'ready for sending' to
	// 'not ready for sending', even if the channel is closed between the two observations,
	// they imply a moment between the two when the channel was both not yet closed
	// and not ready for sending. We behave as if we observed the channel at that moment,
	// and report that the send cannot proceed.
	//
	// It is okay if the reads are reordered here: if we observe that the channel is not
	// ready for sending and then observe that it is not closed, that implies that the
	// channel wasn't closed during the first observation.
	// 快速路径：在没有获取锁的情况下检查失败的非阻塞操作。
    //
    // 观察到通道未关闭后，我们观察到该通道尚未准备好发送。 这些观察中的每一个都是单个字（word）大小的读取
    // （根据通道的类型，取第一个c.closed和第二个c.recvq.first或c.qcount）。
    // 因为关闭的通道无法从“准备发送”转换为“未准备发送”，所以即使通道在两个观测值之间处于关闭状态，
    // 它们也隐含着两者之间的一个时刻，即通道既未关闭又未关闭准备发送。 我们的行为就好像我们当时在观察该通道，
    // 并报告发送无法继续进行。
    //
    // 如果在此处对读取进行了重新排序，也是可以的：如果我们观察到该通道尚未准备好发送，然后观察到它没有关闭，
    // 则意味着该通道在第一次观察期间没有关闭。
    // !block：非阻塞状态
    // c.closed == 0：通道未关闭
    // (c.dataqsiz == 0 && c.recvq.first == nil)：通道中没有数据，并且没有接收者
    // (c.dataqsiz > 0 && c.qcount == c.dataqsiz)：通道中有数据，并且已经满了
	if !block && c.closed == 0 && ((c.dataqsiz == 0 && c.recvq.first == nil) ||
		(c.dataqsiz > 0 && c.qcount == c.dataqsiz)) {
		return false
	}

	var t0 int64
	if blockprofilerate > 0 {
		t0 = cputicks()
	}

    // 加锁
	lock(&c.lock)

	if c.closed != 0 { // 通道已经关闭，解锁，抛出panic
		unlock(&c.lock)
		panic(plainError("send on closed channel"))
	}

	if sg := c.recvq.dequeue(); sg != nil {
		// Found a waiting receiver. We pass the value we want to send
		// directly to the receiver, bypassing the channel buffer (if any).
		// 找到了等待的接收者。 我们绕过通道缓冲区（如果有）将要发送的值直接发送给接收器。
		send(c, sg, ep, func() { unlock(&c.lock) }, 3)
		return true
	}

    // 没有找到接收者，并且队列元素未填满通道的循环队列
	if c.qcount < c.dataqsiz {
		// Space is available in the channel buffer. Enqueue the element to send.
		// 通道缓冲区中有可用空间。 使要发送的元素入队。
		qp := chanbuf(c, c.sendx)
		if raceenabled { // 此值已经为false
			raceacquire(qp)
			racerelease(qp)
		}
		// 执行内存移动
		typedmemmove(c.elemtype, qp, ep)
		c.sendx++ // 指向下一个可以发送数据的空位
		if c.sendx == c.dataqsiz { // 已经达到了末尾
			c.sendx = 0 // 重新指向头部
		}
		c.qcount++ // 通道总的数据加1
		unlock(&c.lock) // 解锁
		return true // 说明发送成功
	}

    // 非阻塞，因为找不到接收者，所以失败
	if !block {
		unlock(&c.lock)
		return false
	}

	// Block on the channel. Some receiver will complete our operation for us.
    // 在通道上阻塞。一些接收器将为我们完成操作。
    // getg将返回指向当前g的指针。获取suodg
	gp := getg()
	mysg := acquireSudog()
	mysg.releasetime = 0
	if t0 != 0 {
		mysg.releasetime = -1
	}
	// No stack splits between assigning elem and enqueuing mysg
	// on gp.waiting where copystack can find it.
	// 在分配elem和将mysg入队到gp.waitcopy可以找到它的地方之间没有堆栈拆分。
	mysg.elem = ep
	mysg.waitlink = nil
	mysg.g = gp
	mysg.isSelect = false
	mysg.c = c
	gp.waiting = mysg
	gp.param = nil
	c.sendq.enqueue(mysg)
	gopark(chanparkcommit, unsafe.Pointer(&c.lock), waitReasonChanSend, traceEvGoBlockSend, 2)
	// Ensure the value being sent is kept alive until the
	// receiver copies it out. The sudog has a pointer to the
	// stack object, but sudogs aren't considered as roots of the
	// stack tracer.
	// 确保发送的值保持活动状态，直到接收者将其复制出来。
	// sudog具有指向堆栈对象的指针，但是sudog不被视为堆栈跟踪器的根。
	KeepAlive(ep)

	// someone woke us up.
	// 有人把我们唤醒了
	if mysg != gp.waiting {
		throw("G waiting list is corrupted")
	}
	gp.waiting = nil
	gp.activeStackChans = false
	if gp.param == nil {
		if c.closed == 0 {
			throw("chansend: spurious wakeup")
		}
		panic(plainError("send on closed channel"))
	}
	gp.param = nil
	if mysg.releasetime > 0 {
		blockevent(mysg.releasetime-t0, 2)
	}
	mysg.c = nil
	releaseSudog(mysg) // 释放sudog
	return true
}

// send processes a send operation on an empty channel c.
// The value ep sent by the sender is copied to the receiver sg.
// The receiver is then woken up to go on its merry way.
// Channel c must be empty and locked.  send unlocks c with unlockf.
// sg must already be dequeued from c.
// ep must be non-nil and point to the heap or the caller's stack.
/**
 * send在空通道c上执行发送操作。
 * 将发送方发送的ep值复制到接收方sg。
 * 然后将接收器唤醒，继续前进。
 * 频道c必须为空且已锁定。 使用unlockf发送解锁通道c。
 * sg必须已经从c中出队。
 * ep必须为非nil，并指向堆或调用者的堆栈。
 * @param c 通道对象
 * @param sg
 * @param ep 元素指针
 * @param unlockf 角锁方法
 * @param skip
 * @return
 **/
func send(c *hchan, sg *sudog, ep unsafe.Pointer, unlockf func(), skip int) {
	if raceenabled { // 此值已经为false
		if c.dataqsiz == 0 {
			racesync(c, sg)
		} else {
			// Pretend we go through the buffer, even though
			// we copy directly. Note that we need to increment
			// the head/tail locations only when raceenabled.
			qp := chanbuf(c, c.recvx)
			raceacquire(qp)
			racerelease(qp)
			raceacquireg(sg.g, qp)
			racereleaseg(sg.g, qp)
			c.recvx++
			if c.recvx == c.dataqsiz {
				c.recvx = 0
			}
			c.sendx = c.recvx // c.sendx = (c.sendx+1) % c.dataqsiz
		}
	}
	if sg.elem != nil { // 元素不为空，直接发送
		sendDirect(c.elemtype, sg, ep)
		sg.elem = nil
	}
	gp := sg.g
	unlockf()
	gp.param = unsafe.Pointer(sg)
	if sg.releasetime != 0 {
		sg.releasetime = cputicks()
	}
	goready(gp, skip+1) // Question: 这里是用来做什么？
}

// Sends and receives on unbuffered or empty-buffered channels are the
// only operations where one running goroutine writes to the stack of
// another running goroutine. The GC assumes that stack writes only
// happen when the goroutine is running and are only done by that
// goroutine. Using a write barrier is sufficient to make up for
// violating that assumption, but the write barrier has to work.
// typedmemmove will call bulkBarrierPreWrite, but the target bytes
// are not in the heap, so that will not help. We arrange to call
// memmove and typeBitsBulkBarrier instead.
/**
 * 在一个无缓冲通道或空缓冲通道上发送和接收是一个正在运行的goroutine写入另一个正在运行的goroutine堆栈的唯一操作。
 * GC假定仅在goroutine运行时才发生堆栈写入，并且仅由该goroutine完成。 使用写屏障足以弥补违反该假设的缺点，
 * 但是写屏障必须起作用。 typedmemmove将调用bulkBarrierPreWrite，但是目标字节不在堆中，因此这无济于事。
 * 我们安排调用memmove和typeBitsBulkBarrier。
 * @param t 元素类型
 * @param sg
 * @param src 数据指针
 * @return
 **/
func sendDirect(t *_type, sg *sudog, src unsafe.Pointer) {
	// src is on our stack, dst is a slot on another stack.
	// src在我们的栈上，dst是另一个栈上的位置。

	// Once we read sg.elem out of sg, it will no longer
	// be updated if the destination's stack gets copied (shrunk).
	// So make sure that no preemption points can happen between read & use.
	// 一旦我们从sg中读取出sg.elem，如果目标堆栈被复制（缩小），它将不再被更新。
	// 因此，请确保在读取和使用之间没有任何抢占点。
	dst := sg.elem
	// 带屏障的写操作，参见：runtime/mbitmap.go/typeBitsBulkBarrier方法
	typeBitsBulkBarrier(t, uintptr(dst), uintptr(src), t.size)
	// No need for cgo write barrier checks because dst is always
	// Go memory.
	// 不需要cgo写屏障检查，因为dst始终是Go内存。
	memmove(dst, src, t.size)
}

/**
 * 直接接收通道数据
 * @param
 * @return
 **/
func recvDirect(t *_type, sg *sudog, dst unsafe.Pointer) {
	// dst is on our stack or the heap, src is on another stack.
	// The channel is locked, so src will not move during this
	// operation.
	// dst在我们的栈或堆上，src在另一个栈上。 通道已锁定，因此src在此操作期间将不会移动。
	src := sg.elem
	typeBitsBulkBarrier(t, uintptr(dst), uintptr(src), t.size)
	memmove(dst, src, t.size)
}
/**
 * 关闭通道
 * @param
 * @return
 **/
func closechan(c *hchan) {
	if c == nil { // 通道为nil
		panic(plainError("close of nil channel"))
	}

	lock(&c.lock) // 锁定
	if c.closed != 0 { // 通道已经关闭，再关闭就报panic
		unlock(&c.lock) // 解锁
		panic(plainError("close of closed channel"))
	}

	if raceenabled { // 此值已经为false
		callerpc := getcallerpc()
		racewritepc(c.raceaddr(), callerpc, funcPC(closechan))
		racerelease(c.raceaddr())
	}

	c.closed = 1 // 标记通道已经关闭

	var glist gList

	// release all readers // 释放所有的接收者
	for {
		sg := c.recvq.dequeue() // 接收者队列
		if sg == nil { // 队列为空
			break
		}
		if sg.elem != nil {
			typedmemclr(c.elemtype, sg.elem) // 进行内存清理
			sg.elem = nil
		}
		if sg.releasetime != 0 {
			sg.releasetime = cputicks()
		}
		gp := sg.g
		gp.param = nil // 参数清零
		if raceenabled {
			raceacquireg(gp, c.raceaddr())
		}
		glist.push(gp) // gp入队头
	}

	// release all writers (they will panic)
	// 释放所有写对象（他们会恐慌）
	for {
		sg := c.sendq.dequeue() // 发送者队列
		if sg == nil { // 队列为空
			break
		}
		sg.elem = nil // 直接清空内存？这么做不会内存溢出？
		if sg.releasetime != 0 {
			sg.releasetime = cputicks()
		}
		gp := sg.g
		gp.param = nil // 参数清零
		if raceenabled {
			raceacquireg(gp, c.raceaddr())
		}
		glist.push(gp) // gp入队头
	}
	unlock(&c.lock)

	// Ready all Gs now that we've dropped the channel lock.
	// 现在我们已经释放了通道锁，准备好所有G。
	for !glist.empty() { // 对所有的g，将schedlink清0，并且设置已经已经准备好
		gp := glist.pop()
		gp.schedlink = 0
		goready(gp, 3)
	}
}

// entry points for <- c from compiled code
//go:nosplit
/**
 * 编译代码中 <-c 的入口点
 * @param
 * @return
 **/
func chanrecv1(c *hchan, elem unsafe.Pointer) {
	chanrecv(c, elem, true)
}

//go:nosplit
/**
 *
 * @param
 * @return received true:表示已经接收到
 **/
func chanrecv2(c *hchan, elem unsafe.Pointer) (received bool) {
	_, received = chanrecv(c, elem, true)
	return
}

// chanrecv receives on channel c and writes the received data to ep.
// ep may be nil, in which case received data is ignored.
// If block == false and no elements are available, returns (false, false).
// Otherwise, if c is closed, zeros *ep and returns (true, false).
// Otherwise, fills in *ep with an element and returns (true, true).
// A non-nil ep must point to the heap or the caller's stack.
/**
 * chanrecv在通道c上接收并将接收到的数据写入ep。
 * ep可能为nil，在这种情况下，接收到的数据将被忽略。
 * 如果block == false并且没有可用元素，则返回（false，false）。
 * 否则，如果c关闭，则* ep为零并返回（true，false）。
 * 否则，用一个元素填充* ep并返回（true，true）。
 * 非nil必须指向堆或调用者的堆栈。
 * @param c 通道对象
 * @param ep 用于接收数据的指针
 * @param block true: 表示阻塞
 * @return selected true表示被选择
 * @return received true表示已经接收到值
 **/
func chanrecv(c *hchan, ep unsafe.Pointer, block bool) (selected, received bool) {
	// raceenabled: don't need to check ep, as it is always on the stack
	// or is new memory allocated by reflect.
	// raceenabled：不需要检查ep，因为它始终在堆栈中，或者是反射所分配的新内存。

	if debugChan { // 此值已经为false
		print("chanrecv: chan=", c, "\n")
	}

	if c == nil { // 通道为空
		if !block { // 非阻塞
			return
		}
		// 阻塞状态
        // 将当前goroutine置于等待状态并调用unlockf。
		gopark(nil, nil, waitReasonChanReceiveNilChan, traceEvGoStop, 2)
		throw("unreachable")
	}

	// Fast path: check for failed non-blocking operation without acquiring the lock.
	//
	// After observing that the channel is not ready for receiving, we observe that the
	// channel is not closed. Each of these observations is a single word-sized read
	// (first c.sendq.first or c.qcount, and second c.closed).
	// Because a channel cannot be reopened, the later observation of the channel
	// being not closed implies that it was also not closed at the moment of the
	// first observation. We behave as if we observed the channel at that moment
	// and report that the receive cannot proceed.
	//
	// The order of operations is important here: reversing the operations can lead to
	// incorrect behavior when racing with a close.
	// 快速路径：不获取锁定而检查失败的非阻塞操作。
    //
    // 在观察到通道尚未准备好接收之后，我们观察到通道未关闭。 这些观察中的每一个都是单个字（word）大小的读取
    // （第一个c.sendq.first或c.qcount，第二个c.closed）。
    // 由于无法重新打开通道，因此对通道未关闭的后续观察意味着它在第一次观察时也未关闭。
    // 我们的行为就好像我们当时在观察该通道，并报告接收无法继续进行。
    //
    // 操作顺序在这里很重要：在进行抢占关闭时，反转操作可能导致错误的行为。
    // !block : 非阻塞
    // c.dataqsiz == 0 && c.sendq.first == nil : 通道没有空间，并且没有发送者
    // c.dataqsiz > 0 && atomic.Loaduint(&c.qcount) == 0 :  通道中没有空间，并且没有数据
    // atomic.Load(&c.closed) : 通道未关闭
	if !block && (c.dataqsiz == 0 && c.sendq.first == nil ||
		c.dataqsiz > 0 && atomic.Loaduint(&c.qcount) == 0) &&
		atomic.Load(&c.closed) == 0 {
		return
	}

	var t0 int64
	if blockprofilerate > 0 {
		t0 = cputicks()
	}

	lock(&c.lock) // 加锁

	if c.closed != 0 && c.qcount == 0 { // 通道已经关闭，并且通道中没有数据
		if raceenabled {
			raceacquire(c.raceaddr())
		}
		unlock(&c.lock) // 解锁
		if ep != nil {
			typedmemclr(c.elemtype, ep) // 进行内存清理
		}
		return true, false
	}

	if sg := c.sendq.dequeue(); sg != nil { // 发送者队例不为空
		// Found a waiting sender. If buffer is size 0, receive value
		// directly from sender. Otherwise, receive from head of queue
		// and add sender's value to the tail of the queue (both map to
		// the same buffer slot because the queue is full).
		// 找到了等待发送者。 如果缓冲区的大小为0，则直接从发送方接收值。
		// 否则，从队列的开头接收并将发件人的值添加到队列的末尾（由于队列已满，因此两者都映射到同一缓冲区）。
		recv(c, sg, ep, func() { unlock(&c.lock) }, 3)
		return true, true
	}

    // 没有发送者，数据队列不为空
	if c.qcount > 0 {
		// Receive directly from queue
		// 直接从数据队列中接收数据
		qp := chanbuf(c, c.recvx)
		if raceenabled {
			raceacquire(qp)
			racerelease(qp)
		}
		if ep != nil { // 进行内存数据移动
			typedmemmove(c.elemtype, ep, qp)
		}
		typedmemclr(c.elemtype, qp) // 清理qp的c.elemtype类型内存数据
		c.recvx++ // 指向下一个接收位置
		if c.recvx == c.dataqsiz { // 说明已经指向了队列末尾了的下一个位置了
			c.recvx = 0 // 重新指向头部
		}
		c.qcount-- // 数据队列中的数据减少一个
		unlock(&c.lock) // 解锁
		return true, true
	}

    // 没有发送者，并且队列为空，并且是非阻塞状态
	if !block {
		unlock(&c.lock)
		return false, false
	}

	// no sender available: block on this channel.
	// 没有可用的发送者：在此通道阻塞。
	gp := getg()
	mysg := acquireSudog()
	mysg.releasetime = 0
	if t0 != 0 {
		mysg.releasetime = -1
	}
	// No stack splits between assigning elem and enqueuing mysg
	// on gp.waiting where copystack can find it.
	// 在分配elem和将mysg入队到gp.waitcopy可以找到它的地方之间没有堆栈拆分。
	mysg.elem = ep
	mysg.waitlink = nil
	gp.waiting = mysg
	mysg.g = gp
	mysg.isSelect = false
	mysg.c = c
	gp.param = nil
	c.recvq.enqueue(mysg)
	// 阻塞状态
    // 将当前goroutine置于等待状态并调用unlockf。
	gopark(chanparkcommit, unsafe.Pointer(&c.lock), waitReasonChanReceive, traceEvGoBlockRecv, 2)

	// someone woke us up
	// 有人把我们唤醒了
	if mysg != gp.waiting {
		throw("G waiting list is corrupted")
	}
	gp.waiting = nil
	gp.activeStackChans = false
	if mysg.releasetime > 0 {
		blockevent(mysg.releasetime-t0, 2)
	}
	closed := gp.param == nil
	gp.param = nil
	mysg.c = nil
	releaseSudog(mysg) // 释放sudog
	return true, !closed
}

// recv processes a receive operation on a full channel c.
// There are 2 parts:
// 1) The value sent by the sender sg is put into the channel
//    and the sender is woken up to go on its merry way.
// 2) The value received by the receiver (the current G) is
//    written to ep.
// For synchronous channels, both values are the same.
// For asynchronous channels, the receiver gets its data from
// the channel buffer and the sender's data is put in the
// channel buffer.
// Channel c must be full and locked. recv unlocks c with unlockf.
// sg must already be dequeued from c.
// A non-nil ep must point to the heap or the caller's stack.
/**
 * recv在完整通道c上处理接收操作。
 * 有2个部分：
 *      1）将发送方sg发送的值放入通道中，并唤醒发送方以继续进行。
 *      2）接收方接收到的值（当前G）被写入ep。
 * 对于同步通道，两个值相同。
 * 对于异步通道，接收者从通道缓冲区获取数据，而发送者的数据放入通道缓冲区。
 * 频道c必须已满且已锁定。 recv用unlockf解锁c。
 * sg必须已经从c中出队。
 * 非nil必须指向堆或调用者的堆栈。
 * @param c 通道对象针对
 * @param sg
 * @param ep 用户接收元素的指针
 * @param unlockf 解锁函数
 * @param skip
 * @return
 **/
func recv(c *hchan, sg *sudog, ep unsafe.Pointer, unlockf func(), skip int) {
	if c.dataqsiz == 0 { // 数据队列的大小是0，表示这上一个无缓冲的通道
		if raceenabled {
			racesync(c, sg)
		}
		if ep != nil { // 元素指针不为空
			// copy data from sender
			// 直接从发送发拷贝数据
			recvDirect(c.elemtype, sg, ep)
		}
	} else { // 有缓冲通道
		// Queue is full. Take the item at the
		// head of the queue. Make the sender enqueue
		// its item at the tail of the queue. Since the
		// queue is full, those are both the same slot.
		// 队列已满。 将item放在队列的开头。 使发送者将其item排入队列的末尾。
		// 由于队列已满，因此它们都是相同的槽位。
		qp := chanbuf(c, c.recvx)
		if raceenabled { // 此值已经为false
			raceacquire(qp)
			racerelease(qp)
			raceacquireg(sg.g, qp)
			racereleaseg(sg.g, qp)
		}
		// copy data from queue to receiver
		if ep != nil {
			typedmemmove(c.elemtype, ep, qp)
		}
		// copy data from sender to queue
		// 将数据从队列复制到接收者
		typedmemmove(c.elemtype, qp, sg.elem)
		c.recvx++ // 指向下一个接收位置
		if c.recvx == c.dataqsiz { // 已经达到了末尾的下一个位置，需要重新指向头部
			c.recvx = 0
		}
		c.sendx = c.recvx // c.sendx = (c.sendx+1) % c.dataqsiz // 发送者位置前移
	}
	sg.elem = nil
	gp := sg.g
	unlockf() // 解锁
	gp.param = unsafe.Pointer(sg)
	if sg.releasetime != 0 {
		sg.releasetime = cputicks()
	}
	// 将当前goroutine置于等待状态并解锁锁。 可以通过调用goready（gp）使goroutine重新运行
	goready(gp, skip+1) // 标记go
}

/**
 * 
 * @param 
 * @return 
 **/
func chanparkcommit(gp *g, chanLock unsafe.Pointer) bool {
	// There are unlocked sudogs that point into gp's stack. Stack
	// copying must lock the channels of those sudogs.
	// 有未锁定的sudog指向gp的堆栈。堆栈复制必须锁定那些sudog的通道。
	// activeStackChans指示存在指向该goroutine堆栈的未锁定通道。 
	// 如果为true，则堆栈复制需要获取通道锁以保护堆栈的这些区域。
	gp.activeStackChans = true
	unlock((*mutex)(chanLock)) // 解锁
	return true
}

// compiler implements
//
//	select {
//	case c <- v:
//		... foo
//	default:
//		... bar
//	}
//
// as
//
//	if selectnbsend(c, v) {
//		... foo
//	} else {
//		... bar
//	}
//
/**
 * 编译器实现，将goroutine的select send(case c <- v)语句转成对应的方法执行
 * @param
 * @return
 **/
func selectnbsend(c *hchan, elem unsafe.Pointer) (selected bool) {
	return chansend(c, elem, false, getcallerpc())
}

// compiler implements
//
//	select {
//	case v = <-c:
//		... foo
//	default:
//		... bar
//	}
//
// as
//
//	if selectnbrecv(&v, c) {
//		... foo
//	} else {
//		... bar
//	}
//
/**
 * 编译器实现，将goroutine的select receive(v = <-c)语句转成对应的方法执行
 * @param
 * @return
 **/
func selectnbrecv(elem unsafe.Pointer, c *hchan) (selected bool) {
	selected, _ = chanrecv(c, elem, false)
	return
}

// compiler implements
//
//	select {
//	case v, ok = <-c:
//		... foo
//	default:
//		... bar
//	}
//
// as
//
//	if c != nil && selectnbrecv2(&v, &ok, c) {
//		... foo
//	} else {
//		... bar
//	}
//
/**
 * 编译器实现，将goroutine的select receive(case v, ok = <-c:)语句转成对应的方法执行
 * @param
 * @return
 **/
func selectnbrecv2(elem unsafe.Pointer, received *bool, c *hchan) (selected bool) {
	// TODO(khr): just return 2 values from this function, now that it is in Go.
	// TODO（khr）：此函数位于Go中，只需返回2个值即可。表示这是一个历史遗留
	selected, *received = chanrecv(c, elem, false)
	return
}

//go:linkname reflect_chansend reflect.chansend
/**
 *
 * go:linkname引导编译器将当前(私有)方法或者变量在编译时链接到指定的位置的方法或者变量，
 * 第一个参数表示当前方法或变量，第二个参数表示目标方法或变量，因为这关指令会破坏系统和包的模块化，
 * 因此在使用时必须导入unsafe
 * 参见：https://blog.csdn.net/lastsweetop/article/details/78830772
 * @param
 * @return
 **/
func reflect_chansend(c *hchan, elem unsafe.Pointer, nb bool) (selected bool) {
	return chansend(c, elem, !nb, getcallerpc())
}

//go:linkname reflect_chanrecv reflect.chanrecv
func reflect_chanrecv(c *hchan, nb bool, elem unsafe.Pointer) (selected bool, received bool) {
	return chanrecv(c, elem, !nb)
}

//go:linkname reflect_chanlen reflect.chanlen
func reflect_chanlen(c *hchan) int {
	if c == nil {
		return 0
	}
	return int(c.qcount)
}

//go:linkname reflectlite_chanlen internal/reflectlite.chanlen
func reflectlite_chanlen(c *hchan) int {
	if c == nil {
		return 0
	}
	return int(c.qcount)
}

//go:linkname reflect_chancap reflect.chancap
func reflect_chancap(c *hchan) int {
	if c == nil {
		return 0
	}
	return int(c.dataqsiz)
}

//go:linkname reflect_chanclose reflect.chanclose
func reflect_chanclose(c *hchan) {
	closechan(c)
}

/**
 * 入队操作
 * @param sudog 需要入队的元素
 * @return
 **/
func (q *waitq) enqueue(sgp *sudog) {
	sgp.next = nil
	x := q.last
	if x == nil { // 队列中没有元素
		sgp.prev = nil
		q.first = sgp
		q.last = sgp
		return
	}

	// 队列中已经有元素
	sgp.prev = x
	x.next = sgp
	q.last = sgp
}

/**
 * 出队
 * @param
 * @return
 **/
func (q *waitq) dequeue() *sudog {
	for {
		sgp := q.first
		if sgp == nil { // 队列中没有元素
			return nil
		}
		y := sgp.next
		if y == nil { // 队列中只有一个元素
			q.first = nil
			q.last = nil
		} else {
			y.prev = nil
			q.first = y
			sgp.next = nil // mark as removed (see dequeueSudog) // 标记为已删除（请参阅dequeueSudog）
		}

		// if a goroutine was put on this queue because of a
		// select, there is a small window between the goroutine
		// being woken up by a different case and it grabbing the
		// channel locks. Once it has the lock
		// it removes itself from the queue, so we won't see it after that.
		// We use a flag in the G struct to tell us when someone
		// else has won the race to signal this goroutine but the goroutine
		// hasn't removed itself from the queue yet.
		// 如果由于选择而将goroutine放在此队列中，则在其他情况下唤醒goroutine并获取通道锁之间会有一个小窗口。
		// 一旦拥有了锁，它就会将自己从队列中删除，因此之后我们将看不到它。
        // 我们在G结构中使用一个标志来告诉我们何时其他人赢得了发信号通知此goroutine的竞赛，
        // 但goroutine尚未将自己从队列中删除。
		if sgp.isSelect && !atomic.Cas(&sgp.g.selectDone, 0, 1) {
			continue
		}

		return sgp
	}
}

func (c *hchan) raceaddr() unsafe.Pointer {
	// Treat read-like and write-like operations on the channel to
	// happen at this address. Avoid using the address of qcount
	// or dataqsiz, because the len() and cap() builtins read
	// those addresses, and we don't want them racing with
	// operations like close().
	// 将通道上的读取和写入操作视为在此地址发生。 避免使用qcount或dataqsiz的地址，
	// 因为内置的len()和cap()会读取这些地址，并且我们不希望它们与close()之类的操作竞争。
	return unsafe.Pointer(&c.buf)
}

/**
 * 这个方法现在看来没有什么用了
 * @param
 * @return
 **/
func racesync(c *hchan, sg *sudog) {
	racerelease(chanbuf(c, 0))
	raceacquireg(sg.g, chanbuf(c, 0))
	racereleaseg(sg.g, chanbuf(c, 0))
	raceacquire(chanbuf(c, 0))
}
```