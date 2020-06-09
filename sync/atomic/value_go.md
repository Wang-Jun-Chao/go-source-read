```go

package atomic

import (
	"unsafe"
)

// A Value provides an atomic load and store of a consistently typed value.
// The zero value for a Value returns nil from Load.
// Once Store has been called, a Value must not be copied.
//
// A Value must not be copied after first use.
// Value提供原子加载并存储一致类型的值。 Value的零值从Load返回零。 调用存储后，不得复制Value。 首次使用后不得复制Value。
type Value struct {
	v interface{}
}

// ifaceWords is interface{} internal representation.
// ifaceWords是interface{}内部表示。
type ifaceWords struct {
	typ  unsafe.Pointer
	data unsafe.Pointer
}

// Load returns the value set by the most recent Store.
// It returns nil if there has been no call to Store for this Value.
// Load返回最新Store设置的值。 如果没有针对此值的Store调用，则返回nil。
func (v *Value) Load() (x interface{}) {
	vp := (*ifaceWords)(unsafe.Pointer(v))
	typ := LoadPointer(&vp.typ)
	if typ == nil || uintptr(typ) == ^uintptr(0) {
		// First store not yet completed. // 首次Store尚未完成。
		return nil
	}
	data := LoadPointer(&vp.data)
	xp := (*ifaceWords)(unsafe.Pointer(&x))
	xp.typ = typ
	xp.data = data
	return
}

// Store sets the value of the Value to x.
// All calls to Store for a given Value must use values of the same concrete type.
// Store of an inconsistent type panics, as does Store(nil).
// Store将Value的值设置为x。 给定值的所有Store调用都必须使用相同具体类型的值。
// 不一致类型的存储会panic，Store（nil）也是如此。
func (v *Value) Store(x interface{}) {
	if x == nil {
		panic("sync/atomic: store of nil value into Value")
	}
	vp := (*ifaceWords)(unsafe.Pointer(v))
	xp := (*ifaceWords)(unsafe.Pointer(&x))
	for {
		typ := LoadPointer(&vp.typ)
		if typ == nil {
			// Attempt to start first store.
			// Disable preemption so that other goroutines can use
			// active spin wait to wait for completion; and so that
			// GC does not see the fake type accidentally.
			// 尝试开始首次存储。 禁用抢占，以便其他goroutine可以使用活动的spin等待来等待完成；
			// 这样GC不会偶然看到伪造的类型。
			runtime_procPin()
			if !CompareAndSwapPointer(&vp.typ, nil, unsafe.Pointer(^uintptr(0))) {
				runtime_procUnpin()
				continue
			}
			// Complete first store. // 完成首次存储
			StorePointer(&vp.data, xp.data)
			StorePointer(&vp.typ, xp.typ)
			runtime_procUnpin()
			return
		}
		if uintptr(typ) == ^uintptr(0) {
			// First store in progress. Wait.
			// Since we disable preemption around the first store,
			// we can wait with active spinning.
			// 进行中的首次Store。 等待。 由于我们在首次Store附近禁用了抢占，因此我们可以等待主动旋转。
			continue
		}
		// First store completed. Check type and overwrite data.
		// 首次Store完成。 检查类型并覆盖数据。
		if typ != xp.typ {
			panic("sync/atomic: store of inconsistently typed value into Value")
		}
		StorePointer(&vp.data, xp.data)
		return
	}
}

// Disable/enable preemption, implemented in runtime. // 禁用/启用抢占，在运行时实现。
func runtime_procPin()
func runtime_procUnpin()
```