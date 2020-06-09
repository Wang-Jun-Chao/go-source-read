```go

package sync

import (
	"sync/atomic"
	"unsafe"
)

// Cond implements a condition variable, a rendezvous point
// for goroutines waiting for or announcing the occurrence
// of an event.
//
// Each Cond has an associated Locker L (often a *Mutex or *RWMutex),
// which must be held when changing the condition and
// when calling the Wait method.
//
// A Cond must not be copied after first use.
//
// Cond实现了一个条件变量，它是goroutine等待或宣布事件发生的集合点。
//
// 每个Cond都有一个关联的Locker L（通常是* Mutex或* RWMutex），在更改条件和调用Wait方法时必须将其保留。
//
// 第一次使用后，不得复制条件。
type Cond struct {
	noCopy noCopy

	// L is held while observing or changing the condition
	// 观察或更改条件时持有L
	L Locker

	notify  notifyList
	checker copyChecker
}

// NewCond returns a new Cond with Locker l.
// NewCond返回带有Locker l的新Cond。
func NewCond(l Locker) *Cond {
	return &Cond{L: l}
}

// Wait atomically unlocks c.L and suspends execution
// of the calling goroutine. After later resuming execution,
// Wait locks c.L before returning. Unlike in other systems,
// Wait cannot return unless awoken by Broadcast or Signal.
// Wait原子地解锁c.L并中止调用goroutine的执行。 在稍后恢复执行后，等待锁定c.L，然后再返回。
// 与其他系统不同，等待不会返回，除非被广播或信号唤醒。

//
// Because c.L is not locked when Wait first resumes, the caller
// typically cannot assume that the condition is true when
// Wait returns. Instead, the caller should Wait in a loop:
//
// 因为在等待第一次恢复时c.L未被锁定，所以调用者通常无法假定等待返回时条件为真。 而是，调用者应在循环中等待：
//
//    c.L.Lock()
//    for !condition() {
//        c.Wait()
//    }
//    ... make use of condition ...
//    c.L.Unlock()
//
func (c *Cond) Wait() {
	c.checker.check()
	t := runtime_notifyListAdd(&c.notify)
	c.L.Unlock()
	runtime_notifyListWait(&c.notify, t)
	c.L.Lock()
}

// Signal wakes one goroutine waiting on c, if there is any.
//
// It is allowed but not required for the caller to hold c.L
// during the call.
// Signal唤醒一个等待在c上的goroutine，如果有的话。
//
// 在调用过程中，允许但不要求调用者保持c.L。
func (c *Cond) Signal() {
	c.checker.check()
	runtime_notifyListNotifyOne(&c.notify)
}

// Broadcast wakes all goroutines waiting on c.
//
// It is allowed but not required for the caller to hold c.L
// during the call.
// 广播唤醒所有等待c的goroutine。
//
// 调用者在调用过程中可以保持c.L，但不是必须的。
func (c *Cond) Broadcast() {
	c.checker.check()
	runtime_notifyListNotifyAll(&c.notify)
}

// copyChecker holds back pointer to itself to detect object copying.
// copyChecker保留指向自身的指针以检测对象的复制。
type copyChecker uintptr

func (c *copyChecker) check() {
	if uintptr(*c) != uintptr(unsafe.Pointer(c)) &&
		!atomic.CompareAndSwapUintptr((*uintptr)(c), 0, uintptr(unsafe.Pointer(c))) &&
		uintptr(*c) != uintptr(unsafe.Pointer(c)) {
		panic("sync.Cond is copied")
	}
}

// noCopy may be embedded into structs which must not be copied
// after the first use.
// noCopy可以嵌入到第一次使用后不得复制的结构中。
//
// See https://golang.org/issues/8005#issuecomment-190753527
// for details.
type noCopy struct{}

// Lock is a no-op used by -copylocks checker from `go vet`.
func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
```
