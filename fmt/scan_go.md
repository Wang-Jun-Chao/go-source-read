
```go
package fmt

import (
	"errors"
	"io"
	"math"
	"os"
	"reflect"
	"strconv"
	"sync"
	"unicode/utf8"
)

// ScanState代表一个将传递给Scanner接口的Scan方法的扫描环境。Scan函数中，可以进行一次一个rune的扫描，或者使用Token方法获得下一个token（比如空白分隔的token）。
// ScanState represents the scanner state passed to custom scanners.
// Scanners may do rune-at-a-time scanning or ask the ScanState
// to discover the next space-delimited token.
type ScanState interface {
    // ReadRune从输入中读取下一个符文（Unicode代码点）。如果在Scanln，Fscanln或Sscanln期间调用，则ReadRune（）将在返回第一个'\ n'或读取超出指定宽度后返回EOF。
    // @return r        读取到的unicode字符
    // @return size     TODO
    // @return err      是否出错
	// ReadRune reads the next rune (Unicode code point) from the input.
	// If invoked during Scanln, Fscanln, or Sscanln, ReadRune() will
	// return EOF after returning the first '\n' or when reading beyond
	// the specified width.
	ReadRune() (r rune, size int, err error)
	// UnreadRune导致对ReadRune的下一次调用返回相同的符文。
	// @return err      是否出错
	// UnreadRune causes the next call to ReadRune to return the same rune.
	UnreadRune() error
	// SkipSpace跳过输入中的空间。换行符在操作过程中会被适当处理；有关更多信息，请参见软件包文档。
	// SkipSpace skips space in the input. Newlines are treated appropriately
	// for the operation being performed; see the package documentation
	// for more information.
	SkipSpace()
	// 如果skipSpace为true，则Token会跳过输入中的空格，然后返回满足f(c)的Unicode码点值c。如果f为nil，则使用!unicode.IsSpace(c);也就是说，Token将包含非空格字符。换行符在操作过程中会被适当处理；有关更多信息，请参见软件包文档。返回的切片指向共享数据，这些次数据可能会被下一次的Token调用重写，也可能使用ScanState作为输入的对Scan函数的调用重写，还可能当调用Scan方法返回时写重。
	// @param  skipSpace        是否忽略空格             
	// @param  func(rune) bool  rune判定函数
	// @return token            返回读取的token数
	// @return err              读取是否出错
	// Token skips space in the input if skipSpace is true, then returns the
	// run of Unicode code points c satisfying f(c).  If f is nil,
	// !unicode.IsSpace(c) is used; that is, the token will hold non-space
	// characters. Newlines are treated appropriately for the operation being
	// performed; see the package documentation for more information.
	// The returned slice points to shared data that may be overwritten
	// by the next call to Token, a call to a Scan function using the ScanState
	// as input, or when the calling Scan method returns.
	Token(skipSpace bool, f func(rune) bool) (token []byte, err error)
	// Width返回width选项的值以及是否已设置。 单位是Unicode代码点。
	// @return wid  width选项的值
	// @return ok   width选项的值是否已经设置
	// Width returns the value of the width option and whether it has been set.
	// The unit is Unicode code points.
	Width() (wid int, ok bool)
	// 由于ReadRune由该接口实现，因此扫描程序永远不应调用Read，并且ScanState的有效实现可以选择始终从Read返回错误。
	// @param   buff    待读取的字节数组
	// @return  n       本次读取的字符数
	// @return  err     读取是否出错
	// Because ReadRune is implemented by the interface, Read should never be
	// called by the scanning routines and a valid implementation of
	// ScanState may choose always to return an error from Read.
	Read(buf []byte) (n int, err error)
}
// 当Scan、Scanf、Scanln或类似函数接受实现了Scanner接口的类型（其Scan方法的receiver必须是指针，该方法从输入读取该类型值的字符串表示并将结果写入receiver）作为参数时，会调用其Scan方法进行定制的扫描。
// Scanner is implemented by any value that has a Scan method, which scans
// the input for the representation of a value and stores the result in the
// receiver, which must be a pointer to be useful. The Scan method is called
// for any argument to Scan, Scanf, or Scanln that implements it.
type Scanner interface {
	Scan(state ScanState, verb rune) error
}

// Scan会扫描从标准输入读取的文本，并将连续的以空格分隔的值存储到连续的参数中。 换行符算作空格。 它返回成功扫描的项目数。 如果该数目少于参数数目，则err将报告原因。
// @param   a   用于保存值的参数
// @return  n   写入的参数个数
// @return  err 出错情况
// Scan scans text read from standard input, storing successive
// space-separated values into successive arguments. Newlines count
// as space. It returns the number of items successfully scanned.
// If that is less than the number of arguments, err will report why.
func Scan(a ...interface{}) (n int, err error) {
	return Fscan(os.Stdin, a...)
}

// Scanln与Scan类似，但是在换行符处停止扫描，并且在最后一个条目之后必须有换行符或EOF。
// @param   a   用于保存值的参数
// @return  n   写入的参数个数
// @return  err 出错情况
// Scanln is similar to Scan, but stops scanning at a newline and
// after the final item there must be a newline or EOF.
func Scanln(a ...interface{}) (n int, err error) {
	return Fscanln(os.Stdin, a...)
}

//Scanf扫描从标准输入读取的文本，将连续的空格分隔的值存储到由格式确定的连续的参数中。它返回成功扫描的项目数。如果该数目少于参数数目，则err将报告原因。输入中的换行符必须与格式中的换行符匹配。一个例外：动词％c始终扫描输入中的下一个符文，即使它是空格（或制表符等）或换行符也是如此。
// @param   format  字符串格式
// @param   a       用于保存值的参数
// @return  n       写入的参数个数
// @return  err     出错情况
// Scanf scans text read from standard input, storing successive
// space-separated values into successive arguments as determined by
// the format. It returns the number of items successfully scanned.
// If that is less than the number of arguments, err will report why.
// Newlines in the input must match newlines in the format.
// The one exception: the verb %c always scans the next rune in the
// input, even if it is a space (or tab etc.) or newline.
func Scanf(format string, a ...interface{}) (n int, err error) {
	return Fscanf(os.Stdin, format, a...)
}

type stringReader string

// 将字符串中的值拷贝到字节数组b中
// @param   b   用于保存数据的字节数组
// @return  n   写入的参数个数
// @return  err 读取是否出错
func (r *stringReader) Read(b []byte) (n int, err error) {
	n = copy(b, *r)
	*r = (*r)[n:]
	if n == 0 { // 没有拷贝说明已经结束
		err = io.EOF
	}
	return
}

// Sscan扫描参数字符串，并将连续的以空格分隔的值存储到连续的参数中。换行符算作空格。它返回成功扫描的项目数。如果该数目少于参数数目，则err将报告原因。
// @param   str 待扫描的字符串
// @param   a   用于保存值的参数
// @return  n   写入的参数个数
// @return  err 出错情况
// Sscan scans the argument string, storing successive space-separated
// values into successive arguments. Newlines count as space. It
// returns the number of items successfully scanned. If that is less
// than the number of arguments, err will report why.
func Sscan(str string, a ...interface{}) (n int, err error) {
	return Fscan((*stringReader)(&str), a...)
}

// Sscanln与Sscan类似，但是Sscanln在换行符处停止扫描，并且在最后一个项目之后必须有换行符或EOF。
// @param   str 待扫描的字符串
// @param   a   用于保存值的参数
// @return  n   写入的参数个数
// @return  err 出错情况
// Sscanln is similar to Sscan, but stops scanning at a newline and
// after the final item there must be a newline or EOF.
func Sscanln(str string, a ...interface{}) (n int, err error) {
	return Fscanln((*stringReader)(&str), a...)
}

// Sscanf扫描参数字符串，将连续的以空格分隔的值存储到由格式确定的连续的参数中。 它返回成功解析的项目数。
输入中的换行符必须与格式中的换行符匹配。
// @param   str     待扫描的字符串
// @parma   format  格式化字符串
// @param   a       用于保存值的参数
// @return  n       写入的参数个数
// @return  err     出错情况
// Sscanf scans the argument string, storing successive space-separated
// values into successive arguments as determined by the format. It
// returns the number of items successfully parsed.
// Newlines in the input must match newlines in the format.
func Sscanf(str string, format string, a ...interface{}) (n int, err error) {
	return Fscanf((*stringReader)(&str), format, a...)
}

// Fscan扫描从r读取的文本，并将连续的以空格分隔的值存储到连续的参数中。换行符算作空格。它返回成功扫描的项目数。如果该数目少于参数数目，则err将报告原因。
// @param   r       数据输入源
// @param   a       用于保存值的参数
// @return  n       写入的参数个数
// @return  err     出错情况
// Fscan scans text read from r, storing successive space-separated
// values into successive arguments. Newlines count as space. It
// returns the number of items successfully scanned. If that is less
// than the number of arguments, err will report why.
func Fscan(r io.Reader, a ...interface{}) (n int, err error) {
	s, old := newScanState(r, true, false)
	n, err = s.doScan(a)
	s.free(old)
	return
}

// Fscanln与Fscan类似，但是Fscanln在换行符处停止扫描，并且在最后一个条目之后必须有换行符或EOF。
// @param   r       数据输入源
// @param   a       用于保存值的参数
// @return  n       写入的参数个数
// @return  err     出错情况
// Fscanln is similar to Fscan, but stops scanning at a newline and
// after the final item there must be a newline or EOF.
func Fscanln(r io.Reader, a ...interface{}) (n int, err error) {
	s, old := newScanState(r, false, true)
	n, err = s.doScan(a)
	s.free(old)
	return
}
// Fscanf扫描从r读取的文本，将连续的以空格分隔的值存储到由格式确定的连续的参数中。 它返回成功解析的项目数。 输入中的换行符必须与格式中的换行符匹配。
// @param   r       数据输入源
// @parma   format  格式化字符串
// @param   a       用于保存值的参数
// @return  n       写入的参数个数
// @return  err     出错情况
// Fscanf scans text read from r, storing successive space-separated
// values into successive arguments as determined by the format. It
// returns the number of items successfully parsed.
// Newlines in the input must match newlines in the format.
func Fscanf(r io.Reader, format string, a ...interface{}) (n int, err error) {
	s, old := newScanState(r, false, false)
	n, err = s.doScanf(format, a)
	s.free(old)
	return
}

// scanError表示由扫描软件生成的错误。 它用作唯一的签名，以在恢复时识别此类错误。
// scanError represents an error generated by the scanning software.
// It's used as a unique signature to identify such errors when recovering.
type scanError struct {
	err error
}

const eof = -1

// ss is the internal implementation of ScanState.
type ss struct {
	rs    io.RuneScanner // where to read input
	buf   buffer         // token accumulator
	count int            // runes consumed so far.
	atEOF bool           // already read EOF
	ssave
}

// save保存ss中需要在递归扫描中保存和恢复的部分。
// ssave holds the parts of ss that need to be
// saved and restored on recursive scans.
type ssave struct {
	validSave bool // is or was a part of an actual ss. // 是或曾经是实际ss的一部分。
	nlIsEnd   bool // whether newline terminates scan // 换行符是否终止扫描
	nlIsSpace bool // whether newline counts as white space // 换行符是否算作空格
	argLimit  int  // max value of ss.count for this arg; argLimit <= limit // 此arg的ss.count的最大值； argLimit <= limit
	limit     int  // max value of ss.count. // ss.count的最大值。
	maxWid    int  // width of this arg. // 此arg的宽度
}

// 此方法不应该被调用
// The Read method is only in ScanState so that ScanState
// satisfies io.Reader. It will never be called when used as
// intended, so there is no need to make it actually work.
func (s *ss) Read(buf []byte) (n int, err error) {
	return 0, errors.New("ScanState's Read should not be called. Use ReadRune")
}

// 从s中读取一个字符
// @return  r       读取到的字符
// @return  size    字符表示需要的字节数
// @return  err     错误信息
func (s *ss) ReadRune() (r rune, size int, err error) {
	if s.atEOF || s.count >= s.argLimit { // 读取末尾或者超出限制
		err = io.EOF
		return
	}

	r, size, err = s.rs.ReadRune() // 读取一个字符
	if err == nil { // 无错误，字符数计算加一
		s.count++
		if s.nlIsEnd && r == '\n' { // 换行符是终止符
			s.atEOF = true // 票房扫描结束
		}
	} else if err == io.EOF {  // 已经扫描结束
		s.atEOF = true
	}
	return
}

// 求宽度
// @return  wid 宽度
// @return  ok  宽度是否设置
func (s *ss) Width() (wid int, ok bool) {
	if s.maxWid == hugeWid // hugeWid = 1 << 30
		return 0, false
	}
	return s.maxWid, true
}

// getRune的公开方法返回错误； 包私有方法返回恐慌。如果getRune达到EOF，则返回值为EOF（-1）。
// The public method returns an error; this private one panics.
// If getRune reaches EOF, the return value is EOF (-1).
func (s *ss) getRune() (r rune) {
	r, _, err := s.ReadRune()
	if err != nil {
		if err == io.EOF {
			return eof
		}
		s.error(err)
	}
	return
}

// mustReadRune turns io.EOF into a panic(io.ErrUnexpectedEOF).
// It is called in cases such as string scanning where an EOF is a
// syntax error.
func (s *ss) mustReadRune() (r rune) {
	r = s.getRune()
	if r == eof {
		s.error(io.ErrUnexpectedEOF)
	}
	return
}

// 回退
// @return err 总是nil
func (s *ss) UnreadRune() error {
	s.rs.UnreadRune()
	s.atEOF = false
	s.count--
	return nil
}

// 抛出panice
func (s *ss) error(err error) {
	panic(scanError{err})
}

// 抛出panice
func (s *ss) errorString(err string) {
	panic(scanError{errors.New(err)})
}

// 读取一个字符
// @param   skipSpace   忽略空格
// @param   f           匹配函数
// @return  tok         读取到的字符以字节方式存储
// @return  err         是否有错误
func (s *ss) Token(skipSpace bool, f func(rune) bool) (tok []byte, err error) {
	defer func() {
		if e := recover(); e != nil {
			if se, ok := e.(scanError); ok {
				err = se.err
			} else {
				panic(e)
			}
		}
	}()
	if f == nil { 
		f = notSpace // 不忽略空白字符
	}
	s.buf = s.buf[:0]
	tok = s.token(skipSpace, f)
	return
}

// space是unicode.White_Space范围的副本，以避免依赖于软件包unicode。
// space is a copy of the unicode.White_Space ranges,
// to avoid depending on package unicode.
var space = [][2]uint16{
	{0x0009, 0x000d},
	{0x0020, 0x0020},
	{0x0085, 0x0085},
	{0x00a0, 0x00a0},
	{0x1680, 0x1680},
	{0x2000, 0x200a},
	{0x2028, 0x2029},
	{0x202f, 0x202f},
	{0x205f, 0x205f},
	{0x3000, 0x3000},
}

// 判断是否是空白字符
func isSpace(r rune) bool {
	if r >= 1<<16 {
		return false
	}
	rx := uint16(r)
	for _, rng := range space {
		if rx < rng[0] {
			return false
		}
		if rx <= rng[1] {
			return true
		}
	}
	return false
}

// notSpace是Token方法中使用的默认扫描功能。
// notSpace is the default scanning function used in Token.
func notSpace(r rune) bool {
	return !isSpace(r)
}

// readRune是一种用于从io.Reader读取UTF-8编码的代码点的结构全。 如果授予scanner的reader尚未实现io.RuneScanner，则使用它。
// readRune is a structure to enable reading UTF-8 encoded code points
// from an io.Reader. It is used if the Reader given to the scanner does
// not already implement io.RuneScanner.
type readRune struct {
	reader   io.Reader
	buf      [utf8.UTFMax]byte // used only inside ReadRune // 仅在ReadRune内部使用
	pending  int               // number of bytes in pendBuf; only >0 for bad UTF-8 // pendBuf中的字节数； 对于错误的UTF-8，仅> 0
	pendBuf  [utf8.UTFMax]byte // bytes left over
	peekRune rune              // if >=0 next rune; when <0 is ^(previous Rune) // 当>=0时，表示下一个字符，当<0时，表示上一个字符的异或
}

// readByte返回输入的下一个字节，如果UTF-8格式不正确，则该字节可以从上一次读取中保留下来。
// return b     读取到的字节
// return err   出错误情况
// readByte returns the next byte from the input, which may be
// left over from a previous read if the UTF-8 was ill-formed.
func (r *readRune) readByte() (b byte, err error) {
	if r.pending > 0 { // 如果有pending，就取pendBuf和第一个字节
		b = r.pendBuf[0]
		copy(r.pendBuf[0:], r.pendBuf[1:]) // 字节数组第一个元素之后的值，整体向前移动一个位置
		r.pending--
		return
	}
	n, err := io.ReadFull(r.reader, r.pendBuf[:1]) // 使用io.ReadFull处理pendBuf第一个字节
	if n != 1 { // 说明处理不成功
		return 0, err
	}
	return r.pendBuf[0], err
}

// ReadRune从r中的io.Reader返回下一个UTF-8编码的代码点。
// @return rr   读取到的字符
// @return size rune的字节数
// @return err  是否出错
// ReadRune returns the next UTF-8 encoded code point from the
// io.Reader inside r.
func (r *readRune) ReadRune() (rr rune, size int, err error) {
	if r.peekRune >= 0 { // 存在下一个字符
		rr = r.peekRune // 读取
		r.peekRune = ^r.peekRune // 求异或
		size = utf8.RuneLen(rr) // 求长度
		return
	}
	
	// r.peekRune不代表下一个字符
	r.buf[0], err = r.readByte() // 读取一个字符
	if err != nil {
		return
	}
	
	// 表示单个字节表示的unicode字符
	if r.buf[0] < utf8.RuneSelf { // fast check for common ASCII case // 快速检查常见ASCII大小写
		rr = rune(r.buf[0])
		size = 1 // Known to be 1.
		// Flip the bits of the rune so it's available to UnreadRune. // 翻转符文的各个位，以便UnreadRune可以使用。
		r.peekRune = ^rr
		return
	}
	
	// 一个字节，二个字节，...，进行尝试读取字符
	var n int
	for n = 1; !utf8.FullRune(r.buf[:n]); n++ {
		r.buf[n], err = r.readByte()
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return
		}
	}
	
	// 转成字符
	rr, size = utf8.DecodeRune(r.buf[:n])
	if size < n { // an error, save the bytes for the next read // 错误，保存字节供下次读取
		copy(r.pendBuf[r.pending:], r.buf[size:n])
		r.pending += n - size
	}
	// 翻转符文的各个位，以便UnreadRune可以使用。
	// Flip the bits of the rune so it's available to UnreadRune.
	r.peekRune = ^rr
	return
}

// 还原一个字符读取
// @return 是否有错误
func (r *readRune) UnreadRune() error {
    // 当时的r.peekRune是有效的，不能还原
	if r.peekRune >= 0 {
		return errors.New("fmt: scanning called UnreadRune with no rune available")
	}
	
	// 将先前读取的符文的位反转以获得有效的>=0状态。
	// Reverse bit flip of previously read rune to obtain valid >=0 state.
	r.peekRune = ^r.peekRune
	return nil
}

// 缓存池，用于创建ss对象
var ssFree = sync.Pool{
	New: func() interface{} { return new(ss) },
}

// newScanState分配一个新的ss结构或获取一个缓存的结构。
// @param   r           数据源
// @param   niIsSpace   换行符是否算作空格
// @param   nlIsEnd     换行符是否终止扫描
// @return  s           ss类型
// @return  old         总是返回false
// newScanState allocates a new ss struct or grab a cached one.
func newScanState(r io.Reader, nlIsSpace, nlIsEnd bool) (s *ss, old ssave) {
	s = ssFree.Get().(*ss) // 从缓存中取值
	// 如果s是io.RuneScanner类型就是直接赋值给s.rs，否则创建readRune再赋值
	if rs, ok := r.(io.RuneScanner); ok { 
		s.rs = rs
	} else {
		s.rs = &readRune{reader: r, peekRune: -1}
	}
	
	// 设置初始属性值
	s.nlIsSpace = nlIsSpace
	s.nlIsEnd = nlIsEnd
	s.atEOF = false
	s.limit = hugeWid
	s.argLimit = hugeWid
	s.maxWid = hugeWid
	s.validSave = true
	s.count = 0
	return
}

// free将使用过的ss结构保存在ssFree中； 避免每次调用分配。
// free saves used ss structs in ssFree; avoid an allocation per invocation.
func (s *ss) free(old ssave) {
    // 如果以递归方式使用它，则只需恢复旧状态即可。
	// If it was used recursively, just restore the old state.
	if old.validSave {
		s.ssave = old
		return
	}
	
	// 大缓冲区的ss结构不回收，这么做是为了避免缓冲区占用大量内存
	// Don't hold on to ss structs with large buffers.
	if cap(s.buf) > 1024 {
		return
	}
	
	// 进行数据清理
	s.buf = s.buf[:0]
	s.rs = nil
	ssFree.Put(s)
}

// SkipSpace使Scan方法能够跳过空格和换行符，以与格式字符串和Scan / Scanln设置的当前扫描模式保持一致。
// SkipSpace provides Scan methods the ability to skip space and newline
// characters in keeping with the current scanning mode set by format strings
// and Scan/Scanln.
func (s *ss) SkipSpace() {
	for {
		r := s.getRune()
		if r == eof { // 到达末尾
			return
		}
		if r == '\r' && s.peek("\n") { // "\r\n"换行
			continue
		}
		if r == '\n' { // "换行"
			if s.nlIsSpace {
				continue
			}
			s.errorString("unexpected newline")
			return
		}
		if !isSpace(r) {
			s.UnreadRune()
			break
		}
	}
}

// token从输入中返回下一个以空格分隔的字符串。 它跳过空白。 对于Scanln，它在换行符处停止。 对于Scan，换行符被视为空格。
// @param   skipSpace   忽略空格
// @param   f           匹配函数
// token returns the next space-delimited string from the input. It
// skips white space. For Scanln, it stops at newlines. For Scan,
// newlines are treated as spaces.
func (s *ss) token(skipSpace bool, f func(rune) bool) []byte {
	if skipSpace { // 忽略空格
		s.SkipSpace()
	}
    // 直到读取到了空格或者换行就停止
	// read until white space or newline 
	for {
		r := s.getRune()
		if r == eof {
			break
		}
		if !f(r) {
			s.UnreadRune()
			break
		}
		s.buf.writeRune(r)
	}
	return s.buf
}

// 复数错误
var complexError = errors.New("syntax error scanning complex number")
// bool值错误
var boolError = errors.New("syntax error scanning boolean")

// 求r在s的中位置，不存在返回-1
// @param s 字符串
// @param r 字符
// return 位置
func indexRune(s string, r rune) int {
	for i, c := range s {
		if c == r {
			return i
		}
	}
	return -1
}

// consume读取输入中的下一个符文，并报告其是否在ok字符串中。 如果accept为true，则将字符放入输入token中。
// @param ok        输入字符串
// @param accept    是否接受
// @return          是否读取
// consume reads the next rune in the input and reports whether it is in the ok string.
// If accept is true, it puts the character into the input token.
func (s *ss) consume(ok string, accept bool) bool {
	r := s.getRune()
	if r == eof {
		return false
	}
	if indexRune(ok, r) >= 0 {
		if accept {
			s.buf.writeRune(r)
		}
		return true
	}
	if r != eof && accept { // 不在字符串中，并且接口，就要还原
		s.UnreadRune()
	}
	return false
}

// peek报告下一个字符是否在ok字符串中，而不使用它。
// peek reports whether the next character is in the ok string, without consuming it.
func (s *ss) peek(ok string) bool {
	r := s.getRune()
	if r != eof {
		s.UnreadRune()
	}
	return indexRune(ok, r) >= 0
}

// 确保没有读取到末尾
func (s *ss) notEOF() {
    // 确保有要读取的数据。
	// Guarantee there is data to be read.
	if r := s.getRune(); r == eof {
		panic(io.EOF)
	}
	s.UnreadRune()
}

// accept检查输入中的下一个符文。 如果它是字符串中的字节（sic），则将其放入缓冲区并返回true。 否则返回false。
// accept checks the next rune in the input. If it's a byte (sic) in the string, it puts it in the
// buffer and returns true. Otherwise it return false.
func (s *ss) accept(ok string) bool {
	return s.consume(ok, true)
}

// okVerb验证该verb动词字符是否存在于okVerbs动词字符串中，如果不存在则适当设置s.err。
// @param verb 字符
// @parama okVerbs 字符串
// @return true：在，false：不在
// okVerb verifies that the verb is present in the list, setting s.err appropriately if not.
func (s *ss) okVerb(verb rune, okVerbs, typ string) bool {
	for _, v := range okVerbs { // 检查动词
		if v == verb {
			return true
		}
	}
	
	// 不在就设置错误
	s.errorString("bad verb '%" + string(verb) + "' for " + typ)
	return false
}

// scanBool返回下一个标记表示的布尔值。
// @param verb 动词字符
// @return 
// scanBool returns the value of the boolean represented by the next token.
func (s *ss) scanBool(verb rune) bool {
	s.SkipSpace() // 忽略空格
	s.notEOF()  // 没有处理完
	if !s.okVerb(verb, "tv", "boolean") { // 动词字符只能是t或者v
		return false
	}
	// 将各种值转换成bool类型
	// Syntax-checking a boolean is annoying. We're not fastidious about case.
	switch s.getRune() { // 取一个字符进行转换
	case '0':
		return false
	case '1':
		return true
	case 't', 'T':
		if s.accept("rR") && (!s.accept("uU") || !s.accept("eE")) {
			s.error(boolError)
		}
		return true
	case 'f', 'F':
		if s.accept("aA") && (!s.accept("lL") || !s.accept("sS") || !s.accept("eE")) {
			s.error(boolError)
		}
		return false
	}
	return false
}

// 各种进制和数学表达式要使用到的字符
// Numerical elements
const (
	binaryDigits      = "01"
	octalDigits       = "01234567"
	decimalDigits     = "0123456789"
	hexadecimalDigits = "0123456789aAbBcCdDeEfF"
	sign              = "+-"
	period            = "."
	exponent          = "eEpP"
)

// getBase返回由动词及其数字字符串表示的数字基。
// @param verb 动词
// @param 动词对应的基数
// @param 基数使用的字符串
// getBase returns the numeric base represented by the verb and its digit string.
func (s *ss) getBase(verb rune) (base int, digits string) {
	s.okVerb(verb, "bdoUxXv", "integer") // sets s.err // 检视动词，只能接受bdoUxXv类型
	base = 10 // 默认基数是10
	digits = decimalDigits
	switch verb {
	case 'b':
		base = 2
		digits = binaryDigits
	case 'o':
		base = 8
		digits = octalDigits
	case 'x', 'X', 'U':
		base = 16
		digits = hexadecimalDigits
	}
	return
}

// scanNumber返回从此时处理位置开始具有指定数字的数字字符串
// @param digits 数字字符串
// @param haveDigits 是否有数字
// @return 数字字符串
// scanNumber returns the numerical string with specified digits starting here.
func (s *ss) scanNumber(digits string, haveDigits bool) string {
	if !haveDigits {// 没有数字
		s.notEOF() // 未到末尾
		if !s.accept(digits) { 
			s.errorString("expected integer")
		}
	}
	for s.accept(digits) { // 读取字符串，直到不是数字字符
	}
	return string(s.buf) // 返回数字字符串
}

// scanRune返回输入中的下一个符文值。以int64方式返回
// @param bitSize 读取的字节数
// @return 读取到的字符，使用int64返回
// scanRune returns the next rune value in the input.
func (s *ss) scanRune(bitSize int) int64 {
	s.notEOF()
	r := int64(s.getRune()) // 读取一个字符
	n := uint(bitSize)
	x := (r << (64 - n)) >> (64 - n) // TODO
	if x != r {
		s.errorString("overflow on character value " + string(r))
	}
	return r
}

// scanBasePrefix报告整数是否以基数前缀开头并返回基数，数字字符串，以及是否找到零。 仅当动词为%v时才被调用。
// @return base 数字基数
// @return digits 基数数字字符串
// @return zeroFound 是否找到0
// scanBasePrefix reports whether the integer begins with a base prefix
// and returns the base, digit string, and whether a zero was found.
// It is called only if the verb is %v.
func (s *ss) scanBasePrefix() (base int, digits string, zeroFound bool) {
	if !s.peek("0") { // 当前位置非0开头，说明
		return 0, decimalDigits + "_", false 
	}
	s.accept("0") // 消费0
	// 处理各种进制
	// Special cases for 0, 0b, 0o, 0x.
	switch {
	case s.peek("bB"):
		s.consume("bB", true)
		return 0, binaryDigits + "_", true
	case s.peek("oO"):
		s.consume("oO", true)
		return 0, octalDigits + "_", true
	case s.peek("xX"):
		s.consume("xX", true)
		return 0, hexadecimalDigits + "_", true
	default: // 默认8进制
		return 0, octalDigits + "_", true
	}
}

// scanInt返回下一个标记表示的整数值，检查是否溢出。 任何错误都存储在s.err中。
// @param verb 动词
// @param bbitSize 读取的位数
// @return 读取到的数值
// scanInt returns the value of the integer represented by the next
// token, checking for overflow. Any error is stored in s.err.
func (s *ss) scanInt(verb rune, bitSize int) int64 {
	if verb == 'c' { // 如果c动词就读取指定位数
		return s.scanRune(bitSize)
	}
	
	// 忽略空格并检测是否到文末
	s.SkipSpace()
	s.notEOF()
	base, digits := s.getBase(verb) // 取base和base字符串
	haveDigits := false
	if verb == 'U' { // U动词
		if !s.consume("U", false) || !s.consume("+", false) { // "U+"消费出错，设置错误
			s.errorString("bad unicode format ")
		}
	} else {
		s.accept(sign) // If there's a sign, it will be left in the token buffer.// 如果有符号，它将保留在Token缓冲区中。
		if verb == 'v' { // v动词
			base, digits, haveDigits = s.scanBasePrefix() // 处理前进制的前缀
		}
	}
	// 处理数字
	tok := s.scanNumber(digits, haveDigits)
	i, err := strconv.ParseInt(tok, base, 64)
	if err != nil {
		s.error(err)
	}
	n := uint(bitSize)
	x := (i << (64 - n)) >> (64 - n)
	if x != i {
		s.errorString("integer overflow on token " + tok)
	}
	return i
}

// scanUint返回下一个标记表示的无符号整数的值，以检查是否溢出。 任何错误都存储在s.err中。
// @param verb 动词
// @param bbitSize 读取的位数
// @return 读取到的数值
// scanUint returns the value of the unsigned integer represented
// by the next token, checking for overflow. Any error is stored in s.err.
func (s *ss) scanUint(verb rune, bitSize int) uint64 {
	if verb == 'c' {
		return uint64(s.scanRune(bitSize))
	}
	s.SkipSpace()
	s.notEOF()
	base, digits := s.getBase(verb)
	haveDigits := false
	if verb == 'U' {
		if !s.consume("U", false) || !s.consume("+", false) {
			s.errorString("bad unicode format ")
		}
	} else if verb == 'v' {
		base, digits, haveDigits = s.scanBasePrefix()
	}
	tok := s.scanNumber(digits, haveDigits)
	i, err := strconv.ParseUint(tok, base, 64)
	if err != nil {
		s.error(err)
	}
	n := uint(bitSize)
	x := (i << (64 - n)) >> (64 - n)
	if x != i {
		s.errorString("unsigned integer overflow on token " + tok)
	}
	return i
}

// floatToken返回从此处开始的浮点数，如果指定了宽度，则不超过swid。它对语法并不严格，因为它不会检查我们是否至少有一些数字，但是Atof会做到这一点。
// floatToken returns the floating-point number starting here, no longer than swid
// if the width is specified. It's not rigorous about syntax because it doesn't check that
// we have at least some digits, but Atof will do that.
func (s *ss) floatToken() string {
	s.buf = s.buf[:0]
	// 非数字处理
	// NaN?
	if s.accept("nN") && s.accept("aA") && s.accept("nN") {
		return string(s.buf)
	}
	// 有符号位
	// leading sign?
	s.accept(sign)
	// 无穷数
	// Inf?
	if s.accept("iI") && s.accept("nN") && s.accept("fF") {
		return string(s.buf)
	}
	
	// 10进制，数字和_
	digits := decimalDigits + "_"
	exp := exponent
	if s.accept("0") && s.accept("xX") { // 16进制
		digits = hexadecimalDigits + "_"
		exp = "pP" // 指数
	}
	
	// 处理系数的整数部分（a*10^b），a是系数，10是底数，b是指数
	// digits?
	for s.accept(digits) {
	}
	// 处理小数点
	// decimal point?
	if s.accept(period) {
	    // 处理小数部分
		// fraction?
		for s.accept(digits) {
		}
	}
	// 处理指数部分
	// exponent?
	if s.accept(exp) {
	    // 处理指数符号位
		// leading sign?
		s.accept(sign)
		// 处理指数值
		// digits?
		for s.accept(decimalDigits + "_") {
		}
	}
	return string(s.buf)
}

// complexTokens返回从此处开始的复数的实部和虚部。数字可能带有括号并具有（N + Ni）格式，其中N是浮点数并且其中没有空格。
// return real 实部字符串
// return imag 虚部字符串
// complexTokens returns the real and imaginary parts of the complex number starting here.
// The number might be parenthesized and has the format (N+Ni) where N is a floating-point
// number and there are no spaces within.
func (s *ss) complexTokens() (real, imag string) {
	// TODO: accept N and Ni independently?
	parens := s.accept("(")
	real = s.floatToken()
	s.buf = s.buf[:0]
	// 必须要有符号位
	// Must now have a sign.
	if !s.accept("+-") {
		s.error(complexError)
	}
	// 虚部符号
	// Sign is now in buffer
	imagSign := string(s.buf)
	imag = s.floatToken()
	if !s.accept("i") { // 虚部标识
		s.error(complexError)
	}
	if parens && !s.accept(")") { // 最后处理括号
		s.error(complexError)
	}
	return real, imagSign + imag
}

// 字符串中是否包含x不分大小写
func hasX(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == 'x' || s[i] == 'X' {
			return true
		}
	}
	return false
}

// convertFloat将字符串转换为float64值。
// @param str 数字字符串
// @param n 基数
// @param n 浮点值
// convertFloat converts the string to a float64value.
func (s *ss) convertFloat(str string, n int) float64 {
    // strconv.ParseFloat将处理"+0x1.fp+2"，但是我们必须自己实现非标准的十进制+二进制指数混合（1.2p4）。
	// strconv.ParseFloat will handle "+0x1.fp+2",
	// but we have to implement our non-standard
	// decimal+binary exponent mix (1.2p4) ourselves.
	if p := indexRune(str, 'p'); p >= 0 && !hasX(str) { // 字符串中有p并且有x
	    //Atof不处理2的幂的指数，但它们很容易评估。
		// Atof doesn't handle power-of-2 exponents,
		// but they're easy to evaluate.
		f, err := strconv.ParseFloat(str[:p], n) // 使用指定基本数进行处理
		if err != nil {
		    // 如果有错误，并且是*strconv.NumError类型，就将待处理的字符串保存到错误中
			// Put full string into error.
			if e, ok := err.(*strconv.NumError); ok {
				e.Num = str
			}
			s.error(err)
		}
		m, err := strconv.Atoi(str[p+1:]) // 使用10进制再次处理
		if err != nil { // 出错
			// Put full string into error.
			if e, ok := err.(*strconv.NumError); ok {
				e.Num = str
			}
			s.error(err)
		}
		
		// 最后使用数学库进行处理
		return math.Ldexp(f, m)
	}

	f, err := strconv.ParseFloat(str, n)
	if err != nil {
		s.error(err)
	}
	return f
}

// convertComplex将下一个标记转换为complex128值。 atof参数是基础类型的特定于类型的读取器。如果我们正在阅读complex64，则atof将解析float32，并将其转换为float64，以避免为每种复杂类型重现此代码。
// @param verb 动词字符
// @parma n 基数
// return 位数，64或者128
// convertComplex converts the next token to a complex128 value.
// The atof argument is a type-specific reader for the underlying type.
// If we're reading complex64, atof will parse float32s and convert them
// to float64's to avoid reproducing this code for each complex type.
func (s *ss) scanComplex(verb rune, n int) complex128 {
    // 如果不是浮点数，直接返回
	if !s.okVerb(verb, floatVerbs, "complex") {
		return 0
	}
	s.SkipSpace()
	s.notEOF()
	sreal, simag := s.complexTokens()
	real := s.convertFloat(sreal, n/2)
	imag := s.convertFloat(simag, n/2)
	return complex(real, imag)
}

// convertString返回下一个输入字符表示的字符串。 输入的格式由动词确定。
// @param verb 格式动词
// @return 格式化后的字符串
// convertString returns the string represented by the next input characters.
// The format of the input is determined by the verb.
func (s *ss) convertString(verb rune) (str string) {
	if !s.okVerb(verb, "svqxX", "string") { // 检察动词是否合法，不合法直接返回
		return ""
	}
	s.SkipSpace()
	s.notEOF()
	switch verb {
	case 'q': 
		str = s.quotedString()
	case 'x', 'X':
		str = s.hexString()
	default:
		str = string(s.token(true, notSpace)) // %s and %v just return the next word // ％s和％v仅返回下一个单词
	}
	return
}

// quotedString returns the double- or back-quoted string represented by the next input characters.
func (s *ss) quotedString() string {
	s.notEOF()
	quote := s.getRune()
	switch quote {
	case '`':
		// Back-quoted: Anything goes until EOF or back quote.
		for {
			r := s.mustReadRune()
			if r == quote {
				break
			}
			s.buf.writeRune(r)
		}
		return string(s.buf)
	case '"':
		// Double-quoted: Include the quotes and let strconv.Unquote do the backslash escapes.
		s.buf.writeByte('"')
		for {
			r := s.mustReadRune()
			s.buf.writeRune(r)
			if r == '\\' {
				// In a legal backslash escape, no matter how long, only the character
				// immediately after the escape can itself be a backslash or quote.
				// Thus we only need to protect the first character after the backslash.
				s.buf.writeRune(s.mustReadRune())
			} else if r == '"' {
				break
			}
		}
		result, err := strconv.Unquote(string(s.buf))
		if err != nil {
			s.error(err)
		}
		return result
	default:
		s.errorString("expected quoted string")
	}
	return ""
}

// hexDigit returns the value of the hexadecimal digit.
func hexDigit(d rune) (int, bool) {
	digit := int(d)
	switch digit {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return digit - '0', true
	case 'a', 'b', 'c', 'd', 'e', 'f':
		return 10 + digit - 'a', true
	case 'A', 'B', 'C', 'D', 'E', 'F':
		return 10 + digit - 'A', true
	}
	return -1, false
}

// hexByte returns the next hex-encoded (two-character) byte from the input.
// It returns ok==false if the next bytes in the input do not encode a hex byte.
// If the first byte is hex and the second is not, processing stops.
func (s *ss) hexByte() (b byte, ok bool) {
	rune1 := s.getRune()
	if rune1 == eof {
		return
	}
	value1, ok := hexDigit(rune1)
	if !ok {
		s.UnreadRune()
		return
	}
	value2, ok := hexDigit(s.mustReadRune())
	if !ok {
		s.errorString("illegal hex digit")
		return
	}
	return byte(value1<<4 | value2), true
}

// hexString returns the space-delimited hexpair-encoded string.
func (s *ss) hexString() string {
	s.notEOF()
	for {
		b, ok := s.hexByte()
		if !ok {
			break
		}
		s.buf.writeByte(b)
	}
	if len(s.buf) == 0 {
		s.errorString("no hex data for %x string")
		return ""
	}
	return string(s.buf)
}

const (
	floatVerbs = "beEfFgGv"

	hugeWid = 1 << 30

	intBits     = 32 << (^uint(0) >> 63)
	uintptrBits = 32 << (^uintptr(0) >> 63)
)

// scanPercent scans a literal percent character.
func (s *ss) scanPercent() {
	s.SkipSpace()
	s.notEOF()
	if !s.accept("%") {
		s.errorString("missing literal %")
	}
}

// scanOne scans a single value, deriving the scanner from the type of the argument.
func (s *ss) scanOne(verb rune, arg interface{}) {
	s.buf = s.buf[:0]
	var err error
	// If the parameter has its own Scan method, use that.
	if v, ok := arg.(Scanner); ok {
		err = v.Scan(s, verb)
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			s.error(err)
		}
		return
	}

	switch v := arg.(type) {
	case *bool:
		*v = s.scanBool(verb)
	case *complex64:
		*v = complex64(s.scanComplex(verb, 64))
	case *complex128:
		*v = s.scanComplex(verb, 128)
	case *int:
		*v = int(s.scanInt(verb, intBits))
	case *int8:
		*v = int8(s.scanInt(verb, 8))
	case *int16:
		*v = int16(s.scanInt(verb, 16))
	case *int32:
		*v = int32(s.scanInt(verb, 32))
	case *int64:
		*v = s.scanInt(verb, 64)
	case *uint:
		*v = uint(s.scanUint(verb, intBits))
	case *uint8:
		*v = uint8(s.scanUint(verb, 8))
	case *uint16:
		*v = uint16(s.scanUint(verb, 16))
	case *uint32:
		*v = uint32(s.scanUint(verb, 32))
	case *uint64:
		*v = s.scanUint(verb, 64)
	case *uintptr:
		*v = uintptr(s.scanUint(verb, uintptrBits))
	// Floats are tricky because you want to scan in the precision of the result, not
	// scan in high precision and convert, in order to preserve the correct error condition.
	case *float32:
		if s.okVerb(verb, floatVerbs, "float32") {
			s.SkipSpace()
			s.notEOF()
			*v = float32(s.convertFloat(s.floatToken(), 32))
		}
	case *float64:
		if s.okVerb(verb, floatVerbs, "float64") {
			s.SkipSpace()
			s.notEOF()
			*v = s.convertFloat(s.floatToken(), 64)
		}
	case *string:
		*v = s.convertString(verb)
	case *[]byte:
		// We scan to string and convert so we get a copy of the data.
		// If we scanned to bytes, the slice would point at the buffer.
		*v = []byte(s.convertString(verb))
	default:
		val := reflect.ValueOf(v)
		ptr := val
		if ptr.Kind() != reflect.Ptr {
			s.errorString("type not a pointer: " + val.Type().String())
			return
		}
		switch v := ptr.Elem(); v.Kind() {
		case reflect.Bool:
			v.SetBool(s.scanBool(verb))
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v.SetInt(s.scanInt(verb, v.Type().Bits()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			v.SetUint(s.scanUint(verb, v.Type().Bits()))
		case reflect.String:
			v.SetString(s.convertString(verb))
		case reflect.Slice:
			// For now, can only handle (renamed) []byte.
			typ := v.Type()
			if typ.Elem().Kind() != reflect.Uint8 {
				s.errorString("can't scan type: " + val.Type().String())
			}
			str := s.convertString(verb)
			v.Set(reflect.MakeSlice(typ, len(str), len(str)))
			for i := 0; i < len(str); i++ {
				v.Index(i).SetUint(uint64(str[i]))
			}
		case reflect.Float32, reflect.Float64:
			s.SkipSpace()
			s.notEOF()
			v.SetFloat(s.convertFloat(s.floatToken(), v.Type().Bits()))
		case reflect.Complex64, reflect.Complex128:
			v.SetComplex(s.scanComplex(verb, v.Type().Bits()))
		default:
			s.errorString("can't scan type: " + val.Type().String())
		}
	}
}

// errorHandler turns local panics into error returns.
func errorHandler(errp *error) {
	if e := recover(); e != nil {
		if se, ok := e.(scanError); ok { // catch local error
			*errp = se.err
		} else if eof, ok := e.(error); ok && eof == io.EOF { // out of input
			*errp = eof
		} else {
			panic(e)
		}
	}
}

// doScan does the real work for scanning without a format string.
func (s *ss) doScan(a []interface{}) (numProcessed int, err error) {
	defer errorHandler(&err)
	for _, arg := range a {
		s.scanOne('v', arg)
		numProcessed++
	}
	// Check for newline (or EOF) if required (Scanln etc.).
	if s.nlIsEnd {
		for {
			r := s.getRune()
			if r == '\n' || r == eof {
				break
			}
			if !isSpace(r) {
				s.errorString("expected newline")
				break
			}
		}
	}
	return
}

// advance determines whether the next characters in the input match
// those of the format. It returns the number of bytes (sic) consumed
// in the format. All runs of space characters in either input or
// format behave as a single space. Newlines are special, though:
// newlines in the format must match those in the input and vice versa.
// This routine also handles the %% case. If the return value is zero,
// either format starts with a % (with no following %) or the input
// is empty. If it is negative, the input did not match the string.
func (s *ss) advance(format string) (i int) {
	for i < len(format) {
		fmtc, w := utf8.DecodeRuneInString(format[i:])

		// Space processing.
		// In the rest of this comment "space" means spaces other than newline.
		// Newline in the format matches input of zero or more spaces and then newline or end-of-input.
		// Spaces in the format before the newline are collapsed into the newline.
		// Spaces in the format after the newline match zero or more spaces after the corresponding input newline.
		// Other spaces in the format match input of one or more spaces or end-of-input.
		if isSpace(fmtc) {
			newlines := 0
			trailingSpace := false
			for isSpace(fmtc) && i < len(format) {
				if fmtc == '\n' {
					newlines++
					trailingSpace = false
				} else {
					trailingSpace = true
				}
				i += w
				fmtc, w = utf8.DecodeRuneInString(format[i:])
			}
			for j := 0; j < newlines; j++ {
				inputc := s.getRune()
				for isSpace(inputc) && inputc != '\n' {
					inputc = s.getRune()
				}
				if inputc != '\n' && inputc != eof {
					s.errorString("newline in format does not match input")
				}
			}
			if trailingSpace {
				inputc := s.getRune()
				if newlines == 0 {
					// If the trailing space stood alone (did not follow a newline),
					// it must find at least one space to consume.
					if !isSpace(inputc) && inputc != eof {
						s.errorString("expected space in input to match format")
					}
					if inputc == '\n' {
						s.errorString("newline in input does not match format")
					}
				}
				for isSpace(inputc) && inputc != '\n' {
					inputc = s.getRune()
				}
				if inputc != eof {
					s.UnreadRune()
				}
			}
			continue
		}

		// Verbs.
		if fmtc == '%' {
			// % at end of string is an error.
			if i+w == len(format) {
				s.errorString("missing verb: % at end of format string")
			}
			// %% acts like a real percent
			nextc, _ := utf8.DecodeRuneInString(format[i+w:]) // will not match % if string is empty
			if nextc != '%' {
				return
			}
			i += w // skip the first %
		}

		// Literals.
		inputc := s.mustReadRune()
		if fmtc != inputc {
			s.UnreadRune()
			return -1
		}
		i += w
	}
	return
}

// doScanf does the real work when scanning with a format string.
// At the moment, it handles only pointers to basic types.
func (s *ss) doScanf(format string, a []interface{}) (numProcessed int, err error) {
	defer errorHandler(&err)
	end := len(format) - 1
	// We process one item per non-trivial format
	for i := 0; i <= end; {
		w := s.advance(format[i:])
		if w > 0 {
			i += w
			continue
		}
		// Either we failed to advance, we have a percent character, or we ran out of input.
		if format[i] != '%' {
			// Can't advance format. Why not?
			if w < 0 {
				s.errorString("input does not match format")
			}
			// Otherwise at EOF; "too many operands" error handled below
			break
		}
		i++ // % is one byte

		// do we have 20 (width)?
		var widPresent bool
		s.maxWid, widPresent, i = parsenum(format, i, end)
		if !widPresent {
			s.maxWid = hugeWid
		}

		c, w := utf8.DecodeRuneInString(format[i:])
		i += w

		if c != 'c' {
			s.SkipSpace()
		}
		if c == '%' {
			s.scanPercent()
			continue // Do not consume an argument.
		}
		s.argLimit = s.limit
		if f := s.count + s.maxWid; f < s.argLimit {
			s.argLimit = f
		}

		if numProcessed >= len(a) { // out of operands
			s.errorString("too few operands for format '%" + format[i-w:] + "'")
			break
		}
		arg := a[numProcessed]

		s.scanOne(c, arg)
		numProcessed++
		s.argLimit = s.limit
	}
	if numProcessed < len(a) {
		s.errorString("too many operands")
	}
	return
}
```