```go

package sync

import (
	"sync/atomic"
)

// Once is an object that will perform exactly one action.
// Once是一个对象，它将完全执行一次动作。
type Once struct {
	// done indicates whether the action has been performed.
	// It is first in the struct because it is used in the hot path.
	// The hot path is inlined at every call site.
	// Placing done first allows more compact instructions on some architectures (amd64/x86),
	// and fewer instructions (to calculate offset) on other architectures.
	// done表示是否已执行操作。它是结构中的第一个，因为它在热路径中使用。每个调用点都会内联热路径。
    // 将done放置在第一个在某些体系结构（amd64 / x86）上可以使用更紧凑的指令，而在其他体系结构上可以使用更少的指令（用于计算偏移量）。
	done uint32
	m    Mutex
}

// Do calls the function f if and only if Do is being called for the
// first time for this instance of Once. In other words, given
// 	var once Once
// if once.Do(f) is called multiple times, only the first call will invoke f,
// even if f has a different value in each invocation. A new instance of
// Once is required for each function to execute.
//
// Do is intended for initialization that must be run exactly once. Since f
// is niladic, it may be necessary to use a function literal to capture the
// arguments to a function to be invoked by Do:
// 	config.once.Do(func() { config.init(filename) })
//
// Because no call to Do returns until the one call to f returns, if f causes
// Do to be called, it will deadlock.
//
// If f panics, Do considers it to have returned; future calls of Do return
// without calling f.
//
// 当且仅当针对Once的此实例第一次调用Do时，Do才调用函数f。换句话说，给定
//  var once Once
// 如果多次调用once.Do(f)，即使f在每次调用中具有不同的值，也只有第一个调用会调用f。要执行每个功能，都需要一个新的Once实例。
//
// Do用于初始化，必须只运行一次。由于f是iniladic，因此可能有必要使用函数文字来捕获由Do调用的函数的参数：
// 	config.once.Do(func() { config.init(filename) })
//
// 因为对Do的调用直到返回对f的一次调用才返回，所以如果f导致调用Do，它将死锁。
//
// 如果出现panic情况，Do认为它已经返回； Do的未来调用将不调用而返回f。
func (o *Once) Do(f func()) {
	// Note: Here is an incorrect implementation of Do:
	//
	//	if atomic.CompareAndSwapUint32(&o.done, 0, 1) {
	//		f()
	//	}
	//
	// Do guarantees that when it returns, f has finished.
	// This implementation would not implement that guarantee:
	// given two simultaneous calls, the winner of the cas would
	// call f, and the second would return immediately, without
	// waiting for the first's call to f to complete.
	// This is why the slow path falls back to a mutex, and why
	// the atomic.StoreUint32 must be delayed until after f returns.
	//
	//  注意：这是Do的错误实现：
    //
	//	if atomic.CompareAndSwapUint32(&o.done, 0, 1) {
	//		f()
	//	}
    //
    // 确保返回时f已完成。
    // 此实现不会实现该保证：给定两个同时进行的调用，cas的获胜者将调用f，第二个将立即返回，而无需等待第一个对f的调用完成。
    // 这就是为什么慢速路径退回到互斥锁的原因，以及为什么atomic.StoreUint32必须延迟到f返回之后的原因。

	if atomic.LoadUint32(&o.done) == 0 {
		// Outlined slow-path to allow inlining of the fast-path.
		// 概述慢速路径，以允许快速路径的内联。
		o.doSlow(f)
	}
}

func (o *Once) doSlow(f func()) {
	o.m.Lock()
	defer o.m.Unlock()
	if o.done == 0 {
		defer atomic.StoreUint32(&o.done, 1)
		f()
	}
}
```