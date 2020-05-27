```go
package runtime

import (
	"internal/cpu"
	"runtime/internal/atomic"
	"runtime/internal/sys"
	"unsafe"
)

// defined constants
const (
	// G status
	//
	// Beyond indicating the general state of a G, the G status
	// acts like a lock on the goroutine's stack (and hence its
	// ability to execute user code).
	//
	// If you add to this list, add to the list
	// of "okay during garbage collection" status
	// in mgcmark.go too.
	//
	// TODO(austin): The _Gscan bit could be much lighter-weight.
	// For example, we could choose not to run _Gscanrunnable
	// goroutines found in the run queue, rather than CAS-looping
	// until they become _Grunnable. And transitions like
	// _Gscanwaiting -> _Gscanrunnable are actually okay because
	// they don't affect stack ownership.

	// _Gidle means this goroutine was just allocated and has not
	// yet been initialized.
	//
	// G状态
    //
    // 除了指示G的一般状态外，G状态还像goroutine堆栈上的锁一样（因此具有执行用户代码的能力）。
    //
    // 如果添加到此列表，则也添加到mgcmark.go中的“垃圾收集期间可以”状态列表。
    //
    // TODO（奥斯汀）：_Gscan位的重量可能更轻。
    // 例如，我们可以选择不运行在运行队列中找到的_Gscanrunnable goroutine，
    // 而不是运行CAS循环直到它们变为_Grunnable。像_Gscanwaiting-> _Gscanrunnable
    // 这样的转换实际上是可以的，因为它们不会影响堆栈所有权。

    // _Gidle表示此goroutine已分配，尚未初始化。
	_Gidle = iota // 0

	// _Grunnable means this goroutine is on a run queue. It is
	// not currently executing user code. The stack is not owned.
	//
	// _Grunnable表示此goroutine在运行队列中。当前未执行用户代码。堆栈不拥有此goroutine。
	_Grunnable // 1

	// _Grunning means this goroutine may execute user code. The
	// stack is owned by this goroutine. It is not on a run queue.
	// It is assigned an M and a P (g.m and g.m.p are valid).
	//
	// _Grunning表示该goroutine可以执行用户代码。堆栈由该goroutine拥有。它不在运行队列中。
    // 为它分配一个M和一个P（g.m和g.m.p有效）。
	_Grunning // 2

	// _Gsyscall means this goroutine is executing a system call.
	// It is not executing user code. The stack is owned by this
	// goroutine. It is not on a run queue. It is assigned an M.
	//
	// _Gsyscall表示此goroutine正在执行系统调用。
    // 它不执行用户代码。堆栈由该goroutine拥有。它不在运行队列中。它被分配了一个M。
	_Gsyscall // 3

	// _Gwaiting means this goroutine is blocked in the runtime.
	// It is not executing user code. It is not on a run queue,
	// but should be recorded somewhere (e.g., a channel wait
	// queue) so it can be ready()d when necessary. The stack is
	// not owned *except* that a channel operation may read or
	// write parts of the stack under the appropriate channel
	// lock. Otherwise, it is not safe to access the stack after a
	// goroutine enters _Gwaiting (e.g., it may get moved).
	//
	// _Gwaiting表示此goroutine在运行时被阻止。
    // 它不执行用户代码。它不在运行队列中，但应记录在某个地方（例如，通道等待队列），以便在必要时可以ready（）d。
    // *除*通道操作可以在适当的通道锁定下读取或写入堆栈的某些部分外，不拥有该堆栈。
    // 否则，在goroutine进入_Gwaiting之后（例如，它可能被移动），访问堆栈是不安全的。
	_Gwaiting // 4

	// _Gmoribund_unused is currently unused, but hardcoded in gdb
	// scripts.
	//
	// _Gmoribund_unused当前未使用，但在gdb脚本中进行了硬编码。
	_Gmoribund_unused // 5

	// _Gdead means this goroutine is currently unused. It may be
	// just exited, on a free list, or just being initialized. It
	// is not executing user code. It may or may not have a stack
	// allocated. The G and its stack (if any) are owned by the M
	// that is exiting the G or that obtained the G from the free
	// list.
	//
	// _Gdead表示此goroutine当前未使用。它可能刚刚退出，处于空闲列表中或刚刚被初始化。
	// 它不执行用户代码。它可能分配也可能没有分配堆栈。 G及其堆栈（如果有）由退出G或从空闲列表中获得G的M拥有。
	_Gdead // 6

	// _Genqueue_unused is currently unused.
	//
	// _Genqueue_unused当前未使用。
	_Genqueue_unused // 7

	// _Gcopystack means this goroutine's stack is being moved. It
	// is not executing user code and is not on a run queue. The
	// stack is owned by the goroutine that put it in _Gcopystack.
	//
	// _Gcopystack表示此goroutine的堆栈正在移动。它不执行用户代码，也不在运行队列中。
	// 堆栈由将其放入_Gcopystack的goroutine拥有。
	_Gcopystack // 8

	// _Gpreempted means this goroutine stopped itself for a
	// suspendG preemption. It is like _Gwaiting, but nothing is
	// yet responsible for ready()ing it. Some suspendG must CAS
	// the status to _Gwaiting to take responsibility for
	// ready()ing this G.
	//
	// _Gpreempted表示此goroutine停止了自身的suspendG抢占。就像_Gwaiting，但是尚无任何准备就绪的东西。
	// 某些suspendG必须将CAS的状态变为_Gwaiting，以负责为此G做好准备。
	_Gpreempted // 9

	// _Gscan combined with one of the above states other than
	// _Grunning indicates that GC is scanning the stack. The
	// goroutine is not executing user code and the stack is owned
	// by the goroutine that set the _Gscan bit.
	//
	// _Gscanrunning is different: it is used to briefly block
	// state transitions while GC signals the G to scan its own
	// stack. This is otherwise like _Grunning.
	//
	// atomicstatus&~Gscan gives the state the goroutine will
	// return to when the scan completes.
	//
	// _Gscan与_Grunning以外的上述状态之一组合表示GC正在扫描堆栈。 goroutine未执行用户代码，并且堆栈由设置_Gscan位的goroutine拥有。
    //
    // _Gscanrunning是不同的：它用于短暂阻止状态转换，而GC会通知G扫描其自己的堆栈。否则就像_Grunning。
    //
    // atomicstatus＆〜Gscan会在扫描完成时返回状态。
	_Gscan          = 0x1000
	_Gscanrunnable  = _Gscan + _Grunnable  // 0x1001
	_Gscanrunning   = _Gscan + _Grunning   // 0x1002
	_Gscansyscall   = _Gscan + _Gsyscall   // 0x1003
	_Gscanwaiting   = _Gscan + _Gwaiting   // 0x1004
	_Gscanpreempted = _Gscan + _Gpreempted // 0x1009
)

const (
	// P status

	// _Pidle means a P is not being used to run user code or the
	// scheduler. Typically, it's on the idle P list and available
	// to the scheduler, but it may just be transitioning between
	// other states.
	//
	// The P is owned by the idle list or by whatever is
	// transitioning its state. Its run queue is empty.
	//
	// _Pidle表示未使用P来运行用户代码或调度程序。通常，它在空闲的P列表中，可供调度程序使用，但可能只是在其他状态之间转换。
    //
    // P由空闲列表或正在转换其状态的任何对象拥有。它的运行队列为空。
	_Pidle = iota

	// _Prunning means a P is owned by an M and is being used to
	// run user code or the scheduler. Only the M that owns this P
	// is allowed to change the P's status from _Prunning. The M
	// may transition the P to _Pidle (if it has no more work to
	// do), _Psyscall (when entering a syscall), or _Pgcstop (to
	// halt for the GC). The M may also hand ownership of the P
	// off directly to another M (e.g., to schedule a locked G).
	//
	// _Prunning表示P由M拥有，并用于运行用户代码或调度程序。仅拥有该P的M允许从_Prunning更改P的状态。
	// M可以将P转换为_Pidle（如果没有更多工作要做），_ Psyscall（进入系统调用时）或_Pgcstop（以停止GC）。
	// M还可以将P的所有权直接移交给另一个M（例如，以调度锁定的G）。
	_Prunning

	// _Psyscall means a P is not running user code. It has
	// affinity to an M in a syscall but is not owned by it and
	// may be stolen by another M. This is similar to _Pidle but
	// uses lightweight transitions and maintains M affinity.
	//
	// Leaving _Psyscall must be done with a CAS, either to steal
	// or retake the P. Note that there's an ABA hazard: even if
	// an M successfully CASes its original P back to _Prunning
	// after a syscall, it must understand the P may have been
	// used by another M in the interim.
	//
	// _Psyscall表示P没有运行用户代码。它与系统调用中的M有亲缘关系，但不属于它，并且可能被另一个M窃取。
	// 这类似于_Pidle，但是使用轻量级转换并保持M亲和力。
    //
    // 离开_Psyscall必须通过CAS，才能窃取或重新获得P。
    // 请注意，这有ABA的危险：即使M在syscall后成功将其原始P返回到_Prunning，它也必须了解P可能已经在此期间由另一个M使用。
	_Psyscall

	// _Pgcstop means a P is halted for STW and owned by the M
	// that stopped the world. The M that stopped the world
	// continues to use its P, even in _Pgcstop. Transitioning
	// from _Prunning to _Pgcstop causes an M to release its P and
	// park.
	//
	// The P retains its run queue and startTheWorld will restart
	// the scheduler on Ps with non-empty run queues.
	//
	// _Pgcstop表示P被STW暂停，由全局停机的M拥有。全局停机的M甚至在_Pgcstop中也继续使用其P。
	// 从_Prunning过渡到_Pgcstop会导致M释放其P并停放。
    //
    // P保留其运行队列，startTheWorld将在具有非空运行队列的Ps上重新启动调度程序。
	_Pgcstop

	// _Pdead means a P is no longer used (GOMAXPROCS shrank). We
	// reuse Ps if GOMAXPROCS increases. A dead P is mostly
	// stripped of its resources, though a few things remain
	// (e.g., trace buffers).
	//
	// _Pdead表示不再使用P（GOMAXPROCS缩小）。如果GOMAXPROCS增加，我们将重用P。
	// 一个死掉的P大部分被剥夺了其资源，尽管还剩下一些东西（例如跟踪缓冲区）。
	_Pdead
)

// Mutual exclusion locks.  In the uncontended case,
// as fast as spin locks (just a few user-level instructions),
// but on the contention path they sleep in the kernel.
// A zeroed Mutex is unlocked (no need to initialize each lock).
//
// 互斥锁。在无竞争的情况下，其速度与自旋锁一样快（只有一些用户级指令），
// 但是在争用路径上它们却在内核中休眠。零位的互斥锁被解锁（无需初始化每个锁）。
type mutex struct {
	// Futex-based impl treats it as uint32 key,
	// while sema-based impl as M* waitm.
	// Used to be a union, but unions break precise GC.
	// 基于Futex的实现将其视为uint32键，而基于sema的实验则视为M* waitm。过去曾经是一个联合，但是联合破坏了精确的GC。
	key uintptr
}

// sleep and wakeup on one-time events.
// before any calls to notesleep or notewakeup,
// must call noteclear to initialize the Note.
// then, exactly one thread can call notesleep
// and exactly one thread can call notewakeup (once).
// once notewakeup has been called, the notesleep
// will return.  future notesleep will return immediately.
// subsequent noteclear must be called only after
// previous notesleep has returned, e.g. it's disallowed
// to call noteclear straight after notewakeup.
//
// notetsleep is like notesleep but wakes up after
// a given number of nanoseconds even if the event
// has not yet happened.  if a goroutine uses notetsleep to
// wake up early, it must wait to call noteclear until it
// can be sure that no other goroutine is calling
// notewakeup.
//
// notesleep/notetsleep are generally called on g0,
// notetsleepg is similar to notetsleep but is called on user g.
//
// 一次性事件的睡眠和唤醒。在任何调用notesleep或notewakeup之前，必须调用noteclear来初始化Note。
// 那么，恰好一个线程可以调用notesleep，恰好一个线程可以调用notewakeup（一次）。一旦调用了notewakeup，
// 便会返回notesleep。将来的notesleep将立即返回。后续的noteclear必须仅在先前的notesleep返回后才能调用，
// 例如禁止在notewakeup之后立即调用noteclear。
//
// notetsleep类似于notesleep，但是即使事件尚未发生，也会在给定的纳秒数后唤醒。如果goroutine使用notetsleep提前唤醒，
// 则必须等待调用noteclear，直到可以确定没有其他goroutine正在调用notewakeup。
//
// 通常在g0上调用notesleep/notetsleep，notetsleepg与notetsleep类似，但在用户g上调用
type note struct {
	// Futex-based impl treats it as uint32 key,
	// while sema-based impl as M* waitm.
	// Used to be a union, but unions break precise GC.
	// 基于Futex的实现将其视为uint32键，而基于sema的实验则视为M* waitm。过去曾经是一个联合，但是联合破坏了精确的GC。
	key uintptr
}

type funcval struct {
	fn uintptr
	// variable-size, fn-specific data here
}

type iface struct {
	tab  *itab
	data unsafe.Pointer
}

type eface struct {
	_type *_type
	data  unsafe.Pointer
}

func efaceOf(ep *interface{}) *eface {
	return (*eface)(unsafe.Pointer(ep))
}

// The guintptr, muintptr, and puintptr are all used to bypass write barriers.
// It is particularly important to avoid write barriers when the current P has
// been released, because the GC thinks the world is stopped, and an
// unexpected write barrier would not be synchronized with the GC,
// which can lead to a half-executed write barrier that has marked the object
// but not queued it. If the GC skips the object and completes before the
// queuing can occur, it will incorrectly free the object.
//
// We tried using special assignment functions invoked only when not
// holding a running P, but then some updates to a particular memory
// word went through write barriers and some did not. This breaks the
// write barrier shadow checking mode, and it is also scary: better to have
// a word that is completely ignored by the GC than to have one for which
// only a few updates are ignored.
//
// Gs and Ps are always reachable via true pointers in the
// allgs and allp lists or (during allocation before they reach those lists)
// from stack variables.
//
// Ms are always reachable via true pointers either from allm or
// freem. Unlike Gs and Ps we do free Ms, so it's important that
// nothing ever hold an muintptr across a safe point.

// A guintptr holds a goroutine pointer, but typed as a uintptr
// to bypass write barriers. It is used in the Gobuf goroutine state
// and in scheduling lists that are manipulated without a P.
//
// The Gobuf.g goroutine pointer is almost always updated by assembly code.
// In one of the few places it is updated by Go code - func save - it must be
// treated as a uintptr to avoid a write barrier being emitted at a bad time.
// Instead of figuring out how to emit the write barriers missing in the
// assembly manipulation, we change the type of the field to uintptr,
// so that it does not require write barriers at all.
//
// Goroutine structs are published in the allg list and never freed.
// That will keep the goroutine structs from being collected.
// There is never a time that Gobuf.g's contain the only references
// to a goroutine: the publishing of the goroutine in allg comes first.
// Goroutine pointers are also kept in non-GC-visible places like TLS,
// so I can't see them ever moving. If we did want to start moving data
// in the GC, we'd need to allocate the goroutine structs from an
// alternate arena. Using guintptr doesn't make that problem any worse.
//
// guintptr，muintptr和puintptr都用于绕过写屏障。释放当前P时避免写屏障尤为重要，
// 因为GC认为全局已停机，并且意外的写屏障不会与GC同步，这可能导致半执行的写屏障标记了对象但未将其排队。
// 如果GC跳过对象并在排队之前完成，它将错误地释放对象。
//
// 我们尝试使用仅在不持有运行P的情况下才调用的特殊赋值函数，但随后对特定存储字的某些更新会遇到写屏障，而某些则不会。
// 这打破了写屏障阴影检查模式，而且也很可怕：拥有一个被GC完全忽略的单词比拥有一个只被少数更新忽略的单词更好。
//
// Gs和Ps始终可以通过allgs和allp列表中的真实指针或（从分配过程中到达它们的列表之前）堆栈变量中的真实指针访问。
//
// 总是可以通过来自allm或freem的真实指针来访问M。与Gs和Ps不同，我们免费提供Ms，因此，任何人都不能在安全点上持有muintptr，这一点很重要。

// guintptr拥有一个goroutine指针，但被键入为uintptr来绕过写屏障。在Gobuf goroutine状态和不使用P进行操作的调度列表中使用。
//
// Gobuf.g goroutine指针几乎总是由汇编代码更新。在少数几个地方，它会通过Go代码进行更新-func save-必须将其视为uintptr，
// 以避免在不好的时候发出写屏障。我们没有弄清楚如何发出汇编操作中缺少的写屏障，而是将字段的类型更改为uintptr，因此它根本不需要写屏障。
//
// Goroutine结构在allg列表中发布，并且从不释放。这样可以避免收集goroutine结构。从来没有机会Gobuf.g仅包含对goroutine的引用：首先在allg中发布goroutine。
// Goroutine指针也保存在TLS等非GC可见的位置，因此我看不到它们一直在移动。如果确实要在GC中开始移动数据，则需要从备用区域分配goroutine结构。使用guintptr不会使这个问题更严重。
type guintptr uintptr

//go:nosplit
func (gp guintptr) ptr() *g { return (*g)(unsafe.Pointer(gp)) }

//go:nosplit
func (gp *guintptr) set(g *g) { *gp = guintptr(unsafe.Pointer(g)) }

//go:nosplit
func (gp *guintptr) cas(old, new guintptr) bool {
	return atomic.Casuintptr((*uintptr)(unsafe.Pointer(gp)), uintptr(old), uintptr(new))
}

// setGNoWB performs *gp = new without a write barrier.
// For times when it's impractical to use a guintptr.
//
// setGNoWB执行*gp = new，没有写屏障。有时无法使用guintptr。
//go:nosplit
//go:nowritebarrier
func setGNoWB(gp **g, new *g) {
	(*guintptr)(unsafe.Pointer(gp)).set(new)
}

type puintptr uintptr

//go:nosplit
func (pp puintptr) ptr() *p { return (*p)(unsafe.Pointer(pp)) }

//go:nosplit
func (pp *puintptr) set(p *p) { *pp = puintptr(unsafe.Pointer(p)) }

// muintptr is a *m that is not tracked by the garbage collector.
//
// Because we do free Ms, there are some additional constrains on
// muintptrs:
//
// 1. Never hold an muintptr locally across a safe point.
//
// 2. Any muintptr in the heap must be owned by the M itself so it can
//    ensure it is not in use when the last true *m is released.
//
// muintptr是垃圾收集器未跟踪的* m。
//
// 因为我们释放m，所以在muintptrs上还有一些其他限制：
//
// 1.切勿在安全点附近本地持有muintptr。
//
// 2.堆中的任何muintptr都必须归M自己所有，这样它可以确保在释放最后一个true * m时不使用它。
type muintptr uintptr

//go:nosplit
func (mp muintptr) ptr() *m { return (*m)(unsafe.Pointer(mp)) }

//go:nosplit
func (mp *muintptr) set(m *m) { *mp = muintptr(unsafe.Pointer(m)) }

// setMNoWB performs *mp = new without a write barrier.
// For times when it's impractical to use an muintptr.
//
// setMNoWB执行*mp = new，没有写屏障。有时不适合使用muintptr。
//go:nosplit
//go:nowritebarrier
func setMNoWB(mp **m, new *m) {
	(*muintptr)(unsafe.Pointer(mp)).set(new)
}

type gobuf struct {
	// The offsets of sp, pc, and g are known to (hard-coded in) libmach.
	//
	// ctxt is unusual with respect to GC: it may be a
	// heap-allocated funcval, so GC needs to track it, but it
	// needs to be set and cleared from assembly, where it's
	// difficult to have write barriers. However, ctxt is really a
	// saved, live register, and we only ever exchange it between
	// the real register and the gobuf. Hence, we treat it as a
	// root during stack scanning, which means assembly that saves
	// and restores it doesn't need write barriers. It's still
	// typed as a pointer so that any other writes from Go get
	// write barriers.
	//
	// libmach已知（硬编码在其中）sp，pc和g的偏移量。
    //
    // ctxt对于GC不常见：它可能是堆分配的funcval，因此GC需要对其进行跟踪，但需要对其进行设置并将其从汇编中清除，这在这里很难实现写屏障。
    // 但是，ctxt实际上是一个保存的实时寄存器，我们只在真实寄存器和gobuf之间交换它。因此，我们在堆栈扫描期间将其视为根，
    // 这意味着保存和恢复它的程序集不需要写屏障。它仍然被用作指针，以便Go进行的任何其他写入都会遇到写入屏障。
	sp   uintptr
	pc   uintptr
	g    guintptr
	ctxt unsafe.Pointer
	ret  sys.Uintreg
	lr   uintptr
	bp   uintptr // for GOEXPERIMENT=framepointer
}

// sudog represents a g in a wait list, such as for sending/receiving
// on a channel.
//
// sudog is necessary because the g ↔ synchronization object relation
// is many-to-many. A g can be on many wait lists, so there may be
// many sudogs for one g; and many gs may be waiting on the same
// synchronization object, so there may be many sudogs for one object.
//
// sudogs are allocated from a special pool. Use acquireSudog and
// releaseSudog to allocate and free them.
//
// sudog在等待列表中表示g，例如用于在通道上发送/接收。
//
// sudog是必需的，因为g↔同步对象关系是多对多的。一个g可以出现在许多等待列表上，因此一个g可能有很多sudog。
// 并且许多g可能正在等待同一个同步对象，因此一个对象可能有许多sudog。
//
// sudog是从特殊池中分配的。使用acquireSudog和releaseSudog分配和释放它们。
type sudog struct {
	// The following fields are protected by the hchan.lock of the
	// channel this sudog is blocking on. shrinkstack depends on
	// this for sudogs involved in channel ops.
	//
	// 以下字段受此sudog阻止的通道的hchan.lock保护。对于参与通道操作的sudog，srinkstack依赖于此。

	g *g

	// isSelect indicates g is participating in a select, so
	// g.selectDone must be CAS'd to win the wake-up race.
	// isSelect表示g正在参与选择，因此必须对g.selectDone进行CAS才能赢得竞争。
	isSelect bool
	next     *sudog
	prev     *sudog
	elem     unsafe.Pointer // data element (may point to stack) // 数据元素（可能指向堆栈）

	// The following fields are never accessed concurrently.
	// For channels, waitlink is only accessed by g.
	// For semaphores, all fields (including the ones above)
	// are only accessed when holding a semaRoot lock.
	//
	// 绝不能同时访问以下字段。
    // 对于通道，waitlink仅由g访问。
    // 对于信号量，仅当持有semaRoot锁时才能访问所有字段（包括上述字段）。

	acquiretime int64
	releasetime int64
	ticket      uint32
	parent      *sudog // semaRoot binary tree // semaRoot二叉树
	waitlink    *sudog // g.waiting list or semaRoot g.waiting列表或semaRoot
	waittail    *sudog // semaRoot
	c           *hchan // channel
}

type libcall struct {
	fn   uintptr
	n    uintptr // number of parameters // 参数数量
	args uintptr // parameters // 参数
	r1   uintptr // return values // 返回值
	r2   uintptr
	err  uintptr // error number // 错误码
}

// describes how to handle callback
// 描述如何处理回调
type wincallbackcontext struct {
	gobody       unsafe.Pointer // go function to call // go函数调用
	argsize      uintptr        // callback arguments size (in bytes) // 回调参数的大小（以字节为单位）
	restorestack uintptr        // adjust stack on return by (in bytes) (386 only) // 调整返回时的堆栈（以字节为单位）（仅386）
	cleanstack   bool
}

// Stack describes a Go execution stack.
// The bounds of the stack are exactly [lo, hi),
// with no implicit data structures on either side.
//
// 堆栈描述了Go执行堆栈。堆栈的边界正好是[lo，hi），在每一侧都没有隐式数据结构。
type stack struct {
	lo uintptr
	hi uintptr
}

type g struct {
	// Stack parameters.
	// stack describes the actual stack memory: [stack.lo, stack.hi).
	// stackguard0 is the stack pointer compared in the Go stack growth prologue.
	// It is stack.lo+StackGuard normally, but can be StackPreempt to trigger a preemption.
	// stackguard1 is the stack pointer compared in the C stack growth prologue.
	// It is stack.lo+StackGuard on g0 and gsignal stacks.
	// It is ~0 on other goroutine stacks, to trigger a call to morestackc (and crash).
	//
	// 堆栈参数。
    // 堆栈描述实际的堆栈内存：[stack.lo，stack.hi）。
    // stackguard0是在Go堆栈增长序言中比较的堆栈指针。
    // 通常是stack.lo + StackGuard，但是可以通过StackPreempt触发抢占。
    // stackguard1是在C堆栈增长序言中比较的堆栈指针。
    // 它是g0和gsignal堆栈上的stack.lo + StackGuard。
    // 在其他goroutine堆栈上为〜0，以触发对morestackc的调用（并崩溃）。
	stack       stack   // offset known to runtime/cgo // runtime/cgo已知的偏移量
	stackguard0 uintptr // offset known to liblink // liblink已知的偏移量
	stackguard1 uintptr // offset known to liblink // liblink已知的偏移量

	_panic       *_panic // innermost panic - offset known to liblink //最内层的恐慌-liblink已知的偏移量
	_defer       *_defer // innermost defer //最内层的defer
	m            *m      // current m; offset known to arm liblink // 当前的m;偏移量为“arm liblink”已知
	sched        gobuf
	syscallsp    uintptr        // if status==Gsyscall, syscallsp = sched.sp to use during gc // 如果status == Syscall，则syscall = schedule.sp在gc期间使用
	syscallpc    uintptr        // if status==Gsyscall, syscallpc = sched.pc to use during gc // 如果status == Syscall，则syscall = schedule.pc在gc期间使用
	stktopsp     uintptr        // expected sp at top of stack, to check in traceback // 预期sp位于堆栈顶部，回溯时检验
	param        unsafe.Pointer // passed parameter on wakeup // 唤醒时传递的参数
	atomicstatus uint32
	stackLock    uint32 // sigprof/scang lock; TODO: fold in to atomicstatus // sigprof/scang锁定； TODO：折入atomicstatus
	goid         int64
	schedlink    guintptr
	waitsince    int64      // approx time when the g become blocked // g被阻塞的大约时间
	waitreason   waitReason // if status==Gwaiting

	preempt       bool // preemption signal, duplicates stackguard0 = stackpreempt // 抢占信号，重复stackguard0 = stackpreempt
	preemptStop   bool // transition to _Gpreempted on preemption; otherwise, just deschedule //抢占时过渡到_Gpreempted；否则，只是排期
	preemptShrink bool // shrink stack at synchronous safe point // 在同步安全点收缩堆栈

	// asyncSafePoint is set if g is stopped at an asynchronous
	// safe point. This means there are frames on the stack
	// without precise pointer information.
	// 如果g在异步安全点停止，则设置asyncSafePoint。这意味着堆栈中的某些帧没有精确的指针信息。
	asyncSafePoint bool

	paniconfault bool // panic (instead of crash) on unexpected fault address // 对意外错误地址进行恐慌（而不是崩溃）
	gcscandone   bool // g has scanned stack; protected by _Gscan bit in status // g已扫描堆栈；受状态中的_Gscan位保护
	throwsplit   bool // must not split stack // 不得拆分堆栈
	// activeStackChans indicates that there are unlocked channels
	// pointing into this goroutine's stack. If true, stack
	// copying needs to acquire channel locks to protect these
	// areas of the stack.
	//
	// activeStackChans指示有指向该goroutine堆栈的未锁定通道。如果为true，则堆栈复制需要获取通道锁以保护堆栈的这些区域。
	activeStackChans bool

	raceignore     int8     // ignore race detection events // 忽略竞态检测事件
	sysblocktraced bool     // StartTrace has emitted EvGoInSyscall about this goroutine // StartTrace已发出有关此goroutine的EvGoInSyscall
	sysexitticks   int64    // cputicks when syscall has returned (for tracing) // 返回系统调用后的cputicks（用于跟踪）
	traceseq       uint64   // trace event sequencer //跟踪事件序列器
	tracelastp     puintptr // last P emitted an event for this goroutine // 最后一个P为此goroutine发出了一个事件
	lockedm        muintptr
	sig            uint32
	writebuf       []byte
	sigcode0       uintptr
	sigcode1       uintptr
	sigpc          uintptr
	gopc           uintptr         // pc of go statement that created this goroutine // 创建该goroutine的go语句的pc
	ancestors      *[]ancestorInfo // ancestor information goroutine(s) that created this goroutine (only used if debug.tracebackancestors) // 创建此goroutine的祖先信息goroutine（仅在debug.tracebackancestors中使用）
	startpc        uintptr         // pc of goroutine function // pc goroutine函数
	racectx        uintptr
	waiting        *sudog         // sudog structures this g is waiting on (that have a valid elem ptr); in lock order // 这个g正在等待的sudog结构（具有有效的elem ptr）；锁定顺序
	cgoCtxt        []uintptr      // cgo traceback context // cgo回溯背景
	labels         unsafe.Pointer // profiler labels //分析器标签
	timer          *timer         // cached timer for time.Sleep // 缓存时间的计时器
	selectDone     uint32         // are we participating in a select and did someone win the race? // 我们是否参加了选择竞争，有人赢得了竞争吗？

	// Per-G GC state

	// gcAssistBytes is this G's GC assist credit in terms of
	// bytes allocated. If this is positive, then the G has credit
	// to allocate gcAssistBytes bytes without assisting. If this
	// is negative, then the G must correct this by performing
	// scan work. We track this in bytes to make it fast to update
	// and check for debt in the malloc hot path. The assist ratio
	// determines how this corresponds to scan work debt.
	//
	// gcAssistBytes是该G的GC辅助功劳，以分配的字节数表示。如果这是正数，则G可以分配gcAssistBytes字节而无需协助。
	// 如果结果是负数的，则G必须通过执行扫描工作来更正此问题。我们以字节为单位跟踪此记录，
	// 以使其快速更新并检查malloc热路径中的债务。协助比率确定这与扫描工作债务的对应关系。
	gcAssistBytes int64
}

type m struct {
	g0      *g     // goroutine with scheduling stack //具有调度堆栈的goroutine
	morebuf gobuf  // gobuf arg to morestack
	divmod  uint32 // div/mod denominator for arm - known to liblink // arm的div/mod分母-liblink已知

	// Fields not known to debuggers. // 调试器不知道的字段。
	procid        uint64       // for debuggers, but offset not hard-coded //用于调试器，但偏移量不是硬编码的
	gsignal       *g           // signal-handling g // 信号处理g
	goSigStack    gsignalStack // Go-allocated signal handling stack // 分配信号处理堆栈
	sigmask       sigset       // storage for saved signal mask // 存储已保存的信号掩码
	tls           [6]uintptr   // thread-local storage (for x86 extern register) // 线程本地存储（用于x86 extern寄存器）
	mstartfn      func()
	curg          *g       // current running goroutine // 当前正在运行的goroutine
	caughtsig     guintptr // goroutine running during fatal signal // goroutine在致命信号期间运行
	p             puintptr // attached p for executing go code (nil if not executing go code) // 附加p用于执行执行代码（如果不执行执行代码，则为nil）
	nextp         puintptr
	oldp          puintptr // the p that was attached before executing a syscall //执行系统调用之前附加的p
	id            int64
	mallocing     int32
	throwing      int32
	preemptoff    string // if != "", keep curg running on this m // 如果 !=""，则在此m上继续运行curg
	locks         int32
	dying         int32
	profilehz     int32
	spinning      bool // m is out of work and is actively looking for work // m没有工作，正在积极寻找工作
	blocked       bool // m is blocked on a note // m被阻塞在note上
	newSigstack   bool // minit on C thread called sigaltstack // 在名为sigaltstack的C线程上的minit
	printlock     int8
	incgo         bool   // m is executing a cgo call // m正在执行cgo调用
	freeWait      uint32 // if == 0, safe to free g0 and delete m (atomic) // 如果== 0，则可以安全地释放g0并删除m（原子地）
	fastrand      [2]uint32
	needextram    bool
	traceback     uint8
	ncgocall      uint64      // number of cgo calls in total // 总的cgo调用数
	ncgo          int32       // number of cgo calls currently in progress // 当前正在进行的cgo调用数
	cgoCallersUse uint32      // if non-zero, cgoCallers in use temporarily // 如果非零，则cgoCallers会暂时使用
	cgoCallers    *cgoCallers // cgo traceback if crashing in cgo call // 如果cgo调用崩溃，则cgo追溯
	park          note
	alllink       *m // on allm
	schedlink     muintptr
	mcache        *mcache
	lockedg       guintptr
	createstack   [32]uintptr // stack that created this thread. // 创建该线程的堆栈。
	lockedExt     uint32      // tracking for external LockOSThread // 跟踪外部LockOSThread
	lockedInt     uint32      // tracking for internal lockOSThread // 跟踪内部lockOSThread
	nextwaitm     muintptr    // next m waiting for lock // 下一个等待锁
	waitunlockf   func(*g, unsafe.Pointer) bool
	waitlock      unsafe.Pointer
	waittraceev   byte
	waittraceskip int
	startingtrace bool
	syscalltick   uint32
	freelink      *m // on sched.freem // 在sched.freem上

	// these are here because they are too large to be on the stack
	// of low-level NOSPLIT functions.
	// 这些在这里是因为它们太大了，无法放在低级NOSPLIT函数的堆栈中。
	libcall   libcall
	libcallpc uintptr // for cpu profiler // 用于cpu分析
	libcallsp uintptr
	libcallg  guintptr
	syscall   libcall // stores syscall parameters on windows // 在Windows上存储syscall参数

	vdsoSP uintptr // SP for traceback while in VDSO call (0 if not in call) // SP在VDSO调用中用于回溯（如果未在调用中，则为0）
	vdsoPC uintptr // PC for traceback while in VDSO call // PC在VDSO调用中进行回溯

	// preemptGen counts the number of completed preemption
	// signals. This is used to detect when a preemption is
	// requested, but fails. Accessed atomically.
	//
	// preemptGen计算完成的抢占信号的数量。这用于检测何时请求抢占但失败。原子访问。
	preemptGen uint32

	// Whether this is a pending preemption signal on this M.
	// Accessed atomically.
	// 这是否是此M上的未决抢占信号。以原子方式访问。
	signalPending uint32

	dlogPerM

	mOS
}

type p struct {
	id          int32
	status      uint32 // one of pidle/prunning/... //piddle/pruning/...其中之一
	link        puintptr
	schedtick   uint32     // incremented on every scheduler call // 在每次调度程序调用时增加
	syscalltick uint32     // incremented on every system call // 在每次系统调用时增加
	sysmontick  sysmontick // last tick observed by sysmon // sysmon观察到的最后一个滴答声
	m           muintptr   // back-link to associated m (nil if idle) // 反向链接到关联的m（如果空闲则为nil）
	mcache      *mcache
	pcache      pageCache
	raceprocctx uintptr

	deferpool    [5][]*_defer // pool of available defer structs of different sizes (see panic.go) // 不同大小的可用延迟结构池（请参见panic.go）
	deferpoolbuf [5][32]*_defer

	// Cache of goroutine ids, amortizes accesses to runtime·sched.goidgen.
	// goroutine ID的缓存，分摊对runtime.sched.goidgen的访问。
	goidcache    uint64
	goidcacheend uint64

	// Queue of runnable goroutines. Accessed without lock.
	// 可运行goroutine的队列。无锁访问。
	runqhead uint32
	runqtail uint32
	runq     [256]guintptr
	// runnext, if non-nil, is a runnable G that was ready'd by
	// the current G and should be run next instead of what's in
	// runq if there's time remaining in the running G's time
	// slice. It will inherit the time left in the current time
	// slice. If a set of goroutines is locked in a
	// communicate-and-wait pattern, this schedules that set as a
	// unit and eliminates the (potentially large) scheduling
	// latency that otherwise arises from adding the ready'd
	// goroutines to the end of the run queue.
	//
	// // runnext（如果不是nil）是当前G准备好的可运行G，并且如果正在运行的G的时间片中还有时间，
	// 则应在下一个而不是runq中运行。它将继承当前时间片中剩余的时间。如果将一组goroutine锁定为通信等待模式，
	// 则此调度会将其设置为一个单元，并消除（可能很大的）调度延迟，否则该延迟可能是由于将就绪的goroutine添加到运行队列的末尾而引起的。
	runnext guintptr

	// Available G's (status == Gdead) // 可用的G（状态== Gdead）
	gFree struct {
		gList
		n int32
	}

	sudogcache []*sudog
	sudogbuf   [128]*sudog

	// Cache of mspan objects from the heap. // 从堆中缓存mspan对象。
	mspancache struct {
		// We need an explicit length here because this field is used
		// in allocation codepaths where write barriers are not allowed,
		// and eliminating the write barrier/keeping it eliminated from
		// slice updates is tricky, moreso than just managing the length
		// ourselves.
		// 这里我们需要一个明确的长度，因为此字段用于不允许写障碍的分配代码路径中，
		// 而且要消除写障碍/从分片更新中消除它是很棘手的，而不仅仅是自己管理长度
		len int
		buf [128]*mspan
	}

	tracebuf traceBufPtr

	// traceSweep indicates the sweep events should be traced.
	// This is used to defer the sweep start event until a span
	// has actually been swept.
	// traceSweep指示应跟踪扫描事件。这用于推迟扫描开始事件，直到实际扫过一个跨度为止。
	traceSweep bool
	// traceSwept and traceReclaimed track the number of bytes
	// swept and reclaimed by sweeping in the current sweep loop.
	// traceSwept和traceReclaimed跟踪通过在当前扫描循环中进行扫描而清除和回收的字节数。
	traceSwept, traceReclaimed uintptr

	palloc persistentAlloc // per-P to avoid mutex // 每个P避免互斥

	_ uint32 // Alignment for atomic fields below // 为下面的原子字段对齐

	// The when field of the first entry on the timer heap.
	// This is updated using atomic functions.
	// This is 0 if the timer heap is empty.
	// 计时器堆上第一个条目的when字段。这是使用原子函数更新的。如果计时器堆为空，则为0。
	timer0When uint64

	// Per-P GC state
	gcAssistTime         int64    // Nanoseconds in assistAlloc // aidAlloc纳秒数
	gcFractionalMarkTime int64    // Nanoseconds in fractional mark worker (atomic)
	gcBgMarkWorker       guintptr // (atomic) // 后台gc标记指针
	gcMarkWorkerMode     gcMarkWorkerMode // go标记工作模式

	// gcMarkWorkerStartTime is the nanotime() at which this mark
	// worker started.
	// gcMarkWorkerStartTime是此标记工作程序开始的nanotime()。
	gcMarkWorkerStartTime int64

	// gcw is this P's GC work buffer cache. The work buffer is
	// filled by write barriers, drained by mutator assists, and
	// disposed on certain GC state transitions.
	// gcw是此P的GC工作缓冲区高速缓存。工作缓冲区由写屏障填充，由辅助变量耗尽，并放置在某些GC状态转换上。
	gcw gcWork

	// wbBuf is this P's GC write barrier buffer.
	//
	// TODO: Consider caching this in the running G.
	//
	// wbBuf是此P的GC写屏障缓冲区。
    //
    // TODO：考虑将其缓存在正在运行的G中。
	wbBuf wbBuf

	runSafePointFn uint32 // if 1, run sched.safePointFn at next safe point // 如果为1，则在下一个安全点运行sched.safePointFn

	// Lock for timers. We normally access the timers while running
	// on this P, but the scheduler can also do it from a different P.
	// 锁定计时器。我们通常在此P上运行时访问计时器，但是调度程序也可以从其他P上执行此操作。
	timersLock mutex

	// Actions to take at some time. This is used to implement the
	// standard library's time package.
	// Must hold timersLock to access.
	// 有时需要采取的行动。这用于实现标准库的时间包。必须持有timersLock才能访问。
	timers []*timer

	// Number of timers in P's heap.
	// Modified using atomic instructions.
	// P的堆中的计时器数。使用原子指令修改。
	numTimers uint32

	// Number of timerModifiedEarlier timers on P's heap.
	// This should only be modified while holding timersLock,
	// or while the timer status is in a transient state
	// such as timerModifying.
	// P的堆上的timerModifiedEarlier计时器数量。
    // 仅应在保持timersLock或计时器状态为过渡状态（例如timerModifying）时进行修改。
	adjustTimers uint32

	// Number of timerDeleted timers in P's heap.
	// Modified using atomic instructions.
	// P堆中已删除的timer计时器的数量。使用原子指令进行修改。
	deletedTimers uint32

	// Race context used while executing timer functions.
	// 执行计时器函数时使用的竞态上下文。
	timerRaceCtx uintptr

	// preempt is set to indicate that this P should be enter the
	// scheduler ASAP (regardless of what G is running on it).
	// 将preempt设置为指示该P应该尽快进入调度程序（无论正在运行的G是什么）。
	preempt bool

	pad cpu.CacheLinePad
}

type schedt struct {
	// accessed atomically. keep at top to ensure alignment on 32-bit systems.
	// 以原子方式访问。保持顶部以确保在32位系统上对齐。
	goidgen   uint64
	lastpoll  uint64 // time of last network poll, 0 if currently polling // 上次网络轮询的时间，如果当前正在轮询，则为0
	pollUntil uint64 // time to which current poll is sleeping // 当前轮询休眠的时间

	lock mutex

	// When increasing nmidle, nmidlelocked, nmsys, or nmfreed, be
	// sure to call checkdead().
	// 当增加nmidle，nmidlelocked，nmsys或nmfreed时，请确保调用checkdead()。

	midle        muintptr // idle m's waiting for work // 空闲的m在等待工作
	nmidle       int32    // number of idle m's waiting for work // 等待工作的空闲m的数量
	nmidlelocked int32    // number of locked m's waiting for work //锁住的等待工作的m数
	mnext        int64    // number of m's that have been created and next M ID // 已创建的m个数和下一个M ID
	maxmcount    int32    // maximum number of m's allowed (or die) //允许的最大m个数（或死亡）
	nmsys        int32    // number of system m's not counted for deadlock // 不计入死锁的系统m的数量
	nmfreed      int64    // cumulative number of freed m's // 释放的m的累计数量

	ngsys uint32 // number of system goroutines; updated atomically // 系统goroutine的数量；原子更新

	pidle      puintptr // idle p's // 空闲p
	npidle     uint32
	nmspinning uint32 // See "Worker thread parking/unparking" comment in proc.go. // 请参见proc.go中的“工作者线程暂停/取消暂停”注释。

	// Global runnable queue. // 全局可运行队列。
	runq     gQueue
	runqsize int32

	// disable controls selective disabling of the scheduler.
	//
	// Use schedEnableUser to control this.
	//
	// disable is protected by sched.lock.
	//
	// disable控制有选择地禁用调度程序。
    // 使用schedEnableUser进行控制。
    // 禁用受sched.lock保护。
	disable struct {
		// user disables scheduling of user goroutines. // 用户禁用用户goroutine的调度。
		user     bool
		runnable gQueue // pending runnable Gs // 待处理的可运行Gs
		n        int32  // length of runnable // 可运行的长度
	}

	// Global cache of dead G's. //已死亡G的全局缓存。
	gFree struct {
		lock    mutex
		stack   gList // Gs with stacks // 带栈的G
		noStack gList // Gs without stacks // 不带栈的G
		n       int32
	}

	// Central cache of sudog structs. // sudog结构的中央缓存。
	sudoglock  mutex
	sudogcache *sudog

	// Central pool of available defer structs of different sizes. // 不同大小的可用延迟结构的中央池。
	deferlock mutex
	deferpool [5]*_defer

	// freem is the list of m's waiting to be freed when their
	// m.exited is set. Linked through m.freelink.
	// freem是设置了m.exited时等待释放的m列表。通过m.freelink链接。
	freem *m

	gcwaiting  uint32 // gc is waiting to run // gc正在等待运行
	stopwait   int32
	stopnote   note
	sysmonwait uint32
	sysmonnote note

	// safepointFn should be called on each P at the next GC
	// safepoint if p.runSafePointFn is set.
	// 如果设置了p.runSafePointFn，则应在下一个GC安全点的每个P上调用safepointFn。
	safePointFn   func(*p)
	safePointWait int32
	safePointNote note

	profilehz int32 // cpu profiling rate // cpu分析频率

	procresizetime int64 // nanotime() of last change to gomaxprocs // 对gomaxprocs的最后更改的nanotime()
	totaltime      int64 // ∫gomaxprocs dt up to procresizetime
}

// Values for the flags field of a sigTabT. // sigTabT的flags字段的值。
const (
	_SigNotify   = 1 << iota // let signal.Notify have signal, even if from kernel // 让signal.Notify有信号，即使来自内核
	_SigKill                 // if signal.Notify doesn't take it, exit quietly // 如果signal.Notify不接受它，请安静地退出
	_SigThrow                // if signal.Notify doesn't take it, exit loudly // 如果signal.Notify不接受，请大声退出
	_SigPanic                // if the signal is from the kernel, panic // 如果信号来自内核，则出现恐慌
	_SigDefault              // if the signal isn't explicitly requested, don't monitor it // 如果未明确请求信号，请不要监视它
	_SigGoExit               // cause all runtime procs to exit (only used on Plan 9). // 导致所有运行时proc退出（仅在plan 9中使用）。
	_SigSetStack             // add SA_ONSTACK to libc handler // 将SA_ONSTACK添加到libc处理程序
	_SigUnblock              // always unblock; see blockableSig // 始终解锁；参见blockableSig
	_SigIgn                  // _SIG_DFL action is to ignore the signal // _SIG_DFL的动作是忽略信号
)

// Layout of in-memory per-function information prepared by linker
// See https://golang.org/s/go12symtab.
// Keep in sync with linker (../cmd/link/internal/ld/pcln.go:/pclntab)
// and with package debug/gosym and with symtab.go in package runtime.
//
// 链接器准备的按功能存储的内存中的信息
// 参见https://golang.org/s/go12symtab。
// 与链接器（../cmd/link/internal/ld/pcln.go:/pclntab）和软件包debug/gosym以及软件包runtime中的symtab.go保持同步。
type _func struct {
	entry   uintptr // start pc // 启动pc指针地址
	nameoff int32   // function name // 函数名

	args        int32  // in/out args size // 入/出参大小
	deferreturn uint32 // offset of start of a deferreturn call instruction from entry, if any. // 延迟返回调用指令与条目的开始的偏移（如果有）。

	pcsp      int32
	pcfile    int32
	pcln      int32
	npcdata   int32
	funcID    funcID  // set for certain special runtime functions // 为某些特殊的运行时函数设置
	_         [2]int8 // unused
	nfuncdata uint8   // must be last // 必须是最后一个
}

// Pseudo-Func that is returned for PCs that occur in inlined code.
// A *Func can be either a *_func or a *funcinl, and they are distinguished
// by the first uintptr.
//
// 对于以内联代码出现的PC返回的伪函数。
// * Func可以是* _func或* funcinl，它们由第一个uintptr区分。
type funcinl struct {
	zero  uintptr // set to 0 to distinguish from _func // 设置为0以区别_func
	entry uintptr // entry of the real (the "outermost") frame. // 真实（“最外”）帧的入口。
	name  string
	file  string
	line  int
}

// layout of Itab known to compilers
// allocated in non-garbage-collected memory
// Needs to be in sync with
// ../cmd/compile/internal/gc/reflect.go:/^func.dumptabs.
//
// 编译器已知的Itab布局
// 分配在非垃圾收集的内存中
// 需要与../cmd/compile/internal/gc/reflect.go:/^func.dumptabs同步。
type itab struct {
	inter *interfacetype
	_type *_type
	hash  uint32 // copy of _type.hash. Used for type switches. // _type.hash的副本。用于类型开关。
	_     [4]byte
	fun   [1]uintptr // variable sized. fun[0]==0 means _type does not implement inter. // 可变大小。 fun [0] == 0表示_type未实现inter。
}

// Lock-free stack node.
// Also known to export_test.go.
// 无锁堆栈节点。
// 也以export_test.go着称。
type lfnode struct {
	next    uint64
	pushcnt uintptr
}

type forcegcstate struct {
	lock mutex
	g    *g
	idle uint32
}

// startup_random_data holds random bytes initialized at startup. These come from
// the ELF AT_RANDOM auxiliary vector (vdso_linux_amd64.go or os_linux_386.go).
// startup_random_data保存启动时初始化的随机字节。这些来自ELF AT_RANDOM辅助向量（vdso_linux_amd64.go或os_linux_386.go）。
var startupRandomData []byte

// extendRandom extends the random numbers in r[:n] to the whole slice r.
// Treats n<0 as n==0.
// extendRandom将r [：n]中的随机数扩展到整个切片r。将n <0视为n == 0。
func extendRandom(r []byte, n int) {
	if n < 0 {
		n = 0
	}
	for n < len(r) {
		// Extend random bits using hash function & time seed
		// 使用哈希函数和时间种子扩展随机位
		w := n
		if w > 16 {
			w = 16
		}
		h := memhash(unsafe.Pointer(&r[n-w]), uintptr(nanotime()), uintptr(w))
		for i := 0; i < sys.PtrSize && n < len(r); i++ {
			r[n] = byte(h)
			n++
			h >>= 8
		}
	}
}

// A _defer holds an entry on the list of deferred calls.
// If you add a field here, add code to clear it in freedefer and deferProcStack
// This struct must match the code in cmd/compile/internal/gc/reflect.go:deferstruct
// and cmd/compile/internal/gc/ssa.go:(*state).call.
// Some defers will be allocated on the stack and some on the heap.
// All defers are logically part of the stack, so write barriers to
// initialize them are not required. All defers must be manually scanned,
// and for heap defers, marked.
//
// _defer在延迟调用列表中保留一个条目。
// 如果您在此处添加字段，请添加代码以在freedefer和deferProcStack中将其清除
// 此结构必须与cmd/compile/internal/gc/reflect.go:deferstruct和cmd/compile/internal/gc/ssa.go:(*state).call中的代码匹配。
// 一些延迟将分配在堆栈上，而一些延迟将分配在堆上。
// 从逻辑上讲，所有延迟器都是堆栈的一部分，因此不需要编写屏障来初始化它们。必须手动扫描所有延迟，并标记堆延迟。
type _defer struct {
	siz     int32 // includes both arguments and results // 同时包含参数和结果
	started bool
	heap    bool
	// openDefer indicates that this _defer is for a frame with open-coded
	// defers. We have only one defer record for the entire frame (which may
	// currently have 0, 1, or more defers active).
	// openDefer表示此_defer用于具有开放编码延迟的帧。整个帧只有一个延迟记录（当前可能有0个，1个或多个激活的延迟）。
	openDefer bool
	sp        uintptr  // sp at time of defer // 延迟时的sp
	pc        uintptr  // pc at time of defer // 延迟时的pc
	fn        *funcval // can be nil for open-coded defers // 对于开放编码的延迟，可以为nil
	_panic    *_panic  // panic that is running defer // 正在运行延迟的恐慌
	link      *_defer

	// If openDefer is true, the fields below record values about the stack
	// frame and associated function that has the open-coded defer(s). sp
	// above will be the sp for the frame, and pc will be address of the
	// deferreturn call in the function.
	//
	// 如果openDefer为true，则下面的字段将记录有关具有开放编码延迟的堆栈框架和相关函数的值。
	// 上面的sp是该帧的sp，而pc是该函数中deferreturn调用的地址。
	fd   unsafe.Pointer // funcdata for the function associated with the frame // 与框架关联的函数的funcdata
	varp uintptr        // value of varp for the stack frame // 堆栈框架的varp值
	// framepc is the current pc associated with the stack frame. Together,
	// with sp above (which is the sp associated with the stack frame),
	// framepc/sp can be used as pc/sp pair to continue a stack trace via
	// gentraceback().
	//
	// framepc是与堆栈框架关联的当前pc。连同上面的sp（与堆栈框架关联的sp）一起，framepc/sp可以用作pc/sp对，
	// 以通过gentraceback()继续堆栈跟踪。
	framepc uintptr
}

// A _panic holds information about an active panic.
//
// This is marked go:notinheap because _panic values must only ever
// live on the stack.
//
// The argp and link fields are stack pointers, but don't need special
// handling during stack growth: because they are pointer-typed and
// _panic values only live on the stack, regular stack pointer
// adjustment takes care of them.
//
//go:notinheap
type _panic struct {
	argp      unsafe.Pointer // pointer to arguments of deferred call run during panic; cannot move - known to liblink
	arg       interface{}    // argument to panic
	link      *_panic        // link to earlier panic
	pc        uintptr        // where to return to in runtime if this panic is bypassed
	sp        unsafe.Pointer // where to return to in runtime if this panic is bypassed
	recovered bool           // whether this panic is over
	aborted   bool           // the panic was aborted
	goexit    bool
}

// stack traces
type stkframe struct {
	fn       funcInfo   // function being run
	pc       uintptr    // program counter within fn
	continpc uintptr    // program counter where execution can continue, or 0 if not
	lr       uintptr    // program counter at caller aka link register
	sp       uintptr    // stack pointer at pc
	fp       uintptr    // stack pointer at caller aka frame pointer
	varp     uintptr    // top of local variables
	argp     uintptr    // pointer to function arguments
	arglen   uintptr    // number of bytes at argp
	argmap   *bitvector // force use of this argmap
}

// ancestorInfo records details of where a goroutine was started.
type ancestorInfo struct {
	pcs  []uintptr // pcs from the stack of this goroutine
	goid int64     // goroutine id of this goroutine; original goroutine possibly dead
	gopc uintptr   // pc of go statement that created this goroutine
}

const (
	_TraceRuntimeFrames = 1 << iota // include frames for internal runtime functions.
	_TraceTrap                      // the initial PC, SP are from a trap, not a return PC from a call
	_TraceJumpStack                 // if traceback is on a systemstack, resume trace at g that called into it
)

// The maximum number of frames we print for a traceback
const _TracebackMaxFrames = 100

// A waitReason explains why a goroutine has been stopped.
// See gopark. Do not re-use waitReasons, add new ones.
type waitReason uint8

const (
	waitReasonZero                  waitReason = iota // ""
	waitReasonGCAssistMarking                         // "GC assist marking"
	waitReasonIOWait                                  // "IO wait"
	waitReasonChanReceiveNilChan                      // "chan receive (nil chan)"
	waitReasonChanSendNilChan                         // "chan send (nil chan)"
	waitReasonDumpingHeap                             // "dumping heap"
	waitReasonGarbageCollection                       // "garbage collection"
	waitReasonGarbageCollectionScan                   // "garbage collection scan"
	waitReasonPanicWait                               // "panicwait"
	waitReasonSelect                                  // "select"
	waitReasonSelectNoCases                           // "select (no cases)"
	waitReasonGCAssistWait                            // "GC assist wait"
	waitReasonGCSweepWait                             // "GC sweep wait"
	waitReasonGCScavengeWait                          // "GC scavenge wait"
	waitReasonChanReceive                             // "chan receive"
	waitReasonChanSend                                // "chan send"
	waitReasonFinalizerWait                           // "finalizer wait"
	waitReasonForceGGIdle                             // "force gc (idle)"
	waitReasonSemacquire                              // "semacquire"
	waitReasonSleep                                   // "sleep"
	waitReasonSyncCondWait                            // "sync.Cond.Wait"
	waitReasonTimerGoroutineIdle                      // "timer goroutine (idle)"
	waitReasonTraceReaderBlocked                      // "trace reader (blocked)"
	waitReasonWaitForGCCycle                          // "wait for GC cycle"
	waitReasonGCWorkerIdle                            // "GC worker (idle)"
	waitReasonPreempted                               // "preempted"
)

var waitReasonStrings = [...]string{
	waitReasonZero:                  "",
	waitReasonGCAssistMarking:       "GC assist marking",
	waitReasonIOWait:                "IO wait",
	waitReasonChanReceiveNilChan:    "chan receive (nil chan)",
	waitReasonChanSendNilChan:       "chan send (nil chan)",
	waitReasonDumpingHeap:           "dumping heap",
	waitReasonGarbageCollection:     "garbage collection",
	waitReasonGarbageCollectionScan: "garbage collection scan",
	waitReasonPanicWait:             "panicwait",
	waitReasonSelect:                "select",
	waitReasonSelectNoCases:         "select (no cases)",
	waitReasonGCAssistWait:          "GC assist wait",
	waitReasonGCSweepWait:           "GC sweep wait",
	waitReasonGCScavengeWait:        "GC scavenge wait",
	waitReasonChanReceive:           "chan receive",
	waitReasonChanSend:              "chan send",
	waitReasonFinalizerWait:         "finalizer wait",
	waitReasonForceGGIdle:           "force gc (idle)",
	waitReasonSemacquire:            "semacquire",
	waitReasonSleep:                 "sleep",
	waitReasonSyncCondWait:          "sync.Cond.Wait",
	waitReasonTimerGoroutineIdle:    "timer goroutine (idle)",
	waitReasonTraceReaderBlocked:    "trace reader (blocked)",
	waitReasonWaitForGCCycle:        "wait for GC cycle",
	waitReasonGCWorkerIdle:          "GC worker (idle)",
	waitReasonPreempted:             "preempted",
}

func (w waitReason) String() string {
	if w < 0 || w >= waitReason(len(waitReasonStrings)) {
		return "unknown wait reason"
	}
	return waitReasonStrings[w]
}

var (
	allglen    uintptr
	allm       *m
	allp       []*p  // len(allp) == gomaxprocs; may change at safe points, otherwise immutable
	allpLock   mutex // Protects P-less reads of allp and all writes
	gomaxprocs int32
	ncpu       int32
	forcegc    forcegcstate
	sched      schedt
	newprocs   int32

	// Information about what cpu features are available.
	// Packages outside the runtime should not use these
	// as they are not an external api.
	// Set on startup in asm_{386,amd64}.s
	processorVersionInfo uint32
	isIntel              bool
	lfenceBeforeRdtsc    bool

	goarm                uint8 // set by cmd/link on arm systems
	framepointer_enabled bool  // set by cmd/link
)

// Set by the linker so the runtime can determine the buildmode.
var (
	islibrary bool // -buildmode=c-shared
	isarchive bool // -buildmode=c-archive
)
```