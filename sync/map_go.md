```go

package sync

import (
	"sync/atomic"
	"unsafe"
)

// Map is like a Go map[interface{}]interface{} but is safe for concurrent use
// by multiple goroutines without additional locking or coordination.
// Loads, stores, and deletes run in amortized constant time.
//
// The Map type is specialized. Most code should use a plain Go map instead,
// with separate locking or coordination, for better type safety and to make it
// easier to maintain other invariants along with the map content.
//
// The Map type is optimized for two common use cases: (1) when the entry for a given
// key is only ever written once but read many times, as in caches that only grow,
// or (2) when multiple goroutines read, write, and overwrite entries for disjoint
// sets of keys. In these two cases, use of a Map may significantly reduce lock
// contention compared to a Go map paired with a separate Mutex or RWMutex.
//
// The zero Map is empty and ready for use. A Map must not be copied after first use.
//
// Map就像Go map[interface{}]interface{}，但是可以安全地被多个goroutine并发使用，而无需额外的锁定或协调。
// 加载，存储和删除以摊销的固定时间运行。
//
// Map类型是专用的。大多数代码应改用带有单独锁定或协调功能的普通Go映射，以提高类型安全性，
// 并使其更易于维护其他不变式以及映射内容。
//
// Map类型针对两种常见用例进行了优化：
// （1）给定键的条目仅写入一次但读取多次，例如在仅增长的高速缓存中；
// （2）当多个goroutine读取，写入时，并覆盖不相交的键集的条目。
// 在这两种情况下，与与单独的Mutex或RWMutex配对的Go映射相比，使用Map可以显着减少锁争用。
//
// zero map为空，可以使用了。首次使用后不得复制map。
type Map struct {
	mu Mutex

	// read contains the portion of the map's contents that are safe for
	// concurrent access (with or without mu held).
	//
	// The read field itself is always safe to load, but must only be stored with
	// mu held.
	//
	// Entries stored in read may be updated concurrently without mu, but updating
	// a previously-expunged entry requires that the entry be copied to the dirty
	// map and unexpunged with mu held.
	//
	// read包含映射内容中可安全进行并发访问的部分（带有或不带有mu）。
    //
    // read字段本身始终可以安全加载，但必须仅在mu保持状态下存储。
    //
    // 存储在read中的条目可以不使用mu并发地更新，但是更新以前删除的条目需要将该条目复制到脏映射中，并且在保留mu的情况下不删除它。
	read atomic.Value // readOnly // readOnly结构

	// dirty contains the portion of the map's contents that require mu to be
	// held. To ensure that the dirty map can be promoted to the read map quickly,
	// it also includes all of the non-expunged entries in the read map.
	//
	// Expunged entries are not stored in the dirty map. An expunged entry in the
	// clean map must be unexpunged and added to the dirty map before a new value
	// can be stored to it.
	//
	// If the dirty map is nil, the next write to the map will initialize it by
	// making a shallow copy of the clean map, omitting stale entries.
	//
	// dirty包含地图内容中需要保留mu的部分。为了确保可以将脏映射迅速提升到读取映射，它还包括读取映射中的所有未删除条目。
    //
    // 删除的条目不会存储在脏映射中。必须先清除干净映射中已删除的条目，然后将其添加到脏映射中，然后才能将新值存储到脏映射中。
    //
    // 如果脏映射为nil，则对映射的下一次写入将通过创建干净映射的浅表副本（忽略陈旧的条目）来初始化它。
	dirty map[interface{}]*entry

	// misses counts the number of loads since the read map was last updated that
	// needed to lock mu to determine whether the key was present.
	//
	// Once enough misses have occurred to cover the cost of copying the dirty
	// map, the dirty map will be promoted to the read map (in the unamended
	// state) and the next store to the map will make a new dirty copy.
	//
	// misses计数自上次更新读取映射以来锁定mu以确定key是否存在所需的负载数。
    //
    // 一旦发生足够的未命中以支付复制脏映射的费用，该脏映射将被提升为已读映射（处于未修改状态），并且下一个存储到该映射的存储将创建新的脏副本。
	misses int
}

// readOnly is an immutable struct stored atomically in the Map.read field.
// readOnly是一个不变的结构，原子存储在Map.read字段中。
type readOnly struct {
	m       map[interface{}]*entry
	amended bool // true if the dirty map contains some key not in m. // 如果脏映射包含不在m中的某个键，则为true。
}

// expunged is an arbitrary pointer that marks entries which have been deleted
// from the dirty map.
// expunged是一个任意指针，用于标记已从脏映射中删除的条目。
var expunged = unsafe.Pointer(new(interface{}))

// An entry is a slot in the map corresponding to a particular key.
// entry是对应于特定键的映射中的槽位。
type entry struct {
	// p points to the interface{} value stored for the entry.
	//
	// If p == nil, the entry has been deleted and m.dirty == nil.
	//
	// If p == expunged, the entry has been deleted, m.dirty != nil, and the entry
	// is missing from m.dirty.
	//
	// Otherwise, the entry is valid and recorded in m.read.m[key] and, if m.dirty
	// != nil, in m.dirty[key].
	//
	// An entry can be deleted by atomic replacement with nil: when m.dirty is
	// next created, it will atomically replace nil with expunged and leave
	// m.dirty[key] unset.
	//
	// An entry's associated value can be updated by atomic replacement, provided
	// p != expunged. If p == expunged, an entry's associated value can be updated
	// only after first setting m.dirty[key] = e so that lookups using the dirty
	// map find the entry.
	// p指向为该条目存储的interface{}值。
    //
    // 如果p == nil，则该条目已被删除，而m.dirty == nil。
    //
    // 如果p == expunged，则该条目已被删除，m.dirty！= nil，并且m.dirty中缺少该条目。
    //
    // 否则，该条目有效并记录在m.read.m [key]中；如果m.dirty != nil，则记录在m.dirty[key]中。
    //
    // 可以通过用nil进行原子替换来删除条目：下次创建m.dirty时，它将自动用expunged替换nil并使m.dirty [key]保持未设置状态。
    //
    // 如果 p != expunged，则可以通过原子替换来更新条目的关联值。如果p == expunged，则只有在首先设置m.dirty [key] = e之后才能更新条目的关联值，以便使用脏映射的查找找到该条目。
	p unsafe.Pointer // *interface{}
}

func newEntry(i interface{}) *entry {
	return &entry{p: unsafe.Pointer(&i)}
}

// Load returns the value stored in the map for a key, or nil if no
// value is present.
// The ok result indicates whether value was found in the map.
// Load返回存储在映射中的值，如果没有值，则返回nil。
// ok表明是否在映射中找到了值。
func (m *Map) Load(key interface{}) (value interface{}, ok bool) {
	read, _ := m.read.Load().(readOnly)
	e, ok := read.m[key]
	if !ok && read.amended {
		m.mu.Lock() // 加锁
		// Avoid reporting a spurious miss if m.dirty got promoted while we were
		// blocked on m.mu. (If further loads of the same key will not miss, it's
		// not worth copying the dirty map for this key.)
		// 如果在m.mu被阻塞时提升了m.dirty，请避免报告虚假的遗漏。 （如果相同key的load不会再丢失，则不值得复制该key的脏映射。）
		read, _ = m.read.Load().(readOnly)
		e, ok = read.m[key] // 再次读取
		if !ok && read.amended { // 数据不在readOnly中，m中有readOnly中没有的key
			e, ok = m.dirty[key]
			// Regardless of whether the entry was present, record a miss: this key
			// will take the slow path until the dirty map is promoted to the read
			// map.
			// 不管是否存在该条目，都记录未命中：此键将采用慢速路径，直到将脏映射提升为读取映射为止。
			m.missLocked()
		}
		m.mu.Unlock()
	}
	if !ok {
		return nil, false
	}
	return e.load()
}

func (e *entry) load() (value interface{}, ok bool) {
	p := atomic.LoadPointer(&e.p)
	if p == nil || p == expunged {
		return nil, false
	}
	return *(*interface{})(p), true
}

// Store sets the value for a key.
// Store设置键的值。
func (m *Map) Store(key, value interface{}) {
	read, _ := m.read.Load().(readOnly)
	if e, ok := read.m[key]; ok && e.tryStore(&value) { // readOnly中有值，就修改readOnly中的值
		return
	}

	m.mu.Lock()
	read, _ = m.read.Load().(readOnly)
	if e, ok := read.m[key]; ok { // readOnly中有对应的key
		if e.unexpungeLocked() {
			// The entry was previously expunged, which implies that there is a
			// non-nil dirty map and this entry is not in it.
			// 该条目先前已删除，这意味着存在一个非零的脏映射，并且该条目不在其中。
			m.dirty[key] = e
		}
		e.storeLocked(&value)
	} else if e, ok := m.dirty[key]; ok { // 脏数据中有对应的key
		e.storeLocked(&value)
	} else { // 这是一个新key
		if !read.amended { // m和readOnly的key一样
			// We're adding the first new key to the dirty map.
			// Make sure it is allocated and mark the read-only map as incomplete.
			// 我们将第一个新键添加到脏映射。确保已分配它，并将只读映射标记为不完整。
			m.dirtyLocked()
			m.read.Store(readOnly{m: read.m, amended: true})
		}
		m.dirty[key] = newEntry(value)
	}
	m.mu.Unlock()
}

// tryStore stores a value if the entry has not been expunged.
//
// If the entry is expunged, tryStore returns false and leaves the entry
// unchanged.
// 如果没有删除条目，tryStore将存储一个值。
//
// 如果删除了该条目，则tryStore返回false并使该条目保持不变。
func (e *entry) tryStore(i *interface{}) bool {
	for {
		p := atomic.LoadPointer(&e.p)
		if p == expunged {
			return false
		}
		if atomic.CompareAndSwapPointer(&e.p, p, unsafe.Pointer(i)) {
			return true
		}
	}
}

// unexpungeLocked ensures that the entry is not marked as expunged.
//
// If the entry was previously expunged, it must be added to the dirty map
// before m.mu is unlocked.
// unexpungeLocked确保该条目未标记为已删除。
//
// 如果该条目先前已删除，则必须在解锁m.mu之前将其添加到脏映射中。
func (e *entry) unexpungeLocked() (wasExpunged bool) {
	return atomic.CompareAndSwapPointer(&e.p, expunged, nil)
}

// storeLocked unconditionally stores a value to the entry.
//
// The entry must be known not to be expunged.
// storeLocked无条件地将值存储到该条目。
//
// 必须知道该条目不会被清除。
func (e *entry) storeLocked(i *interface{}) {
	atomic.StorePointer(&e.p, unsafe.Pointer(i))
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
//
// LoadOrStore返回键的现有值（如果存在）。
// 否则，它将存储并返回给定值。
// 如果已加载该值，则加载的结果为true，如果已存储，则为false。
func (m *Map) LoadOrStore(key, value interface{}) (actual interface{}, loaded bool) {
	// Avoid locking if it's a clean hit.
	read, _ := m.read.Load().(readOnly)
	if e, ok := read.m[key]; ok { // readOnly中有值
		actual, loaded, ok := e.tryLoadOrStore(value)
		if ok {
			return actual, loaded
		}
	}

	m.mu.Lock()
	read, _ = m.read.Load().(readOnly)
	if e, ok := read.m[key]; ok { // readOnly中有值
		if e.unexpungeLocked() { // 删除e，并且将e设置为脏数据
			m.dirty[key] = e
		}
		actual, loaded, _ = e.tryLoadOrStore(value)
	} else if e, ok := m.dirty[key]; ok { // 脏数据中有对应的key
		actual, loaded, _ = e.tryLoadOrStore(value)
		m.missLocked()
	} else {
		if !read.amended {
			// We're adding the first new key to the dirty map.
			// Make sure it is allocated and mark the read-only map as incomplete.
			// 我们将第一个新键添加到脏映射。确保已分配它，并将只读映射标记为不完整。
			m.dirtyLocked()
			m.read.Store(readOnly{m: read.m, amended: true})
		}
		m.dirty[key] = newEntry(value)
		actual, loaded = value, false
	}
	m.mu.Unlock()

	return actual, loaded
}

// tryLoadOrStore atomically loads or stores a value if the entry is not
// expunged.
//
// If the entry is expunged, tryLoadOrStore leaves the entry unchanged and
// returns with ok==false.
func (e *entry) tryLoadOrStore(i interface{}) (actual interface{}, loaded, ok bool) {
	p := atomic.LoadPointer(&e.p)
	if p == expunged {
		return nil, false, false
	}
	if p != nil {
		return *(*interface{})(p), true, true
	}

	// Copy the interface after the first load to make this method more amenable
	// to escape analysis: if we hit the "load" path or the entry is expunged, we
	// shouldn't bother heap-allocating.
	ic := i
	for {
		if atomic.CompareAndSwapPointer(&e.p, nil, unsafe.Pointer(&ic)) {
			return i, false, true
		}
		p = atomic.LoadPointer(&e.p)
		if p == expunged {
			return nil, false, false
		}
		if p != nil {
			return *(*interface{})(p), true, true
		}
	}
}

// Delete deletes the value for a key.
func (m *Map) Delete(key interface{}) {
	read, _ := m.read.Load().(readOnly)
	e, ok := read.m[key]
	if !ok && read.amended {
		m.mu.Lock()
		read, _ = m.read.Load().(readOnly)
		e, ok = read.m[key]
		if !ok && read.amended {
			delete(m.dirty, key)
		}
		m.mu.Unlock()
	}
	if ok {
		e.delete()
	}
}

func (e *entry) delete() (hadValue bool) {
	for {
		p := atomic.LoadPointer(&e.p)
		if p == nil || p == expunged {
			return false
		}
		if atomic.CompareAndSwapPointer(&e.p, p, nil) {
			return true
		}
	}
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
//
// Range does not necessarily correspond to any consistent snapshot of the Map's
// contents: no key will be visited more than once, but if the value for any key
// is stored or deleted concurrently, Range may reflect any mapping for that key
// from any point during the Range call.
//
// Range may be O(N) with the number of elements in the map even if f returns
// false after a constant number of calls.
func (m *Map) Range(f func(key, value interface{}) bool) {
	// We need to be able to iterate over all of the keys that were already
	// present at the start of the call to Range.
	// If read.amended is false, then read.m satisfies that property without
	// requiring us to hold m.mu for a long time.
	read, _ := m.read.Load().(readOnly)
	if read.amended {
		// m.dirty contains keys not in read.m. Fortunately, Range is already O(N)
		// (assuming the caller does not break out early), so a call to Range
		// amortizes an entire copy of the map: we can promote the dirty copy
		// immediately!
		m.mu.Lock()
		read, _ = m.read.Load().(readOnly)
		if read.amended {
			read = readOnly{m: m.dirty}
			m.read.Store(read)
			m.dirty = nil
			m.misses = 0
		}
		m.mu.Unlock()
	}

	for k, e := range read.m {
		v, ok := e.load()
		if !ok {
			continue
		}
		if !f(k, v) {
			break
		}
	}
}

func (m *Map) missLocked() {
	m.misses++
	if m.misses < len(m.dirty) {
		return
	}
	m.read.Store(readOnly{m: m.dirty})
	m.dirty = nil
	m.misses = 0
}

func (m *Map) dirtyLocked() {
	if m.dirty != nil {
		return
	}

	read, _ := m.read.Load().(readOnly)
	m.dirty = make(map[interface{}]*entry, len(read.m))
	for k, e := range read.m {
		if !e.tryExpungeLocked() {
			m.dirty[k] = e
		}
	}
}

func (e *entry) tryExpungeLocked() (isExpunged bool) {
	p := atomic.LoadPointer(&e.p)
	for p == nil {
		if atomic.CompareAndSwapPointer(&e.p, nil, expunged) {
			return true
		}
		p = atomic.LoadPointer(&e.p)
	}
	return p == expunged
}
```