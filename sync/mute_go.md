```go

// Package sync provides basic synchronization primitives such as mutual
// exclusion locks. Other than the Once and WaitGroup types, most are intended
// for use by low-level library routines. Higher-level synchronization is
// better done via channels and communication.
//
// Values containing the types defined in this package should not be copied.
// sync包提供基本的同步原语，例如互斥锁。除Once和WaitGroup类型外，大多数都供低级库例程使用。
// 更高级别的同步最好通过渠道和通信来完成。
//
// 不应复制包含此程序包中定义的类型的值。
package sync

import (
	"internal/race"
	"sync/atomic"
	"unsafe"
)

func throw(string) // provided by runtime // 由运行时提供

// A Mutex is a mutual exclusion lock.
// The zero value for a Mutex is an unlocked mutex.
//
// A Mutex must not be copied after first use.
//
// Mutex是互斥锁。Mutex的零值是未锁定的互斥锁。
//
// 首次使用后不得复制Mutex。
type Mutex struct {
	state int32     // 将一个32位整数拆分为 当前阻塞的goroutine数(29位)|饥饿状态(1位)|唤醒状态(1位)|锁状态(1位) 的形式，来简化字段设计
	sema  uint32    // 信号量
}

// A Locker represents an object that can be locked and unlocked.
// Locker表示可以锁定和解锁的对象。
type Locker interface {
	Lock()
	Unlock()
}

const (
	mutexLocked = 1 << iota // mutex is locked  // 1 0001 含义：用最后一位表示当前对象锁的状态，0-未锁住 1-已锁住
	mutexWoken                                  // 2 0010 含义：用倒数第二位表示当前对象是否被唤醒 0-唤醒 1-未唤醒
	mutexStarving                               // 4 0100 含义：用倒数第三位表示当前对象是否为饥饿模式，0为正常模式，1为饥饿模式
	mutexWaiterShift = iota                     // 3，从倒数第四位往前的bit位表示在排队等待的goroutine数

	// Mutex fairness.
	//
	// Mutex can be in 2 modes of operations: normal and starvation.
	// In normal mode waiters are queued in FIFO order, but a woken up waiter
	// does not own the mutex and competes with new arriving goroutines over
	// the ownership. New arriving goroutines have an advantage -- they are
	// already running on CPU and there can be lots of them, so a woken up
	// waiter has good chances of losing. In such case it is queued at front
	// of the wait queue. If a waiter fails to acquire the mutex for more than 1ms,
	// it switches mutex to the starvation mode.
	//
	// In starvation mode ownership of the mutex is directly handed off from
	// the unlocking goroutine to the waiter at the front of the queue.
	// New arriving goroutines don't try to acquire the mutex even if it appears
	// to be unlocked, and don't try to spin. Instead they queue themselves at
	// the tail of the wait queue.
	//
	// If a waiter receives ownership of the mutex and sees that either
	// (1) it is the last waiter in the queue, or (2) it waited for less than 1 ms,
	// it switches mutex back to normal operation mode.
	//
	// Normal mode has considerably better performance as a goroutine can acquire
	// a mutex several times in a row even if there are blocked waiters.
	// Starvation mode is important to prevent pathological cases of tail latency.
	//
	// 互斥锁公平性。
    //
    // Mutex可以处于两种操作模式：正常和饥饿。在正常模式下，等待者按FIFO顺序排队，但是唤醒的等待者不拥有互斥体，而是与新的goroutine争夺所有权。
    // 新到的goroutine具有一个优势-它们已经在CPU上运行，并且可能有很多，因此醒来的等待者很可能会丢失。在这种情况下，它在等待队列的前面排队。
    // 如果等待者获取互斥对象的时间超过1毫秒，则会将互斥对象切换到饥饿模式。
    //
    // 在饥饿模式下，互斥锁的所有权直接从解锁goroutine移交给队列前面的等待者。新到的goroutine即使看起来已解锁，也不会尝试获取该互斥锁，也不要尝试旋转。
    // 相反，他们将自己排在等待队列的末尾。
    //
    // 如果等待者获得了互斥锁的所有权，并且看到其中一个
    // （1）它是队列中的最后一个等待者，或者（2）等待少于1 ms，它将互斥锁切换回正常操作模式。
    //
    // 普通模式具有更好的性能，因为goroutine可以连续获取多次互斥锁，即使等待者阻塞也是如此。饥饿模式对于预防队列尾部goroutine一致无法获取mutex锁的问题。
	starvationThresholdNs = 1e6 // 1ms
)

// Lock locks m.
// If the lock is already in use, the calling goroutine
// blocks until the mutex is available.
// Lock锁定m。如果锁已在使用中，则调用goroutine会阻塞，直到互斥体可用为止。
func (m *Mutex) Lock() {
	// Fast path: grab unlocked mutex. // 快速路径：获取未锁定的互斥锁。
	// 如果m.state=0，说明当前的对象还没有被锁住，进行原子性赋值操作设置为mutexLocked状态，CompareAnSwapInt32返回true
    // 否则说明对象已被其他goroutine锁住，不会进行原子赋值操作设置，CompareAndSwapInt32返回false
	if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) {
		if race.Enabled {
			race.Acquire(unsafe.Pointer(m))
		}
		return
	}
	// Slow path (outlined so that the fast path can be inlined) // 慢速路径（已概述，以便可以内联快速路径）
	m.lockSlow()
}

func (m *Mutex) lockSlow() {
	var waitStartTime int64 // 开始等待时间戳
	starving := false // 饥饿模式标识
	awoke := false // 唤醒标识
	iter := 0 // 自旋次数
	old := m.state // 保存当前对象锁状态
	for { // 看到这个for{}说明使用了cas算法
		// Don't spin in starvation mode, ownership is handed off to waiters
		// so we won't be able to acquire the mutex anyway.
		// 不要在饥饿模式下旋转，所有权会移交给等待者，因此我们无论如何都无法获取互斥量。
		// 相当于xxxx...x0xx & 0101 = 01，当前对象锁被使用
		if old&(mutexLocked|mutexStarving) == mutexLocked && runtime_canSpin(iter) {
			// Active spinning makes sense.
			// Try to set mutexWoken flag to inform Unlock
			// to not wake other blocked goroutines.
			// 主动旋转很有意义。尝试设置MutexWoken标志来通知Unlock不唤醒其他被阻止的goroutine。
			// old>>mutexWaiterShift 再次确定是否被唤醒： xxxx...xx0x & 0010 = 0
			// atomic.CompareAndSwapInt32(&m.state, old, old|mutexWoken) 将对象锁改为唤醒状态：xxxx...xx0x | 0010 = xxxx...xx1x
			if !awoke && old&mutexWoken == 0 && old>>mutexWaiterShift != 0 &&
				atomic.CompareAndSwapInt32(&m.state, old, old|mutexWoken) {
				awoke = true
			}
			// 进入自旋锁后当前goroutine并不挂起，仍然在占用cpu资源，所以重试一定次数后，不会再进入自旋锁逻辑
			runtime_doSpin()
			iter++ // 自加，表示自旋次数
			old = m.state // 保存mutex对象即将被设置成的状态
			continue
		}

		// 以下代码是不使用**自旋**的情况

		new := old
		// Don't try to acquire starving mutex, new arriving goroutines must queue.
		// 不要试图获取饥饿的互斥体，新的goroutine必须排队。
		// xxxx...x0xx & 0100 = 0xxxx...x0xx
		if old&mutexStarving == 0 {
			new |= mutexLocked // xxxx...x0xx | 0001 = xxxx...x0x1，标识对象锁被锁住
		}

        // xxxx...x1x1 & (0001 | 0100) => xxxx...x1x1 & 0101 != 0;
        // 当前mutex处于饥饿模式并且锁已被占用，新加入进来的goroutine放到队列后面
		if old&(mutexLocked|mutexStarving) != 0 {
			new += 1 << mutexWaiterShift // 更新阻塞goroutine的数量,表示mutex的等待goroutine数目加1
		}
		// The current goroutine switches mutex to starvation mode.
		// But if the mutex is currently unlocked, don't do the switch.
		// Unlock expects that starving mutex has waiters, which will not
		// be true in this case.
		// 当前的goroutine将互斥锁转换为饥饿模式。但是，如果互斥锁当前没有解锁，就不要打开开关,
		// 设置mutex状态为饥饿模式。Unlock预期有饥饿的goroutine
        // old&mutexLocked != 0 => xxxx...xxx1 & 0001 != 0；锁已经被占用
		if starving && old&mutexLocked != 0 {
			new |= mutexStarving // xxxx...xxx | 0101 =>   xxxx...x1x1，标识对象锁被锁住
		}

		// goroutine已经被唤醒，因此需要在两种情况下重设标志
		if awoke {
			// The goroutine has been woken from sleep,
			// so we need to reset the flag in either case.
			// goroutine已从睡眠状态唤醒，因此无论哪种情况，我们都需要重置该标志。
			// new&mutexWoken == 0 => xxxx...xx1x & 0010 = 0,如果唤醒标志为与awoke不相协调就panic
			if new&mutexWoken == 0 {
				throw("sync: inconsistent mutex state")
			}
			// new & (^mutexWoken) => xxxx...xxxx & (^0010) => xxxx...xxxx & 1101 = xxxx...xx0x  设置唤醒状态位0,被唤醒
			new &^= mutexWoken
		}

		// 获取锁成功
		if atomic.CompareAndSwapInt32(&m.state, old, new) {
		    // xxxx...x0x0 & 0101 = 0，已经获取对象锁
			if old&(mutexLocked|mutexStarving) == 0 {
				break // locked the mutex with CAS // 用CAS锁定互斥锁
			}

			// 以下的操作都是为了判断是否从饥饿模式中恢复为正常模式，判断处于FIFO还是LIFO模式

			// If we were already waiting before, queue at the front of the queue.
			// 如果我们之前已经在等待，请在队列的最前面排队。
			queueLifo := waitStartTime != 0
			if waitStartTime == 0 {
				waitStartTime = runtime_nanotime()
			}
			runtime_SemacquireMutex(&m.sema, queueLifo, 1)
			starving = starving || runtime_nanotime()-waitStartTime > starvationThresholdNs
			old = m.state

			// xxxx...x1xx & 0100 != 0
			if old&mutexStarving != 0 {
				// If this goroutine was woken and mutex is in starvation mode,
				// ownership was handed off to us but mutex is in somewhat
				// inconsistent state: mutexLocked is not set and we are still
				// accounted as waiter. Fix that.
				// 如果此goroutine被唤醒，并且互斥锁处于饥饿模式，则所有权已移交给我们，但互斥锁处于某种不一致的状态：
				// MutexLocked未设置，我们仍被视为等待者。解决这个问题。
				// xxxx...xx11 & 0011 != 0
				if old&(mutexLocked|mutexWoken) != 0 || old>>mutexWaiterShift == 0 {
					throw("sync: inconsistent mutex state")
				}
				delta := int32(mutexLocked - 1<<mutexWaiterShift)
				if !starving || old>>mutexWaiterShift == 1 {
					// Exit starvation mode.
					// Critical to do it here and consider wait time.
					// Starvation mode is so inefficient, that two goroutines
					// can go lock-step infinitely once they switch mutex
					// to starvation mode.
					// 退出饥饿模式。在此处进行操作并考虑等待时间至关重要。
					// 饥饿模式效率低下，一旦两个goroutine将互斥锁切换到饥饿模式，它们就可以无限地进行锁步。
					delta -= mutexStarving
				}
				atomic.AddInt32(&m.state, delta)
				break
			}
			awoke = true
			iter = 0
		} else {
			old = m.state
		}
	}

	if race.Enabled {
		race.Acquire(unsafe.Pointer(m))
	}
}

// Unlock unlocks m.
// It is a run-time error if m is not locked on entry to Unlock.
//
// A locked Mutex is not associated with a particular goroutine.
// It is allowed for one goroutine to lock a Mutex and then
// arrange for another goroutine to unlock it.
//
// Unlock以解锁m。
// 如果m在进入Unlock时未锁定，则是运行时错误。
//
// 锁定的互斥锁未与特定的goroutine关联。允许一个goroutine锁定Mutex，然后安排另一个goroutine对其进行解锁。
func (m *Mutex) Unlock() {
	if race.Enabled {
		_ = m.state
		race.Release(unsafe.Pointer(m))
	}

	// Fast path: drop lock bit. // 快速路径：减少等待者。
	new := atomic.AddInt32(&m.state, -mutexLocked)
	if new != 0 {
		// Outlined slow path to allow inlining the fast path.
		// To hide unlockSlow during tracing we skip one extra frame when tracing GoUnblock.
		// 概述了慢速路径，以允许内联快速路径。
        // 要在跟踪过程中隐藏unlockSlow，我们在跟踪GoUnblock时会跳过一帧。
		m.unlockSlow(new)
	}
}

func (m *Mutex) unlockSlow(new int32) {
	if (new+mutexLocked)&mutexLocked == 0 {
		throw("sync: unlock of unlocked mutex")
	}
	if new&mutexStarving == 0 {
		old := new
		for {
			// If there are no waiters or a goroutine has already
			// been woken or grabbed the lock, no need to wake anyone.
			// In starvation mode ownership is directly handed off from unlocking
			// goroutine to the next waiter. We are not part of this chain,
			// since we did not observe mutexStarving when we unlocked the mutex above.
			// So get off the way.
			// 如果没有等待者，或者goroutine已经被唤醒或抓住了锁，则无需唤醒任何人。
			// 在饥饿模式下，所有权从解锁goroutine直接移交给下一个等待者。
			// 我们不属于此情况，因为在解锁上述互斥锁时未观察到互斥锁饥饿。所以不用处理。
			if old>>mutexWaiterShift == 0 || old&(mutexLocked|mutexWoken|mutexStarving) != 0 {
				return
			}
			// Grab the right to wake someone.
			// 获取唤醒某人的权利。
			new = (old - 1<<mutexWaiterShift) | mutexWoken
			if atomic.CompareAndSwapInt32(&m.state, old, new) {
				runtime_Semrelease(&m.sema, false, 1)
				return
			}
			old = m.state
		}
	} else {
		// Starving mode: handoff mutex ownership to the next waiter, and yield
		// our time slice so that the next waiter can start to run immediately.
		// Note: mutexLocked is not set, the waiter will set it after wakeup.
		// But mutex is still considered locked if mutexStarving is set,
		// so new coming goroutines won't acquire it.
		// 饥饿模式：将互斥量所有权移交给下一个等待者，并产生我们的时间片，以便下一个等待者可以立即开始运行。
        // 注意：未设置MutexLocked，唤醒后等待者将对其进行设置。但是如果设置了mutexStarving，互斥锁仍被认为是锁定的，因此新的goroutines不会获取它。
		runtime_Semrelease(&m.sema, true, 1)
	}
}
```