```go

package sync

import (
	"internal/race"
	"sync/atomic"
	"unsafe"
)

// A WaitGroup waits for a collection of goroutines to finish.
// The main goroutine calls Add to set the number of
// goroutines to wait for. Then each of the goroutines
// runs and calls Done when finished. At the same time,
// Wait can be used to block until all goroutines have finished.
//
// A WaitGroup must not be copied after first use.
//
// WaitGroup等待goroutine的集合完成。
// 主goroutine调用Add来设置要等待的goroutine的数量。 然后，每个goroutine都会运行并在完成后调用Done。 同时，可以使用Wait来阻塞，直到所有goroutine完成。
//
// 首次使用后不得复制WaitGroup。
type WaitGroup struct {
    // noCopy用来标记不可复制,只能用指针传递,保证全局唯一.其实即使复制了,编译,运行都没问题,只有用go vet检测时才会显示出错误
	noCopy noCopy

	// 64-bit value: high 32 bits are counter, low 32 bits are waiter count.
	// 64-bit atomic operations require 64-bit alignment, but 32-bit
	// compilers do not ensure it. So we allocate 12 bytes and then use
	// the aligned 8 bytes in them as state, and the other 4 as storage
	// for the sema.
	// 64位值：高32位为计数器，低32位为等待者计数。
    // 64位原子操作需要64位对齐，但是32位编译器不能确保对齐。 因此，我们分配了12个字节，然后将其中对齐的8个字节用作状态，其余4个用作信号量的存储。
    // state1表示一个两个计数器和一个信号量
    // 32位系统：state1[0]: 计数器，state1[1]: 等待者个数，state1[2]: 信号量
    // 64位系统：state1[1]: 计数器，state1[2]: 等待者个数，state1[0]: 信号量
	state1 [3]uint32
}

// state returns pointers to the state and sema fields stored within wg.state1.
// state返回指向存储在wg.state1中的state和sema字段的指针。
func (wg *WaitGroup) state() (statep *uint64, semap *uint32) {
	if uintptr(unsafe.Pointer(&wg.state1))%8 == 0 { // 32位系统
		return (*uint64)(unsafe.Pointer(&wg.state1)), &wg.state1[2]
	} else { // 64位系统
		return (*uint64)(unsafe.Pointer(&wg.state1[1])), &wg.state1[0]
	}
}

// Add adds delta, which may be negative, to the WaitGroup counter.
// If the counter becomes zero, all goroutines blocked on Wait are released.
// If the counter goes negative, Add panics.
//
// Note that calls with a positive delta that occur when the counter is zero
// must happen before a Wait. Calls with a negative delta, or calls with a
// positive delta that start when the counter is greater than zero, may happen
// at any time.
// Typically this means the calls to Add should execute before the statement
// creating the goroutine or other event to be waited for.
// If a WaitGroup is reused to wait for several independent sets of events,
// new Add calls must happen after all previous Wait calls have returned.
// See the WaitGroup example.
//
// Add将增量（可能为负）添加到WaitGroup计数器中。
// 如果计数器变为零，则释放等待时阻塞的所有goroutine。 如果计数器变为负数，请添加恐慌。
//
// 注意，当计数器为零时发生的增量为正的调用必须在等待之前发生。 在计数器大于零时开始的负增量调用或正增量调用可能随时发生。
// 通常，这意味着对Add的调用应在等待创建goroutine或其他事件的语句之前执行。=> 说明：必须先调用wg.add(positive)，然后才调用wg.wait()
// 如果重用WaitGroup来等待几个独立的事件集，则必须在所有先前的Wait调用返回之后再进行新的Add调用。 请参阅WaitGroup示例。
func (wg *WaitGroup) Add(delta int) {
	statep, semap := wg.state()
	if race.Enabled {
		_ = *statep // trigger nil deref early
		if delta < 0 {
			// Synchronize decrements with Wait.
			race.ReleaseMerge(unsafe.Pointer(wg))
		}
		race.Disable()
		defer race.Enable()
	}
	// 更新statep，statep将在wait和add中通过原子操作一起使用
	state := atomic.AddUint64(statep, uint64(delta)<<32)
	v := int32(state >> 32) // 低32位
	w := uint32(state) // 高32位
	if race.Enabled && delta > 0 && v == int32(delta) {
		// The first increment must be synchronized with Wait.
		// Need to model this as a read, because there can be
		// several concurrent wg.counter transitions from 0.
		race.Read(unsafe.Pointer(semap))
	}
	if v < 0 { // 计数值<0，panic
		panic("sync: negative WaitGroup counter")
	}

    // 添加与等待并发调用，报panic
	if w != 0 && delta > 0 && v == int32(delta) {
	    // wait不等于0说明已经执行了Wait，此时不容许Add
		panic("sync: WaitGroup misuse: Add called concurrently with Wait")
	}
	// 正常情况，Add会让v增加，Done会让v减少，如果没有全部Done掉，此处v总是会大于0的，直到v为0才往下走
    // 而w代表是有多少个goruntine在等待done的信号，wait中通过compareAndSwap对这个w进行加1
	if v > 0 || w == 0 {
		return
	}
	// This goroutine has set counter to 0 when waiters > 0.
	// Now there can't be concurrent mutations of state:
	// - Adds must not happen concurrently with Wait,
	// - Wait does not increment waiters if it sees counter == 0.
	// Still do a cheap sanity check to detect WaitGroup misuse.
	//
	// 当 等待者 > 0时，该goroutine将计数器设置为0。现在不能存在并发的状态突变：
    //  - 添加不得与Wait同时进行，
    //  - 如果看到counter == 0，则Wait不会增加等待者的人数。
    // 仍然进行廉价的完整性检查以检测WaitGroup的滥用。

	// 当v为0(Done掉了所有)或者w不为0(已经开始等待)才会到这里，但是在这个过程中又有一次Add，导致statep变化，panic
	if *statep != state {
		panic("sync: WaitGroup misuse: Add called concurrently with Wait")
	}
	// Reset waiters count to 0.
	// 将statep清0，在Wait中通过这个值来保护信号量发出后还对这个Waitgroup进行操作
	*statep = 0
	// 将信号量发出，触发wait结束
	for ; w != 0; w-- {
		runtime_Semrelease(semap, false, 0)
	}
}

// Done decrements the WaitGroup counter by one.
func (wg *WaitGroup) Done() {
	wg.Add(-1)
}

// Wait blocks until the WaitGroup counter is zero.
func (wg *WaitGroup) Wait() {
	statep, semap := wg.state()
	if race.Enabled {
		_ = *statep // trigger nil deref early
		race.Disable()
	}
	for {
		state := atomic.LoadUint64(statep)
		v := int32(state >> 32) // 信号量值
		w := uint32(state) // 等待者个数
		if v == 0 {
			// Counter is 0, no need to wait. // 信号量为0，无需等待
			if race.Enabled {
				race.Enable()
				race.Acquire(unsafe.Pointer(wg))
			}
			return
		}
		// Increment waiters count. // 增加等待者计数。
		if atomic.CompareAndSwapUint64(statep, state, state+1) {
			if race.Enabled && w == 0 {
				// Wait must be synchronized with the first Add.
				// Need to model this is as a write to race with the read in Add.
				// As a consequence, can do the write only for the first waiter,
				// otherwise concurrent Waits will race with each other.
				race.Write(unsafe.Pointer(semap))
			}
			// 等待信号量，目的是作为一个简单的sleep原语，以供同步使用
			runtime_Semacquire(semap)
			// 信号量来了，代表所有Add都已经Done
			if *statep != 0 {
			    // 走到这里，说明在所有Add都已经Done后，触发信号量后，又被执行了Add
				panic("sync: WaitGroup is reused before previous Wait has returned")
			}
			if race.Enabled {
				race.Enable()
				race.Acquire(unsafe.Pointer(wg))
			}
			return
		}
	}
}
```