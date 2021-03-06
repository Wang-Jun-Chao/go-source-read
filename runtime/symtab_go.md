```go

package runtime

import (
	"runtime/internal/atomic"
	"runtime/internal/sys"
	"unsafe"
)

// Frames may be used to get function/file/line information for a
// slice of PC values returned by Callers.
// Frames可用于获取调用者返回的一部分PC值的function/file/line信息。
type Frames struct {
	// callers is a slice of PCs that have not yet been expanded to frames.
	// 调用者是尚未扩展为Frames的PC的一部分。
	callers []uintptr

	// frames is a slice of Frames that have yet to be returned.
	// frames是尚未返回的Frames的一部分。
	frames     []Frame
	frameStore [2]Frame
}

// Frame is the information returned by Frames for each call frame.
// Frame是Frames为每个调用帧返回的信息。
type Frame struct {
	// PC is the program counter for the location in this frame.
	// For a frame that calls another frame, this will be the
	// program counter of a call instruction. Because of inlining,
	// multiple frames may have the same PC value, but different
	// symbolic information.
	//
	// PC是该帧中该位置的程序计数器。
    // 对于调用另一个Frame的Frame，这将是调用指令的程序计数器。由于内联，多个Frame可能具有相同的PC值，但是符号信息不同。
	PC uintptr

	// Func is the Func value of this call frame. This may be nil
	// for non-Go code or fully inlined functions.
	//
	// Func是此调用帧的Func值。对于非Go代码或完全内联的函数，这可能为零。
	Func *Func

	// Function is the package path-qualified function name of
	// this call frame. If non-empty, this string uniquely
	// identifies a single function in the program.
	// This may be the empty string if not known.
	// If Func is not nil then Function == Func.Name().
	//
	// Function是此调用框架的程序包路径限定的函数名称。如果为非空，则此字符串唯一标识程序中的单个函数。
    // 如果未知，则可能是空字符串。
    // 如果Func不为nil，则Function== Func.Name（）。
	Function string

	// File and Line are the file name and line number of the
	// location in this frame. For non-leaf frames, this will be
	// the location of a call. These may be the empty string and
	// zero, respectively, if not known.
	//
	// File 和 Line是此帧中位置的文件名和行号。对于非叶子帧，这将是调用的位置。如果未知，则分别为空字符串和零。
	File string
	Line int

	// Entry point program counter for the function; may be zero
	// if not known. If Func is not nil then Entry ==
	// Func.Entry().
	// 该功能的入口点程序计数器；如果未知，则可能为零。如果Func不为零，则Entry == Func.Entry()。
	Entry uintptr

	// The runtime's internal view of the function. This field
	// is set (funcInfo.valid() returns true) only for Go functions,
	// not for C functions.
	// 该函数的运行时内部视图。仅对于Go函数而不是C函数设置该字段（funcInfo.valid()返回true）。
	funcInfo funcInfo
}

// CallersFrames takes a slice of PC values returned by Callers and
// prepares to return function/file/line information.
// Do not change the slice until you are done with the Frames.
//
// CallersFrames会获取Callers返回的PC值的一部分，并准备返回function/file/line信息。在完成框架之前，请勿更改切片。
func CallersFrames(callers []uintptr) *Frames {
	f := &Frames{callers: callers}
	f.frames = f.frameStore[:0]
	return f
}

// Next returns frame information for the next caller.
// If more is false, there are no more callers (the Frame value is valid).
//
// Next返回下一个调用者的帧信息。如果更多为假，则没有更多的调用者（“帧”值有效）。
func (ci *Frames) Next() (frame Frame, more bool) {
	for len(ci.frames) < 2 {
		// Find the next frame.
		// We need to look for 2 frames so we know what
		// to return for the "more" result.
		// 找到下一帧。我们需要寻找2帧，以便我们知道返回“more”结果的结果。
		if len(ci.callers) == 0 {
			break
		}
		pc := ci.callers[0]
		ci.callers = ci.callers[1:]
		funcInfo := findfunc(pc)
		if !funcInfo.valid() {
			if cgoSymbolizer != nil {
				// Pre-expand cgo frames. We could do this
				// incrementally, too, but there's no way to
				// avoid allocation in this case anyway.
				// 预展开cgo框架。我们也可以增量地执行此操作，但是在这种情况下，还是无法避免分配。
				ci.frames = append(ci.frames, expandCgoFrames(pc)...)
			}
			continue
		}
		f := funcInfo._Func()
		entry := f.Entry()
		if pc > entry {
			// We store the pc of the start of the instruction following
			// the instruction in question (the call or the inline mark).
			// This is done for historical reasons, and to make FuncForPC
			// work correctly for entries in the result of runtime.Callers.
			// 我们在相关指令（调用或内联标记）之后存储开始指令的pc。
			// 这样做是出于历史原因，并且使FuncForPC对于runtime.Callers结果中的条目正确工作。
			pc--
		}
		name := funcname(funcInfo)
		if inldata := funcdata(funcInfo, _FUNCDATA_InlTree); inldata != nil {
			inltree := (*[1 << 20]inlinedCall)(inldata)
			ix := pcdatavalue(funcInfo, _PCDATA_InlTreeIndex, pc, nil)
			if ix >= 0 {
				// Note: entry is not modified. It always refers to a real frame, not an inlined one.
				// 注意：该条目未修改。它始终是指真实框架，而不是内联框架。
				f = nil
				name = funcnameFromNameoff(funcInfo, inltree[ix].func_)
				// File/line is already correct.
				// TODO: remove file/line from InlinedCall?
			}
		}
		ci.frames = append(ci.frames, Frame{
			PC:       pc,
			Func:     f,
			Function: name,
			Entry:    entry,
			funcInfo: funcInfo,
			// Note: File,Line set below
		})
	}

	// Pop one frame from the frame list. Keep the rest.
	// Avoid allocation in the common case, which is 1 or 2 frames.
	// 从帧列表中弹出一帧。保持其余。避免在通常情况下分配1或2帧。
	switch len(ci.frames) {
	case 0: // In the rare case when there are no frames at all, we return Frame{}. // 在极少数情况下根本没有框架的情况下，我们返回Frame{}。
		return
	case 1:
		frame = ci.frames[0]
		ci.frames = ci.frameStore[:0]
	case 2:
		frame = ci.frames[0]
		ci.frameStore[0] = ci.frames[1]
		ci.frames = ci.frameStore[:1]
	default:
		frame = ci.frames[0]
		ci.frames = ci.frames[1:]
	}
	more = len(ci.frames) > 0
	if frame.funcInfo.valid() {
		// Compute file/line just before we need to return it,
		// as it can be expensive. This avoids computing file/line
		// for the Frame we find but don't return. See issue 32093.
		//
		// 在需要返回file/line之前计算file/line，因为它可能很昂贵。
		// 这样可以避免为我们找到但不返回计算的帧file/line。请参阅问题32093。
		file, line := funcline1(frame.funcInfo, frame.PC, false)
		frame.File, frame.Line = file, int(line)
	}
	return
}

// runtime_expandFinalInlineFrame expands the final pc in stk to include all
// "callers" if pc is inline.
// runtime_expandFinalInlineFrame如果pc是内联的，则将stk中的最终pc扩展为包括所有“调用方”。
//
//go:linkname runtime_expandFinalInlineFrame runtime/pprof.runtime_expandFinalInlineFrame
func runtime_expandFinalInlineFrame(stk []uintptr) []uintptr {
	if len(stk) == 0 {
		return stk
	}
	pc := stk[len(stk)-1]
	tracepc := pc - 1

	f := findfunc(tracepc)
	if !f.valid() {
		// Not a Go function.
		return stk
	}

	inldata := funcdata(f, _FUNCDATA_InlTree)
	if inldata == nil {
		// Nothing inline in f.
		return stk
	}

	// Treat the previous func as normal. We haven't actually checked, but
	// since this pc was included in the stack, we know it shouldn't be
	// elided.
	// 将以前的func视为正常。我们实际上并未进行检查，但是由于此PC已包含在堆栈中，因此我们知道不应删除它。
	lastFuncID := funcID_normal

	// Remove pc from stk; we'll re-add it below.
	// 从stk删除pc；我们将在下面重新添加。
	stk = stk[:len(stk)-1]

	// See inline expansion in gentraceback.
	// 请参阅gentraceback中的内联扩展。
	var cache pcvalueCache
	inltree := (*[1 << 20]inlinedCall)(inldata)
	for {
		ix := pcdatavalue(f, _PCDATA_InlTreeIndex, tracepc, &cache)
		if ix < 0 {
			break
		}
		if inltree[ix].funcID == funcID_wrapper && elideWrapperCalling(lastFuncID) {
			// ignore wrappers
		} else {
			stk = append(stk, pc)
		}
		lastFuncID = inltree[ix].funcID
		// Back up to an instruction in the "caller".
		// 备份到“调用方”中的指令。
		tracepc = f.entry + uintptr(inltree[ix].parentPc)
		pc = tracepc + 1
	}

	// N.B. we want to keep the last parentPC which is not inline.
	// N.B.我们要保留最后一个非内联的parentPC。
	stk = append(stk, pc)

	return stk
}

// expandCgoFrames expands frame information for pc, known to be
// a non-Go function, using the cgoSymbolizer hook. expandCgoFrames
// returns nil if pc could not be expanded.
// expandCgoFrames使用cgoSymbolizer钩子扩展pc的帧信息，这是一个非Go函数。如果无法扩展pc，expandCgoFrames返回nil。
func expandCgoFrames(pc uintptr) []Frame {
	arg := cgoSymbolizerArg{pc: pc}
	callCgoSymbolizer(&arg)

	if arg.file == nil && arg.funcName == nil {
		// No useful information from symbolizer.
		// 没有来自符号生成器的有用信息。
		return nil
	}

	var frames []Frame
	for {
		frames = append(frames, Frame{
			PC:       pc,
			Func:     nil,
			Function: gostring(arg.funcName),
			File:     gostring(arg.file),
			Line:     int(arg.lineno),
			Entry:    arg.entry,
			// funcInfo is zero, which implies !funcInfo.valid().
			// That ensures that we use the File/Line info given here.
			// funcInfo为零，表示!funcInfo.valid()。这样可以确保我们使用此处提供的文件/行信息。
		})
		if arg.more == 0 {
			break
		}
		callCgoSymbolizer(&arg)
	}

	// No more frames for this PC. Tell the symbolizer we are done.
	// We don't try to maintain a single cgoSymbolizerArg for the
	// whole use of Frames, because there would be no good way to tell
	// the symbolizer when we are done.
	// 此PC没有更多帧。告诉符号化器我们已经完成。我们不会在整个框架的使用中都尝试维护一个cgoSymbolizerizerArg，
	// 因为没有什么好方法可以告诉符号化器何时完成。
	arg.pc = 0
	callCgoSymbolizer(&arg)

	return frames
}

// NOTE: Func does not expose the actual unexported fields, because we return *Func
// values to users, and we want to keep them from being able to overwrite the data
// with (say) *f = Func{}.
// All code operating on a *Func must call raw() to get the *_func
// or funcInfo() to get the funcInfo instead.
//
// 注意：Func不会公开实际的未导出字段，因为我们将*Func值返回给用户，并且我们希望防止它们能够用*f = Func{}覆盖数据。
// 在*Func上运行的所有代码都必须调用raw()来获取* _func或funcInfo()来获取funcInfo。

// A Func represents a Go function in the running binary.
// Func代表正在运行的二进制文件中的Go函数。
type Func struct {
	opaque struct{} // unexported field to disallow conversions // 未导出的字段禁止转换
}

func (f *Func) raw() *_func {
	return (*_func)(unsafe.Pointer(f))
}

func (f *Func) funcInfo() funcInfo {
	fn := f.raw()
	return funcInfo{fn, findmoduledatap(fn.entry)}
}

// PCDATA and FUNCDATA table indexes.
// PCDATA和FUNCDATA表索引。
//
// See funcdata.h and ../cmd/internal/objabi/funcdata.go.
const (
	_PCDATA_RegMapIndex   = 0
	_PCDATA_StackMapIndex = 1
	_PCDATA_InlTreeIndex  = 2

	_FUNCDATA_ArgsPointerMaps    = 0
	_FUNCDATA_LocalsPointerMaps  = 1
	_FUNCDATA_RegPointerMaps     = 2
	_FUNCDATA_StackObjects       = 3
	_FUNCDATA_InlTree            = 4
	_FUNCDATA_OpenCodedDeferInfo = 5

	_ArgsSizeUnknown = -0x80000000
)

// A FuncID identifies particular functions that need to be treated
// specially by the runtime.
// Note that in some situations involving plugins, there may be multiple
// copies of a particular special runtime function.
// Note: this list must match the list in cmd/internal/objabi/funcid.go.
//
// FuncID标识运行时需要特殊处理的特定功能。
// 注意，在某些涉及插件的情况下，特定特殊运行时函数可能有多个副本。
// 注意：此列表必须与cmd/internal/objabi/funcid.go中的列表匹配。
type funcID uint8

const (
	funcID_normal funcID = iota // not a special function // 不是特殊功能
	funcID_runtime_main
	funcID_goexit
	funcID_jmpdefer
	funcID_mcall
	funcID_morestack
	funcID_mstart
	funcID_rt0_go
	funcID_asmcgocall
	funcID_sigpanic
	funcID_runfinq
	funcID_gcBgMarkWorker
	funcID_systemstack_switch
	funcID_systemstack
	funcID_cgocallback_gofunc
	funcID_gogo
	funcID_externalthreadhandler
	funcID_debugCallV1
	funcID_gopanic
	funcID_panicwrap
	funcID_handleAsyncEvent
	funcID_asyncPreempt
	funcID_wrapper // any autogenerated code (hash/eq algorithms, method wrappers, etc.) // 任何自动生成的代码（哈希/eq算法，方法包装器等）
)

// moduledata records information about the layout of the executable
// image. It is written by the linker. Any changes here must be
// matched changes to the code in cmd/internal/ld/symtab.go:symtab.
// moduledata is stored in statically allocated non-pointer memory;
// none of the pointers here are visible to the garbage collector.
//
// moduledata记录有关可执行映像布局的信息。它由链接器编写。此处的任何更改都必须与cmd/internal/ld/symtab.go:symtab中的代码更改相匹配。
// 模块数据存储在静态分配的非指针内存中；垃圾收集器看不到这里的所有指针。
type moduledata struct {
	pclntable    []byte
	ftab         []functab
	filetab      []uint32
	findfunctab  uintptr
	minpc, maxpc uintptr

	text, etext           uintptr
	noptrdata, enoptrdata uintptr
	data, edata           uintptr
	bss, ebss             uintptr
	noptrbss, enoptrbss   uintptr
	end, gcdata, gcbss    uintptr
	types, etypes         uintptr

	textsectmap []textsect
	typelinks   []int32 // offsets from types
	itablinks   []*itab

	ptab []ptabEntry

	pluginpath string
	pkghashes  []modulehash

	modulename   string
	modulehashes []modulehash

	hasmain uint8 // 1 if module contains the main function, 0 otherwise // 如果模块包含main函数功能，则为1，否则为0

	gcdatamask, gcbssmask bitvector

	typemap map[typeOff]*_type // offset to *_rtype in previous module // 上一个模块中的* _rtype偏移量

	bad bool // module failed to load and should be ignored // 模块加载失败，应忽略

	next *moduledata
}

// A modulehash is used to compare the ABI of a new module or a
// package in a new module with the loaded program.
//
// For each shared library a module links against, the linker creates an entry in the
// moduledata.modulehashes slice containing the name of the module, the abi hash seen
// at link time and a pointer to the runtime abi hash. These are checked in
// moduledataverify1 below.
//
// For each loaded plugin, the pkghashes slice has a modulehash of the
// newly loaded package that can be used to check the plugin's version of
// a package against any previously loaded version of the package.
// This is done in plugin.lastmoduleinit.
//
// modulehash用于将新模块或新模块中的程序包的ABI与加载的程序进行比较。
//
// 对于模块链接到的每个共享库，链接器在moduledata.modulehashhes切片中创建一个条目，其中包含模块的名称，
// 在链接时看到的abi哈希和指向运行时abi哈希的指针。这些在下面的moduledataverify1中检查。
//
// 对于每个已加载的插件，pkghashes切片具有新加载的程序包的modulehash，可用于对照该程序包的任何先前加载的版本来检查该程序包的插件版本。
// 这是在plugin.lastmoduleinit中完成的。
// abi: 程序二进制接口（Application Binary Interface，ABI）
type modulehash struct {
	modulename   string
	linktimehash string
	runtimehash  *string
}

// pinnedTypemaps are the map[typeOff]*_type from the moduledata objects.
//
// These typemap objects are allocated at run time on the heap, but the
// only direct reference to them is in the moduledata, created by the
// linker and marked SNOPTRDATA so it is ignored by the GC.
//
// To make sure the map isn't collected, we keep a second reference here.
//
// pinnedTypemaps是来自moduledata对象的map[typeOff]*_type。
//
// 这些类型映射对象是在运行时在堆上分配的，但是对它们的唯一直接引用是在模块数据中，该数据由链接器创建并标记为SNOPTRDATA，因此被GC忽略。
//
// 为了确保未收集map，我们在此保留第二个参考。
var pinnedTypemaps []map[typeOff]*_type

var firstmoduledata moduledata  // linker symbol // 链接器符号
var lastmoduledatap *moduledata // linker symbol // 链接器符号
var modulesSlice *[]*moduledata // see activeModules // 参见activeModules

// activeModules returns a slice of active modules.
//
// A module is active once its gcdatamask and gcbssmask have been
// assembled and it is usable by the GC.
//
// This is nosplit/nowritebarrier because it is called by the
// cgo pointer checking code.
//
// activeModules返回活动模块的一部分。
//
// 组装好模块的gcdatamask和gcbssmask后，该模块便处于活动状态，并且可由GC使用。
//
// 这是nosplit / nowritebarrier，因为它由cgo指针检查代码调用。
//
//go:nosplit
//go:nowritebarrier
func activeModules() []*moduledata {
	p := (*[]*moduledata)(atomic.Loadp(unsafe.Pointer(&modulesSlice)))
	if p == nil {
		return nil
	}
	return *p
}

// modulesinit creates the active modules slice out of all loaded modules.
//
// When a module is first loaded by the dynamic linker, an .init_array
// function (written by cmd/link) is invoked to call addmoduledata,
// appending to the module to the linked list that starts with
// firstmoduledata.
//
// There are two times this can happen in the lifecycle of a Go
// program. First, if compiled with -linkshared, a number of modules
// built with -buildmode=shared can be loaded at program initialization.
// Second, a Go program can load a module while running that was built
// with -buildmode=plugin.
//
// After loading, this function is called which initializes the
// moduledata so it is usable by the GC and creates a new activeModules
// list.
//
// Only one goroutine may call modulesinit at a time.
//
// modulesinit从所有已加载模块中创建活动模块切片。
//
// 当动态链接程序首次加载模块时，将调用.init_array函数（由cmd/link编写）以调用addmoduledata，并将该模块追加到以firstmoduledata开头的链接列表中。
//
// 在Go程序的生命周期中可能会发生两次。首先，如果使用-linkshared进行编译，则可以在程序初始化时加载使用-buildmode = shared构建的许多模块。
// 其次，Go程序可以在运行时加载使用-buildmode = plugin构建的模块。
//
// 加载后，将调用此函数以初始化moduledata，以便GC可以使用它并创建一个新的activeModules列表。
//
// 一次只能有一个goroutine可以调用modulesinit。
func modulesinit() {
	modules := new([]*moduledata)
	for md := &firstmoduledata; md != nil; md = md.next {
		if md.bad {
			continue
		}
		*modules = append(*modules, md)
		if md.gcdatamask == (bitvector{}) {
			md.gcdatamask = progToPointerMask((*byte)(unsafe.Pointer(md.gcdata)), md.edata-md.data)
			md.gcbssmask = progToPointerMask((*byte)(unsafe.Pointer(md.gcbss)), md.ebss-md.bss)
		}
	}

	// Modules appear in the moduledata linked list in the order they are
	// loaded by the dynamic loader, with one exception: the
	// firstmoduledata itself the module that contains the runtime. This
	// is not always the first module (when using -buildmode=shared, it
	// is typically libstd.so, the second module). The order matters for
	// typelinksinit, so we swap the first module with whatever module
	// contains the main function.
	//
	// See Issue #18729.
	//
	// 模块按动态加载程序加载的顺序显示在moduledata链表中，但有一个例外：firstmoduledata本身就是包含运行时的模块。
	// 这并不总是第一个模块（使用-buildmode = shared时，通常是libstd.so，它是第二个模块）。
	// 顺序对于typelinksinit很重要，因此我们将第一个模块与包含main函数的任何模块交换。
    //
    //参见问题＃18729。
	for i, md := range *modules {
		if md.hasmain != 0 {
			(*modules)[0] = md
			(*modules)[i] = &firstmoduledata
			break
		}
	}

	atomicstorep(unsafe.Pointer(&modulesSlice), unsafe.Pointer(modules))
}

type functab struct {
	entry   uintptr
	funcoff uintptr
}

// Mapping information for secondary text sections
// 二级文本段的映射信息

type textsect struct {
	vaddr    uintptr // prelinked section vaddr // 预链接的段vaddr
	length   uintptr // section length // 段长
	baseaddr uintptr // relocated section address //重新分配的段地址
}

const minfunc = 16                 // minimum function size // 最小函数大小
const pcbucketsize = 256 * minfunc // size of bucket in the pc->func lookup table // pc->func查找表中存储区的大小

// findfunctab is an array of these structures.
// Each bucket represents 4096 bytes of the text segment.
// Each subbucket represents 256 bytes of the text segment.
// To find a function given a pc, locate the bucket and subbucket for
// that pc. Add together the idx and subbucket value to obtain a
// function index. Then scan the functab array starting at that
// index to find the target function.
// This table uses 20 bytes for every 4096 bytes of code, or ~0.5% overhead.
//
// findfunctab是这些结构的数组。
// 每个存储段代表4096个字节的文本段。
// 每个子桶代表256个字节的文本段。
// 要查找给定pc的功能，请找到该pc的存储桶和子存储桶。将idx和subbucket值相加以获得函数索引。然后从该索引处开始扫描functab数组以找到目标函数。
// 此表每4096个字节的代码使用20个字节，或〜0.5％的开销。
type findfuncbucket struct {
	idx        uint32
	subbuckets [16]byte
}

func moduledataverify() {
	for datap := &firstmoduledata; datap != nil; datap = datap.next {
		moduledataverify1(datap)
	}
}

const debugPcln = false

func moduledataverify1(datap *moduledata) {
	// See golang.org/s/go12symtab for header: 0xfffffffb,
	// two zero bytes, a byte giving the PC quantum,
	// and a byte giving the pointer width in bytes.
	// 有关头部信息，请参见golang.org/s/go12symtab：0xfffffffb，两个零字节，一个字节给出PC量子，一个字节给出指针宽度（以字节为单位）。
	pcln := *(**[8]byte)(unsafe.Pointer(&datap.pclntable))
	pcln32 := *(**[2]uint32)(unsafe.Pointer(&datap.pclntable))
	if pcln32[0] != 0xfffffffb || pcln[4] != 0 || pcln[5] != 0 || pcln[6] != sys.PCQuantum || pcln[7] != sys.PtrSize {
		println("runtime: function symbol table header:", hex(pcln32[0]), hex(pcln[4]), hex(pcln[5]), hex(pcln[6]), hex(pcln[7]))
		throw("invalid function symbol table\n")
	}

	// ftab is lookup table for function by program counter.
	// ftab是按程序计数器查找功能的表。
	nftab := len(datap.ftab) - 1
	for i := 0; i < nftab; i++ {
		// NOTE: ftab[nftab].entry is legal; it is the address beyond the final function.
		// 注意：ftab[nftab].entry是合法的； 它是最终功能之外的地址。
		if datap.ftab[i].entry > datap.ftab[i+1].entry {
			f1 := funcInfo{(*_func)(unsafe.Pointer(&datap.pclntable[datap.ftab[i].funcoff])), datap}
			f2 := funcInfo{(*_func)(unsafe.Pointer(&datap.pclntable[datap.ftab[i+1].funcoff])), datap}
			f2name := "end"
			if i+1 < nftab {
				f2name = funcname(f2)
			}
			println("function symbol table not sorted by program counter:", hex(datap.ftab[i].entry), funcname(f1), ">", hex(datap.ftab[i+1].entry), f2name)
			for j := 0; j <= i; j++ {
				print("\t", hex(datap.ftab[j].entry), " ", funcname(funcInfo{(*_func)(unsafe.Pointer(&datap.pclntable[datap.ftab[j].funcoff])), datap}), "\n")
			}
			if GOOS == "aix" && isarchive {
				println("-Wl,-bnoobjreorder is mandatory on aix/ppc64 with c-archive")
			}
			throw("invalid runtime symbol table")
		}
	}

	if datap.minpc != datap.ftab[0].entry ||
		datap.maxpc != datap.ftab[nftab].entry {
		throw("minpc or maxpc invalid")
	}

	for _, modulehash := range datap.modulehashes {
		if modulehash.linktimehash != *modulehash.runtimehash {
			println("abi mismatch detected between", datap.modulename, "and", modulehash.modulename)
			throw("abi mismatch")
		}
	}
}

// FuncForPC returns a *Func describing the function that contains the
// given program counter address, or else nil.
//
// If pc represents multiple functions because of inlining, it returns
// the a *Func describing the innermost function, but with an entry
// of the outermost function.
//
// FuncForPC返回*Func，描述包含给定程序计数器地址的函数，否则为nil。
//
// 如果pc由于内联而表示多个函数，它将返回*Func描述最内部的函数，但带有最外部的函数的条目。
func FuncForPC(pc uintptr) *Func {
	f := findfunc(pc)
	if !f.valid() {
		return nil
	}
	if inldata := funcdata(f, _FUNCDATA_InlTree); inldata != nil {
		// Note: strict=false so bad PCs (those between functions) don't crash the runtime.
		// We just report the preceding function in that situation. See issue 29735.
		// TODO: Perhaps we should report no function at all in that case.
		// The runtime currently doesn't have function end info, alas.
		// 注意：strict = false，因此错误的PC（功能之间的PC）不会使运行时崩溃。 我们只是报告这种情况下的先前功能。 请参阅问题29735。
		// TODO：在这种情况下，也许我们根本不报告任何功能。
		// 运行时当前没有函数结束信息。
		if ix := pcdatavalue1(f, _PCDATA_InlTreeIndex, pc, nil, false); ix >= 0 {
			inltree := (*[1 << 20]inlinedCall)(inldata)
			name := funcnameFromNameoff(f, inltree[ix].func_)
			file, line := funcline(f, pc)
			fi := &funcinl{
				entry: f.entry, // entry of the real (the outermost) function. // 实际（最外层）函数的入口。
				name:  name,
				file:  file,
				line:  int(line),
			}
			return (*Func)(unsafe.Pointer(fi))
		}
	}
	return f._Func()
}

// Name returns the name of the function.
// Name返回函数的名称。
func (f *Func) Name() string {
	if f == nil {
		return ""
	}
	fn := f.raw()
	if fn.entry == 0 { // inlined version // 内联版本
		fi := (*funcinl)(unsafe.Pointer(fn))
		return fi.name
	}
	return funcname(f.funcInfo())
}

// Entry returns the entry address of the function.
// Entry返回函数的入口地址。
func (f *Func) Entry() uintptr {
	fn := f.raw()
	if fn.entry == 0 { // inlined version // 内联版本
		fi := (*funcinl)(unsafe.Pointer(fn))
		return fi.entry
	}
	return fn.entry
}

// FileLine returns the file name and line number of the
// source code corresponding to the program counter pc.
// The result will not be accurate if pc is not a program
// counter within f.
// FileLine返回与程序计数器pc对应的源代码的文件名和行号。
// 如果pc不是f中的程序计数器，则结果将不准确。
func (f *Func) FileLine(pc uintptr) (file string, line int) {
	fn := f.raw()
	if fn.entry == 0 { // inlined version // 内联版本
		fi := (*funcinl)(unsafe.Pointer(fn))
		return fi.file, fi.line
	}
	// Pass strict=false here, because anyone can call this function,
	// and they might just be wrong about targetpc belonging to f.
	// 在此处传递strict = false，因为任何人都可以调用此函数，而他们对于属于f的targetpc可能只是错误的。
	file, line32 := funcline1(f.funcInfo(), pc, false)
	return file, int(line32)
}

func findmoduledatap(pc uintptr) *moduledata {
	for datap := &firstmoduledata; datap != nil; datap = datap.next {
		if datap.minpc <= pc && pc < datap.maxpc {
			return datap
		}
	}
	return nil
}

type funcInfo struct {
	*_func
	datap *moduledata
}

func (f funcInfo) valid() bool {
	return f._func != nil
}

func (f funcInfo) _Func() *Func {
	return (*Func)(unsafe.Pointer(f._func))
}

func findfunc(pc uintptr) funcInfo {
	datap := findmoduledatap(pc)
	if datap == nil {
		return funcInfo{}
	}
	const nsub = uintptr(len(findfuncbucket{}.subbuckets))

	x := pc - datap.minpc
	b := x / pcbucketsize
	i := x % pcbucketsize / (pcbucketsize / nsub)

	ffb := (*findfuncbucket)(add(unsafe.Pointer(datap.findfunctab), b*unsafe.Sizeof(findfuncbucket{})))
	idx := ffb.idx + uint32(ffb.subbuckets[i])

	// If the idx is beyond the end of the ftab, set it to the end of the table and search backward.
	// This situation can occur if multiple text sections are generated to handle large text sections
	// and the linker has inserted jump tables between them.
	// 如果idx超出ftab的末尾，请将其设置为表的末尾并向后搜索。
    // 如果生成多个文本段来处理较大的文本段，并且链接器在它们之间插入了跳转表，则可能会发生这种情况。

	if idx >= uint32(len(datap.ftab)) {
		idx = uint32(len(datap.ftab) - 1)
	}
	if pc < datap.ftab[idx].entry {
		// With multiple text sections, the idx might reference a function address that
		// is higher than the pc being searched, so search backward until the matching address is found.
		// 对于多个文本段，idx可能引用的功能地址高于要搜索的pc，因此请向后搜索直到找到匹配的地址。

		for datap.ftab[idx].entry > pc && idx > 0 {
			idx--
		}
		if idx == 0 {
			throw("findfunc: bad findfunctab entry idx")
		}
	} else {
		// linear search to find func with pc >= entry. //使用pc >= entry，线性搜索查找函数。
		for datap.ftab[idx+1].entry <= pc {
			idx++
		}
	}
	funcoff := datap.ftab[idx].funcoff
	if funcoff == ^uintptr(0) {
		// With multiple text sections, there may be functions inserted by the external
		// linker that are not known by Go. This means there may be holes in the PC
		// range covered by the func table. The invalid funcoff value indicates a hole.
		// See also cmd/link/internal/ld/pcln.go:pclntab
		// 对于多个文本节段，可能是外部链接器插入了Go未知的功能。这意味着func表覆盖的PC范围中可能有孔。
		// 无效的funcoff值表示有孔。另请参阅cmd/link/internal/ld/pcln.go:pclntab
		return funcInfo{}
	}
	return funcInfo{(*_func)(unsafe.Pointer(&datap.pclntable[funcoff])), datap}
}

type pcvalueCache struct {
	entries [2][8]pcvalueCacheEnt
}

type pcvalueCacheEnt struct {
	// targetpc and off together are the key of this cache entry.
	// targetpc和off一起是此缓存项的键。
	targetpc uintptr
	off      int32
	// val is the value of this cached pcvalue entry.
	// val是此缓存的pcvalue条目的值。
	val int32
}

// pcvalueCacheKey returns the outermost index in a pcvalueCache to use for targetpc.
// It must be very cheap to calculate.
// For now, align to sys.PtrSize and reduce mod the number of entries.
// In practice, this appears to be fairly randomly and evenly distributed.
// pcvalueCacheKey返回pcvalueCache中最外面的索引以用于targetpc。计算它必须非常廉价。
// 现在，对齐sys.PtrSize并减少mod的条目数。实际上，这似乎是相当随机且均匀分布的。
func pcvalueCacheKey(targetpc uintptr) uintptr {
	return (targetpc / sys.PtrSize) % uintptr(len(pcvalueCache{}.entries))
}

func pcvalue(f funcInfo, off int32, targetpc uintptr, cache *pcvalueCache, strict bool) int32 {
	if off == 0 {
		return -1
	}

	// Check the cache. This speeds up walks of deep stacks, which
	// tend to have the same recursive functions over and over.
	//
	// This cache is small enough that full associativity is
	// cheaper than doing the hashing for a less associative
	// cache.
	//
	// 检查缓存。这加快了深层堆栈的运行速度，深层堆栈往往一遍又一遍地具有相同的递归功能。
    //
    // 此缓存足够小，以至于与不那么相关的缓存进行哈希处理相比，完全关联性要廉价。
	if cache != nil {
		x := pcvalueCacheKey(targetpc)
		for i := range cache.entries[x] {
			// We check off first because we're more
			// likely to have multiple entries with
			// different offsets for the same targetpc
			// than the other way around, so we'll usually
			// fail in the first clause.
			// 我们先进行检查，对于同一个targetpc，我们更有可能有多个具有不同偏移量的条目，因此通常在first子句中会失败。
			ent := &cache.entries[x][i]
			if ent.off == off && ent.targetpc == targetpc {
				return ent.val
			}
		}
	}

	if !f.valid() {
		if strict && panicking == 0 {
			print("runtime: no module data for ", hex(f.entry), "\n")
			throw("no module data")
		}
		return -1
	}
	datap := f.datap
	p := datap.pclntable[off:]
	pc := f.entry
	val := int32(-1)
	for {
		var ok bool
		p, ok = step(p, &pc, &val, pc == f.entry)
		if !ok {
			break
		}
		if targetpc < pc {
			// Replace a random entry in the cache. Random
			// replacement prevents a performance cliff if
			// a recursive stack's cycle is slightly
			// larger than the cache.
			// Put the new element at the beginning,
			// since it is the most likely to be newly used.
			// 替换缓存中的随机条目。如果递归堆栈的周期略大于缓存，则随机替换可防止性能下降。
			// 将新元素放在开头，因为它是最有可能被新使用的元素。
			if cache != nil {
				x := pcvalueCacheKey(targetpc)
				e := &cache.entries[x]
				ci := fastrand() % uint32(len(cache.entries[x]))
				e[ci] = e[0]
				e[0] = pcvalueCacheEnt{
					targetpc: targetpc,
					off:      off,
					val:      val,
				}
			}

			return val
		}
	}

	// If there was a table, it should have covered all program counters.
	// If not, something is wrong.
	// 如果有一个表，它应该已经覆盖了所有程序计数器。如果没有，那是不对的。
	if panicking != 0 || !strict {
		return -1
	}

	print("runtime: invalid pc-encoded table f=", funcname(f), " pc=", hex(pc), " targetpc=", hex(targetpc), " tab=", p, "\n")

	p = datap.pclntable[off:]
	pc = f.entry
	val = -1
	for {
		var ok bool
		p, ok = step(p, &pc, &val, pc == f.entry)
		if !ok {
			break
		}
		print("\tvalue=", val, " until pc=", hex(pc), "\n")
	}

	throw("invalid runtime symbol table")
	return -1
}

func cfuncname(f funcInfo) *byte {
	if !f.valid() || f.nameoff == 0 {
		return nil
	}
	return &f.datap.pclntable[f.nameoff]
}

func funcname(f funcInfo) string {
	return gostringnocopy(cfuncname(f))
}

func cfuncnameFromNameoff(f funcInfo, nameoff int32) *byte {
	if !f.valid() {
		return nil
	}
	return &f.datap.pclntable[nameoff]
}

func funcnameFromNameoff(f funcInfo, nameoff int32) string {
	return gostringnocopy(cfuncnameFromNameoff(f, nameoff))
}

func funcfile(f funcInfo, fileno int32) string {
	datap := f.datap
	if !f.valid() {
		return "?"
	}
	return gostringnocopy(&datap.pclntable[datap.filetab[fileno]])
}

func funcline1(f funcInfo, targetpc uintptr, strict bool) (file string, line int32) {
	datap := f.datap
	if !f.valid() {
		return "?", 0
	}
	fileno := int(pcvalue(f, f.pcfile, targetpc, nil, strict))
	line = pcvalue(f, f.pcln, targetpc, nil, strict)
	if fileno == -1 || line == -1 || fileno >= len(datap.filetab) {
		// print("looking for ", hex(targetpc), " in ", funcname(f), " got file=", fileno, " line=", lineno, "\n")
		return "?", 0
	}
	file = gostringnocopy(&datap.pclntable[datap.filetab[fileno]])
	return
}

func funcline(f funcInfo, targetpc uintptr) (file string, line int32) {
	return funcline1(f, targetpc, true)
}

func funcspdelta(f funcInfo, targetpc uintptr, cache *pcvalueCache) int32 {
	x := pcvalue(f, f.pcsp, targetpc, cache, true)
	if x&(sys.PtrSize-1) != 0 {
		print("invalid spdelta ", funcname(f), " ", hex(f.entry), " ", hex(targetpc), " ", hex(f.pcsp), " ", x, "\n")
	}
	return x
}

// funcMaxSPDelta returns the maximum spdelta at any point in f.
// funcMaxSPDelta返回f中任意点的最大spdelta。
func funcMaxSPDelta(f funcInfo) int32 {
	datap := f.datap
	p := datap.pclntable[f.pcsp:]
	pc := f.entry
	val := int32(-1)
	max := int32(0)
	for {
		var ok bool
		p, ok = step(p, &pc, &val, pc == f.entry)
		if !ok {
			return max
		}
		if val > max {
			max = val
		}
	}
}

func pcdatastart(f funcInfo, table int32) int32 {
	return *(*int32)(add(unsafe.Pointer(&f.nfuncdata), unsafe.Sizeof(f.nfuncdata)+uintptr(table)*4))
}

func pcdatavalue(f funcInfo, table int32, targetpc uintptr, cache *pcvalueCache) int32 {
	if table < 0 || table >= f.npcdata {
		return -1
	}
	return pcvalue(f, pcdatastart(f, table), targetpc, cache, true)
}

func pcdatavalue1(f funcInfo, table int32, targetpc uintptr, cache *pcvalueCache, strict bool) int32 {
	if table < 0 || table >= f.npcdata {
		return -1
	}
	return pcvalue(f, pcdatastart(f, table), targetpc, cache, strict)
}

func funcdata(f funcInfo, i uint8) unsafe.Pointer {
	if i < 0 || i >= f.nfuncdata {
		return nil
	}
	p := add(unsafe.Pointer(&f.nfuncdata), unsafe.Sizeof(f.nfuncdata)+uintptr(f.npcdata)*4)
	if sys.PtrSize == 8 && uintptr(p)&4 != 0 {
		if uintptr(unsafe.Pointer(f._func))&4 != 0 {
			println("runtime: misaligned func", f._func)
		}
		p = add(p, 4)
	}
	return *(*unsafe.Pointer)(add(p, uintptr(i)*sys.PtrSize))
}

// step advances to the next pc, value pair in the encoded table.
// 步骤前进到编码表中的下一个pc，值对。
func step(p []byte, pc *uintptr, val *int32, first bool) (newp []byte, ok bool) {
	// For both uvdelta and pcdelta, the common case (~70%)
	// is that they are a single byte. If so, avoid calling readvarint.
	// 对于uvdelta和pcdelta，通常的情况（〜70％）是它们是一个字节。如果是这样，请避免调用readvarint。
	uvdelta := uint32(p[0])
	if uvdelta == 0 && !first {
		return nil, false
	}
	n := uint32(1)
	if uvdelta&0x80 != 0 {
		n, uvdelta = readvarint(p)
	}
	*val += int32(-(uvdelta & 1) ^ (uvdelta >> 1))
	p = p[n:]

	pcdelta := uint32(p[0])
	n = 1
	if pcdelta&0x80 != 0 {
		n, pcdelta = readvarint(p)
	}
	p = p[n:]
	*pc += uintptr(pcdelta * sys.PCQuantum)
	return p, true
}

// readvarint reads a varint from p.
// readvarint从p读取varint。
func readvarint(p []byte) (read uint32, val uint32) {
	var v, shift, n uint32
	for {
		b := p[n]
		n++
		v |= uint32(b&0x7F) << (shift & 31)
		if b&0x80 == 0 {
			break
		}
		shift += 7
	}
	return n, v
}

type stackmap struct {
	n        int32   // number of bitmaps // 位图数量
	nbit     int32   // number of bits in each bitmap // 每个位图中的位数
	bytedata [1]byte // bitmaps, each starting on a byte boundary // 位图，每个位图从字节边界开始
}

//go:nowritebarrier
func stackmapdata(stkmap *stackmap, n int32) bitvector {
	// Check this invariant only when stackDebug is on at all.
	// The invariant is already checked by many of stackmapdata's callers,
	// and disabling it by default allows stackmapdata to be inlined.
	// 仅在完全打开stackDebug时检查此不变式。许多stackmapdata的调用者已经检查了该不变式，默认情况下将其禁用可允许内联stackmapdata。
	if stackDebug > 0 && (n < 0 || n >= stkmap.n) {
		throw("stackmapdata: index out of range")
	}
	return bitvector{stkmap.nbit, addb(&stkmap.bytedata[0], uintptr(n*((stkmap.nbit+7)>>3)))}
}

// inlinedCall is the encoding of entries in the FUNCDATA_InlTree table.
// inlinedCall是FUNCDATA_InlTree表中条目的编码。
type inlinedCall struct {
	parent   int16  // index of parent in the inltree, or < 0 // 父树在索引树中的索引，或<0
	funcID   funcID // type of the called function // 被调用函数的类型
	_        byte
	file     int32 // fileno index into filetab // fileno到filetab中的索引
	line     int32 // line number of the call site // 调用点的行号
	func_    int32 // offset into pclntab for name of called function // 偏移到pclntab中作为被调用函数的名称
	parentPc int32 // position of an instruction whose source position is the call site (offset from entry) // 指令的位置，该指令的源位置是调用位置（从输入位置偏移）
}
```