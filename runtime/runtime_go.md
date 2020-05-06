```go

package runtime

import (
	"runtime/internal/atomic"
	_ "unsafe" // for go:linkname
)

//go:generate go run wincallback.go
//go:generate go run mkduff.go
//go:generate go run mkfastlog2table.go

var ticks struct {
	lock mutex
	pad  uint32 // ensure 8-byte alignment of val on 386 // 确保386上val的8字节对齐
	val  uint64
}

// Note: Called by runtime/pprof in addition to runtime code.
// 注意：除了运行时代码外，还由runtime/pprof调用。
// 求每秒的频率
func tickspersecond() int64 {
	r := int64(atomic.Load64(&ticks.val))
	if r != 0 {
		return r
	}
	lock(&ticks.lock)
	r = int64(ticks.val)
	if r == 0 {
		t0 := nanotime()
		c0 := cputicks()
		usleep(100 * 1000)
		t1 := nanotime()
		c1 := cputicks()
		if t1 == t0 {
			t1++
		}
		r = (c1 - c0) * 1000 * 1000 * 1000 / (t1 - t0)
		if r == 0 {
			r++
		}
		atomic.Store64(&ticks.val, uint64(r))
	}
	unlock(&ticks.lock)
	return r
}

var envs []string
var argslice []string

//go:linkname syscall_runtime_envs syscall.runtime_envs
func syscall_runtime_envs() []string { return append([]string{}, envs...) }

//go:linkname syscall_Getpagesize syscall.Getpagesize
func syscall_Getpagesize() int { return int(physPageSize) }

//go:linkname os_runtime_args os.runtime_args
func os_runtime_args() []string { return append([]string{}, argslice...) }

//go:linkname syscall_Exit syscall.Exit
//go:nosplit
func syscall_Exit(code int) {
	exit(int32(code))
}
```