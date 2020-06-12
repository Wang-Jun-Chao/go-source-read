```go

package sync

import (
	"internal/race"
	"sync/atomic"
	"unsafe"
)

// There is a modified copy of this file in runtime/rwmutex.go.
// If you make any changes here, see if you should make them there.

// A RWMutex is a reader/writer mutual exclusion lock.
// The lock can be held by an arbitrary number of readers or a single writer.
// The zero value for a RWMutex is an unlocked mutex.
//
// A RWMutex must not be copied after first use.
//
// If a goroutine holds a RWMutex for reading and another goroutine might
// call Lock, no goroutine should expect to be able to acquire a read lock
// until the initial read lock is released. In particular, this prohibits
// recursive read locking. This is to ensure that the lock eventually becomes
// available; a blocked Lock call excludes new readers from acquiring the
// lock.
//
// 在runtime/rwmutex.go中有此文件的修改后的副本。如果您在此处进行任何更改，请查看是否应在那里进行更改。
//
// RWMutex是读/写互斥锁。
// 锁可以由任意数量的读者或单个写者持有。
// RWMutex的零值是未锁定的互斥量。
//
// 首次使用后不得复制RWMutex。
//
// 如果一个goroutine拥有RWMutex进行读取，而另一个goroutine可能会调用Lock，则在释放初始读取锁之前，任何goroutine都不应期望能够获取读取锁。
// 特别是，这禁止了递归读取锁定。这是为了确保锁最终可用。封锁的锁定调用者会阻止新读者获取
type RWMutex struct {
	w           Mutex  // held if there are pending writers // 如果有等待的写者，则持有该锁
	writerSem   uint32 // semaphore for writers to wait for completing readers // 信号量，供写者等待读者完成
	readerSem   uint32 // semaphore for readers to wait for completing writers // 信号量，供读者等待写者完成
	readerCount int32  // number of pending readers // 未完成的读者数
	readerWait  int32  // number of departing readers // 离开的读者数
}

const rwmutexMaxReaders = 1 << 30 // 读写锁允许的啊大读者数目

// RLock locks rw for reading.
//
// It should not be used for recursive read locking; a blocked Lock
// call excludes new readers from acquiring the lock. See the
// documentation on the RWMutex type.
//
// RLock锁定rw以进行读取。
//
// 不应将其用于递归读取锁定；对被锁定的锁调用将使新读者无法获得锁定。请参阅有关RWMutex类型的文档。
func (rw *RWMutex) RLock() {
	if race.Enabled {
		_ = rw.w.state
		race.Disable()
	}
	if atomic.AddInt32(&rw.readerCount, 1) < 0 {
		// A writer is pending, wait for it. // 写者正在等待，等待写者完成。
		runtime_SemacquireMutex(&rw.readerSem, false, 0)
	}
	if race.Enabled {
		race.Enable()
		race.Acquire(unsafe.Pointer(&rw.readerSem))
	}
}

// RUnlock undoes a single RLock call;
// it does not affect other simultaneous readers.
// It is a run-time error if rw is not locked for reading
// on entry to RUnlock.
// RUnlock撤消单个RLock调用；
// 它不会影响其他同时读者。
// 如果在读入RUnlock时未将rw锁定以进行读取，则这是运行时错误。
func (rw *RWMutex) RUnlock() {
	if race.Enabled {
		_ = rw.w.state
		race.ReleaseMerge(unsafe.Pointer(&rw.writerSem))
		race.Disable()
	}
	if r := atomic.AddInt32(&rw.readerCount, -1); r < 0 {
		// Outlined slow-path to allow the fast-path to be inlined
		// 圈定慢速路径以允许快速路径内联
		rw.rUnlockSlow(r)
	}
	if race.Enabled {
		race.Enable()
	}
}

func (rw *RWMutex) rUnlockSlow(r int32) {
	if r+1 == 0 || r+1 == -rwmutexMaxReaders {
		race.Enable()
		throw("sync: RUnlock of unlocked RWMutex")
	}
	// A writer is pending. // 写者正在等待。
	if atomic.AddInt32(&rw.readerWait, -1) == 0 {
		// The last reader unblocks the writer. // 最后一个读者解除对写者的封锁。
		runtime_Semrelease(&rw.writerSem, false, 1)
	}
}

// Lock locks rw for writing.
// If the lock is already locked for reading or writing,
// Lock blocks until the lock is available.
// Lock锁定rw进行写入。如果该锁已被锁定以进行读取或写入，则Lock会阻塞直到该锁可用。
func (rw *RWMutex) Lock() {
	if race.Enabled {
		_ = rw.w.state
		race.Disable()
	}
	// First, resolve competition with other writers. // 首先，解决与其他写者的竞争。
	rw.w.Lock()
	// Announce to readers there is a pending writer. // 向读者宣布有一位等待的写者。
	r := atomic.AddInt32(&rw.readerCount, -rwmutexMaxReaders) + rwmutexMaxReaders
	// Wait for active readers. // 等待活跃的读者。
	if r != 0 && atomic.AddInt32(&rw.readerWait, r) != 0 {
		runtime_SemacquireMutex(&rw.writerSem, false, 0)
	}
	if race.Enabled {
		race.Enable()
		race.Acquire(unsafe.Pointer(&rw.readerSem))
		race.Acquire(unsafe.Pointer(&rw.writerSem))
	}
}

// Unlock unlocks rw for writing. It is a run-time error if rw is
// not locked for writing on entry to Unlock.
//
// As with Mutexes, a locked RWMutex is not associated with a particular
// goroutine. One goroutine may RLock (Lock) a RWMutex and then
// arrange for another goroutine to RUnlock (Unlock) it.
//
// Unlock为写入解锁rw。如果rw未被锁定，则会导致运行时错误。
//
// 与互斥锁一样，锁定的RWMutex与特定的goroutine没有关联。一个goroutine可以RLock（锁定）一个RWMutex，然后安排另一个goroutine RUnlock（解锁）。
func (rw *RWMutex) Unlock() {
	if race.Enabled {
		_ = rw.w.state
		race.Release(unsafe.Pointer(&rw.readerSem))
		race.Disable()
	}

	// Announce to readers there is no active writer. // 向读者宣布没有活跃的写者。
	r := atomic.AddInt32(&rw.readerCount, rwmutexMaxReaders)
	if r >= rwmutexMaxReaders {
		race.Enable()
		throw("sync: Unlock of unlocked RWMutex")
	}
	// Unblock blocked readers, if any. // 取消阻止被阻止的读者（如果有）。
	for i := 0; i < int(r); i++ {
		runtime_Semrelease(&rw.readerSem, false, 0)
	}
	// Allow other writers to proceed. // 允许其他写者继续。
	rw.w.Unlock()
	if race.Enabled {
		race.Enable()
	}
}

// RLocker returns a Locker interface that implements
// the Lock and Unlock methods by calling rw.RLock and rw.RUnlock.
// RLocker返回一个Locker接口，该接口通过调用rw.RLock和rw.RUnlock来实现Lock和Unlock方法。
func (rw *RWMutex) RLocker() Locker {
	return (*rlocker)(rw)
}

type rlocker RWMutex

func (r *rlocker) Lock()   { (*RWMutex)(r).RLock() }
func (r *rlocker) Unlock() { (*RWMutex)(r).RUnlock() }
```