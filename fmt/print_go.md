
```go
package fmt

import (
	"internal/fmtsort"
	"io"
	"os"
	"reflect"
	"sync"
	"unicode/utf8"
)

// 与buffer.WriteString一起使用的字符串。 这比使用buffer.Write与字节数组的开销要少。
// Strings for use with buffer.WriteString.
// This is less overhead than using buffer.Write with byte arrays.
// 这里定义了一些字符串格式和常量
const (
	commaSpaceString  = ", "
	nilAngleString    = "<nil>"
	nilParenString    = "(nil)"
	nilString         = "nil"
	mapString         = "map["
	percentBangString = "%!"
	missingString     = "(MISSING)"
	badIndexString    = "(BADINDEX)"
	panicString       = "(PANIC="
	extraString       = "%!(EXTRA "
	badWidthString    = "%!(BADWIDTH)"
	badPrecString     = "%!(BADPREC)"
	noVerbString      = "%!(NOVERB)"
	invReflectString  = "<invalid reflect.Value>"
)

// State代表一个传递给自定义Formatter接口的Format方法的打印环境。它实现了io.Writer接口用来写入格式化的文本，还提供了该操作数的格式字符串指定的选项和宽度、精度信息（通过调用方法）
// State represents the printer state passed to custom formatters.
// It provides access to the io.Writer interface plus information about
// the flags and options for the operand's format specifier.
type State interface {
    // // Write方法用来写入格式化的文本
	// Write is the function to call to emit formatted output to be printed.
	Write(b []byte) (n int, err error)
	// Width返回宽度值，及其是否被设置
	// Width returns the value of the width option and whether it has been set.
	Width() (wid int, ok bool)
	// Precision返回精度值，及其是否被设置
	// Precision returns the value of the precision option and whether it has been set.
	Precision() (prec int, ok bool)

    // Flag报告是否设置了标志， c代表（一个字符，如+、-、#等）
	// Flag reports whether the flag c, a character, has been set.
	Flag(c int) bool
}

// 实现了Formatter接口的类型可以定制自己的格式化输出。Format方法的实现内部可以调用Sprint(f)或Fprint(f)等函数来生成自身的输出。
// Formatter is the interface implemented by values with a custom formatter.
// The implementation of Format may call Sprint(f) or Fprint(f) etc.
// to generate its output.
type Formatter interface {
	Format(f State, c rune)
}

// 实现了Stringer接口的类型（即有String方法），定义了该类型值的原始显示。当采用任何接受字符的verb（%v %s %q %x %X）动作格式化一个操作数时，或者被不使用格式字符串如Print函数打印操作数时，会调用String方法来生成输出的文本。
// Stringer is implemented by any value that has a String method,
// which defines the ``native'' format for that value.
// The String method is used to print values passed as an operand
// to any format that accepts a string or to an unformatted printer
// such as Print.
type Stringer interface {
	String() string
}

// 实现了GoStringer接口的类型（即有GoString方法），定义了该类型值的go语法表示。当采用verb %#v格式化一个操作数时，会调用GoString方法来生成输出的文本。
// GoStringer is implemented by any value that has a GoString method,
// which defines the Go syntax for that value.
// The GoString method is used to print values passed as an operand
// to a %#v format.
type GoStringer interface {
	GoString() string
}

// 使用简单的[] byte而不是bytes.Buffer可以避免较大的依赖性。
// Use simple []byte instead of bytes.Buffer to avoid large dependency.
type buffer []byte

// 向字节数组中添加字节数组
func (b *buffer) write(p []byte) {
	*b = append(*b, p...)
}

// 向字节数组中添加字符串
func (b *buffer) writeString(s string) {
	*b = append(*b, s...)
}

// 向字节数组中添加单个字节
func (b *buffer) writeByte(c byte) {
	*b = append(*b, c)
}

// 向字节数组中添加单个字符
func (bp *buffer) writeRune(r rune) {
    // 单字字字符
	if r < utf8.RuneSelf {
		*bp = append(*bp, byte(r))
		return
	}

    // 多字节字符，需要先扩容
	b := *bp
	n := len(b)
	for n+utf8.UTFMax > cap(b) { // 可以一次性添加utf8.UTFMax个字节
		b = append(b, 0)
	}
	w := utf8.EncodeRune(b[n:n+utf8.UTFMax], r) // 写入字符，并且返回写入的字节数
	*bp = b[:n+w] // 取真正的有效的数据
}

// pp用于存储打印的状态，并与sync.Pool一起重用使用以避免重新分配。
// pp is used to store a printer's state and is reused with sync.Pool to avoid allocations.
type pp struct {
	buf buffer

    // arg以interface{}形式保存当前值
	// arg holds the current item, as an interface{}.
	arg interface{}

    // 使用value而不是arg来代表反射值。
	// value is used instead of arg for reflect values.
	value reflect.Value

    // fmt用于格式化基础数据，例如整数或字符串。
	// fmt is used to format basic items such as integers or strings.
	fmt fmt

    // 重新排序记录格式字符串是否使用了参数重新排序。
	// reordered records whether the format string used argument reordering.
	reordered bool
	// goodArgNum记录最近的重新排序指令是否有效。
	// goodArgNum records whether the most recent reordering directive was valid.
	goodArgNum bool
	// catchPanic设置恐慌，以避免无限恐慌，恢复，恐慌，...
	// panicking is set by catchPanic to avoid infinite panic, recover, panic, ... recursion.
	panicking bool
	// 当打印错误字符串以防止调用handleMethods时。erroring被设置
	// erroring is set when printing an error string to guard against calling handleMethods.
	erroring bool
	// 当格式字符串可能包含％w动词时，将设置wrapErrs。
	// wrapErrs is set when the format string may contain a %w verb.
	wrapErrs bool
	// appedErr记录％w动词的目标。
	// wrappedErr records the target of the %w verb.
	wrappedErr error
}

var ppFree = sync.Pool{
	New: func() interface{} { return new(pp) },
}

// newPrinter分配一个新的pp结构或获取一个缓存的pp结构。
// newPrinter allocates a new pp struct or grabs a cached one.
func newPrinter() *pp {
    // 因为p有可能是从缓存中取的，归还时可能没有还原，这里再设置一下值
	p := ppFree.Get().(*pp)
	p.panicking = false
	p.erroring = false
	p.wrapErrs = false
	p.fmt.init(&p.buf)
	return p
}

// free将使用过的pp结构保存在ppFree中； 避免每次调用分配。
// free saves used pp structs in ppFree; avoids an allocation per invocation.
func (p *pp) free() {
    // 正确使用sync.Pool要求每个条目具有大约相同的内存成本。为了在存储的类型包含可变大小的缓冲区时获得此属性，我们对最大缓冲区添加了硬限制以放回池中。
	// Proper usage of a sync.Pool requires each entry to have approximately
	// the same memory cost. To obtain this property when the stored type
	// contains a variably-sized buffer, we add a hard limit on the maximum buffer
	// to place back in the pool.
	//
	// See https://golang.org/issue/23199
	if cap(p.buf) > 64<<10 { // 缓存不能大于64KB，大于64KB的就会被系统当作垃圾回收
		return
	}

    // 在这里还原部分数据，并非所有的字段都还原
	p.buf = p.buf[:0]
	p.arg = nil
	p.value = reflect.Value{}
	p.wrappedErr = nil
	ppFree.Put(p)
}

// 取宽度及其标记
func (p *pp) Width() (wid int, ok bool) { return p.fmt.wid, p.fmt.widPresent }

// 取精度及其标记
func (p *pp) Precision() (prec int, ok bool) { return p.fmt.prec, p.fmt.precPresent }

// 判断标记是否存在
func (p *pp) Flag(b int) bool {
	switch b {
	case '-':
		return p.fmt.minus
	case '+':
		return p.fmt.plus || p.fmt.plusV
	case '#':
		return p.fmt.sharp || p.fmt.sharpV
	case ' ':
		return p.fmt.space
	case '0':
		return p.fmt.zero
	}
	return false
}

// 实现Write，以便我们可以在pp上（通过State）调用Fprintf，也可在自定义动词中递归使用。
// Implement Write so we can call Fprintf on a pp (through State), for
// recursive use in custom verbs.
func (p *pp) Write(b []byte) (ret int, err error) {
	p.buf.write(b)
	return len(b), nil
}

// 实现WriteString，以便我们可以在pp（直通状态）上调用io.WriteString，以提高效率。
// Implement WriteString so that we can call io.WriteString
// on a pp (through state), for efficiency.
func (p *pp) WriteString(s string) (ret int, err error) {
	p.buf.writeString(s)
	return len(s), nil
}

// routines可能理解成method，即方法。
// 这些例程以"f"结尾并采用格式字符串。 
// These routines end in 'f' and take a format string.

// NOTE: 调用写入方法，如果中途出错，会有数据写入到buf，但不会还原

// Fprintf根据格式说明符格式化并写入w。 它返回已写入的字节数以及遇到的任何写入错误。
// Fprintf formats according to a format specifier and writes to w.
// It returns the number of bytes written and any write error encountered.
func Fprintf(w io.Writer, format string, a ...interface{}) (n int, err error) {
	p := newPrinter()
	p.doPrintf(format, a)
	n, err = w.Write(p.buf)
	p.free()
	return
}

// Printf根据格式说明符设置格式并写入标准输出。 它返回已写入的字节数以及遇到的任何写入错误。
// Printf formats according to a format specifier and writes to standard output.
// It returns the number of bytes written and any write error encountered.
func Printf(format string, a ...interface{}) (n int, err error) {
	return Fprintf(os.Stdout, format, a...)
}

// Sprintf根据格式说明符设置格式，并返回结果字符串。
// Sprintf formats according to a format specifier and returns the resulting string.
func Sprintf(format string, a ...interface{}) string {
	p := newPrinter()
	p.doPrintf(format, a)
	s := string(p.buf)
	p.free()
	return s
}

// 这些例程不采用格式字符串
// These routines do not take a format string

// Fprint formats using the default formats for its operands and writes to w.
// Spaces are added between operands when neither is a string.
// It returns the number of bytes written and any write error encountered.
func Fprint(w io.Writer, a ...interface{}) (n int, err error) {
	p := newPrinter()
	p.doPrint(a)
	n, err = w.Write(p.buf)
	p.free()
	return
}

// Fprint的操作数使用默认格式进行格式化并写入w。 当都不是字符串时，在操作数之间添加空格。
// Print formats using the default formats for its operands and writes to standard output.
// Spaces are added between operands when neither is a string.
// It returns the number of bytes written and any write error encountered.
func Print(a ...interface{}) (n int, err error) {
	return Fprint(os.Stdout, a...)
}

// 使用默认格式为其操作数设置Sprint格式，并返回结果字符串。 当都不是字符串时，在操作数之间添加空格。
// Sprint formats using the default formats for its operands and returns the resulting string.
// Spaces are added between operands when neither is a string.
func Sprint(a ...interface{}) string {
	p := newPrinter()
	p.doPrint(a)
	s := string(p.buf)
	p.free()
	return s
}

// 这些例程以“ ln”结尾，不使用格式字符串，始终在操作数之间添加空格，并在最后一个操作数之后添加换行符。
// These routines end in 'ln', do not take a format string,
// always add spaces between operands, and add a newline
// after the last operand.

// Fprintln格式使用其操作数的默认格式并写入w。 始终在操作数之间添加空格，并添加换行符。 它返回已写入的字节数以及遇到的任何写入错误。
// Fprintln formats using the default formats for its operands and writes to w.
// Spaces are always added between operands and a newline is appended.
// It returns the number of bytes written and any write error encountered.
func Fprintln(w io.Writer, a ...interface{}) (n int, err error) {
	p := newPrinter()
	p.doPrintln(a)
	n, err = w.Write(p.buf)
	p.free()
	return
}

// Println格式使用其操作数的默认格式并写入标准输出。 始终在操作数之间添加空格，并添加换行符。 它返回已写入的字节数以及遇到的任何写入错误。
// Println formats using the default formats for its operands and writes to standard output.
// Spaces are always added between operands and a newline is appended.
// It returns the number of bytes written and any write error encountered.
func Println(a ...interface{}) (n int, err error) {
	return Fprintln(os.Stdout, a...)
}

// Sprintln使用其操作数的默认格式进行格式化，并返回结果字符串。 始终在操作数之间添加空格，并添加换行符。
// Sprintln formats using the default formats for its operands and returns the resulting string.
// Spaces are always added between operands and a newline is appended.
func Sprintln(a ...interface{}) string {
	p := newPrinter()
	p.doPrintln(a)
	s := string(p.buf)
	p.free()
	return s
}

// getField获取结构体值的第i个字段。 如果字段本身是接口，则返回接口内部的值，而不是接口本身。
// getField gets the i'th field of the struct value.
// If the field is itself is an interface, return a value for
// the thing inside the interface, not the interface itself.
func getField(v reflect.Value, i int) reflect.Value {
	val := v.Field(i) // 取第i个字段
	if val.Kind() == reflect.Interface && !val.IsNil() { // 字段是接口，并且非空
		val = val.Elem()
	}
	return val
}

// tooLarge报告整数的大小是否太大而不能用作格式化宽度或精度。
// tooLarge reports whether the magnitude of the integer is
// too large to be used as a formatting width or precision.
func tooLarge(x int) bool {
	const max int = 1e6
	return x > max || x < -max
}

// parsenum将ASCII转换为整数。 如果没有数字，则num为0（且isnum为false）。
// parsenum converts ASCII to integer.  num is 0 (and isnum is false) if no number present.
func parsenum(s string, start, end int) (num int, isnum bool, newi int) {
	if start >= end {
		return 0, false, end
	}
	for newi = start; newi < end && '0' <= s[newi] && s[newi] <= '9'; newi++ {
		if tooLarge(num) {
		    // 溢出了
			return 0, false, end // Overflow; crazy long number most likely. 
		}
		num = num*10 + int(s[newi]-'0')
		isnum = true
	}
	return
}

// 写入反射类型
func (p *pp) unknownType(v reflect.Value) {
    // 如果类型无效就写入"<nil>"
	if !v.IsValid() {
		p.buf.writeString(nilAngleString)
		return
	}
	
	// 如果类型有效，就写入"?类型字符串?"
	p.buf.writeByte('?')
	p.buf.writeString(v.Type().String())
	p.buf.writeByte('?')
}

// 动词错误
func (p *pp) badVerb(verb rune) {
	p.erroring = true // 标记有错误
	p.buf.writeString(percentBangString) // 写入"%!"
	p.buf.writeRune(verb) // 写入字符
	p.buf.writeByte('(') // 写左括号
	switch {
	case p.arg != nil:  // 参数不为空 ==> argTypeString=argValue
		p.buf.writeString(reflect.TypeOf(p.arg).String()) 
		p.buf.writeByte('=')    
		p.printArg(p.arg, 'v') 
	case p.value.IsValid(): // 值有效 ==> valueTypeString=valueString
		p.buf.writeString(p.value.Type().String())
		p.buf.writeByte('=')
		p.printValue(p.value, 'v', 0)
	default: // 默认 ==> "<nil>"
		p.buf.writeString(nilAngleString)
	}
	p.buf.writeByte(')')
	p.erroring = false // 还原
}

// 格式化bool类型
func (p *pp) fmtBool(v bool, verb rune) {
	switch verb {
	case 't', 'v': // 只有t,动词是合法的
		p.fmt.fmtBoolean(v)
	default:
		p.badVerb(verb)
	}
}

// fmt0x64会以十六进制格式格式化uint64，并根据要求通过临时设置sharp标志，为uint64加上或不加0x作为前缀。
// fmt0x64 formats a uint64 in hexadecimal and prefixes it with 0x or
// not, as requested, by temporarily setting the sharp flag.
func (p *pp) fmt0x64(v uint64, leading0x bool) {
	sharp := p.fmt.sharp
	p.fmt.sharp = leading0x // 是否加上前导0x
	p.fmt.fmtInteger(v, 16, unsigned, 'v', ldigits)
	p.fmt.sharp = sharp
}

// fmtInteger格式化有符号或无符号整数。
// fmtInteger formats a signed or unsigned integer.
func (p *pp) fmtInteger(v uint64, isSigned bool, verb rune) {
	switch verb {
	case 'v': 
		if p.fmt.sharpV && !isSigned { // 有#标记，并且是无符号数
			p.fmt0x64(v, true)
		} else {
			p.fmt.fmtInteger(v, 10, isSigned, verb, ldigits) // 以10进制的方式进行格式化
		}
	case 'd': // 10进制
		p.fmt.fmtInteger(v, 10, isSigned, verb, ldigits)
	case 'b': // 二进制
		p.fmt.fmtInteger(v, 2, isSigned, verb, ldigits) 
	case 'o', 'O': // 8进制
		p.fmt.fmtInteger(v, 8, isSigned, verb, ldigits)
	case 'x': // 小写16进制
		p.fmt.fmtInteger(v, 16, isSigned, verb, ldigits)
	case 'X': // 大写16进制
		p.fmt.fmtInteger(v, 16, isSigned, verb, udigits)
	case 'c': // unicode码
		p.fmt.fmtC(v)
	case 'q': // 单引号括起来的go语法字符字面值
		if v <= utf8.MaxRune { // 合法的unicode字符
			p.fmt.fmtQc(v)
		} else {
			p.badVerb(verb)
		}
	case 'U': // 表示为Unicode格式：U+1234，等价于"U+%04X"
		p.fmt.fmtUnicode(v)
	default:
		p.badVerb(verb)
	}
}

// fmtFloat格式化浮点数。 将每个动词的默认精度指定为fmt_float调用中的最后一个参数。
// fmtFloat formats a float. The default precision for each verb
// is specified as last argument in the call to fmt_float.
func (p *pp) fmtFloat(v float64, size int, verb rune) {
	switch verb {
	case 'v': 
		p.fmt.fmtFloat(v, size, 'g', -1)
	case 'b', 'g', 'G', 'x', 'X':
		p.fmt.fmtFloat(v, size, verb, -1)
	case 'f', 'e', 'E':
		p.fmt.fmtFloat(v, size, verb, 6)
	case 'F':
		p.fmt.fmtFloat(v, size, 'f', 6)
	default:
		p.badVerb(verb)
	}
}

// fmtComplex使用fmtFloat对r和j进行格式化，将r = real(v)和j = imag(v)的复数v格式化为(r + ji)。
// fmtComplex formats a complex number v with
// r = real(v) and j = imag(v) as (r+ji) using
// fmtFloat for r and j formatting.
func (p *pp) fmtComplex(v complex128, size int, verb rune) {
    // 确保在调用fmtFloat之前找到任何不受支持的动词，以免产生错误的错误字符串。
	// Make sure any unsupported verbs are found before the
	// calls to fmtFloat to not generate an incorrect error string.
	switch verb {
	case 'v', 'b', 'g', 'G', 'x', 'X', 'f', 'F', 'e', 'E': // float格式式化支持的类型
		oldPlus := p.fmt.plus
		p.buf.writeByte('(')
		p.fmtFloat(real(v), size/2, verb)
		// 虚部总是有符号位
		// Imaginary part always has a sign.
		p.fmt.plus = true
		p.fmtFloat(imag(v), size/2, verb)
		p.buf.writeString("i)")
		p.fmt.plus = oldPlus
	default:
		p.badVerb(verb)
	}
}

// 字符串格式化
func (p *pp) fmtString(v string, verb rune) {
	switch verb {
	case 'v':
		if p.fmt.sharpV {
			p.fmt.fmtQ(v)
		} else {
			p.fmt.fmtS(v)
		}
	case 's':
		p.fmt.fmtS(v)
	case 'x':
		p.fmt.fmtSx(v, ldigits)
	case 'X':
		p.fmt.fmtSx(v, udigits)
	case 'q':
		p.fmt.fmtQ(v)
	default:
		p.badVerb(verb)
	}
}

// 字节数组格式化
func (p *pp) fmtBytes(v []byte, verb rune, typeString string) {
	switch verb {
	case 'v', 'd':
		if p.fmt.sharpV {// 有#标记
			p.buf.writeString(typeString)
			if v == nil {
				p.buf.writeString(nilParenString)
				return
			}
			p.buf.writeByte('{')
			for i, c := range v {
				if i > 0 {
					p.buf.writeString(commaSpaceString)
				}
				p.fmt0x64(uint64(c), true)
			}
			p.buf.writeByte('}')
		} else { // 无#标记
			p.buf.writeByte('[')
			for i, c := range v {
				if i > 0 {
					p.buf.writeByte(' ')
				}
				p.fmt.fmtInteger(uint64(c), 10, unsigned, verb, ldigits)
			}
			p.buf.writeByte(']')
		}
	case 's':
		p.fmt.fmtBs(v)
	case 'x':
		p.fmt.fmtBx(v, ldigits)
	case 'X':
		p.fmt.fmtBx(v, udigits)
	case 'q':
		p.fmt.fmtQ(string(v))
	default:
		p.printValue(reflect.ValueOf(v), verb, 0)
	}
}

// 格式化指针
func (p *pp) fmtPointer(value reflect.Value, verb rune) {
	var u uintptr
	switch value.Kind() { // 只能通道，函数，map，指针，切片，UnsafePointer有有指针值
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer:
		u = value.Pointer()
	default:
		p.badVerb(verb)
		return
	}

	switch verb {
	case 'v':
		if p.fmt.sharpV {// 有#标记， ==> "(uTypeString)(0x????????????????)" 或者  "(uTypeString)(<nil>)"
			p.buf.writeByte('(')
			p.buf.writeString(value.Type().String())
			p.buf.writeString(")(")
			if u == 0 {
				p.buf.writeString(nilString)
			} else {
				p.fmt0x64(uint64(u), true)
			}
			p.buf.writeByte(')')
		} else {
			if u == 0 { // 无#标记， ==> "0x????????????????" 或者  "<nil>"
				p.fmt.padString(nilAngleString)
			} else {
				p.fmt0x64(uint64(u), !p.fmt.sharp)
			}
		}
	case 'p': // p动词
		p.fmt0x64(uint64(u), !p.fmt.sharp)
	case 'b', 'o', 'd', 'x', 'X': // 2，8，10，16进制输出
		p.fmtInteger(uint64(u), unsigned, verb)
	default:
		p.badVerb(verb)
	}
}

// 捕获异常
func (p *pp) catchPanic(arg interface{}, verb rune, method string) {
	if err := recover(); err != nil {
	    // 如果是nil指针，则只需说“ <nil>”。 最可能的原因是Stringer无法防止nil或值接收器的nil指针，在任何情况下，“ <nil>”都是不错的结果。
		// If it's a nil pointer, just say "<nil>". The likeliest causes are a
		// Stringer that fails to guard against nil or a nil pointer for a
		// value receiver, and in either case, "<nil>" is a nice result.
		if v := reflect.ValueOf(arg); v.Kind() == reflect.Ptr && v.IsNil() {
			p.buf.writeString(nilAngleString)
			return
		}
		
		// 否则，打印简洁的panic消息。 大多数时候，panic值会很好地打印出来。
		// Otherwise print a concise panic message. Most of the time the panic
		// value will print itself nicely.
		if p.panicking {
		    // 嵌套恐慌； printArg中的递归不能成功。再次触发panic
			// Nested panics; the recursion in printArg cannot succeed.
			panic(err)
		}

		oldFlags := p.fmt.fmtFlags
		// 对于此输出，我们需要默认行为。
		// For this output we want default behavior.
		p.fmt.clearflags() // 清除标记

        // 写panic信息
		p.buf.writeString(percentBangString)
		p.buf.writeRune(verb)
		p.buf.writeString(panicString)
		p.buf.writeString(method)
		p.buf.writeString(" method: ")
		p.panicking = true
		p.printArg(err, 'v')
		p.panicking = false
		p.buf.writeByte(')')

		p.fmt.fmtFlags = oldFlags // 标记还原
	}
}

// 处理方法，返回值表示是否已经处理过
func (p *pp) handleMethods(verb rune) (handled bool) {
	if p.erroring { // 如果已经有错误就不用处理
		return
	}
	if verb == 'w' { 
	    // 不能将％w与Errorf多次一起使用，也不能与非错误arg一起使用。
		// It is invalid to use %w other than with Errorf, more than once,
		// or with a non-error arg.
		err, ok := p.arg.(error)
		if !ok || !p.wrapErrs || p.wrappedErr != nil { // arg不是错误，没有出错，也没有处理过错误
			p.wrappedErr = nil
			p.wrapErrs = false
			p.badVerb(verb)
			return true
		}
		p.wrappedErr = err
		// 如果arg是Formatter，则将'v'作为动词传递给它。
		// If the arg is a Formatter, pass 'v' as the verb to it.
		verb = 'v'
	}

    // arg是格式化对象
	// Is it a Formatter?
	if formatter, ok := p.arg.(Formatter); ok {
		handled = true
		defer p.catchPanic(p.arg, verb, "Format")
		formatter.Format(p, verb)
		return
	}

    // 如果我们正在执行Go语法，并且该参数知道如何应用于go语法，请立即处理。
	// If we're doing Go syntax and the argument knows how to supply it, take care of it now.
	if p.fmt.sharpV {
		if stringer, ok := p.arg.(GoStringer); ok {
			handled = true
			defer p.catchPanic(p.arg, verb, "GoString")
			// 无修饰地打印GoString的结果。
			// Print the result of GoString unadorned.
			p.fmt.fmtS(stringer.GoString())
			return
		}
	} else {
	    // 如果根据格式可接受字符串，请查看该值是否满足字符串值接口之一。 Println等将动词设置为％v，这是“可字符串化的”。
		// If a string is acceptable according to the format, see if
		// the value satisfies one of the string-valued interfaces.
		// Println etc. set verb to %v, which is "stringable".
		switch verb {
		case 'v', 's', 'x', 'X', 'q':
		    // 是错误还是Stringer？ 必须在主体中进行复制：必须在调用方法之前进行处理设置和延迟catchPanic。
			// Is it an error or Stringer?
			// The duplication in the bodies is necessary:
			// setting handled and deferring catchPanic
			// must happen before calling the method.
			switch v := p.arg.(type) {
			case error:
				handled = true
				defer p.catchPanic(p.arg, verb, "Error")
				p.fmtString(v.Error(), verb)
				return

			case Stringer:
				handled = true
				defer p.catchPanic(p.arg, verb, "String")
				p.fmtString(v.String(), verb)
				return
			}
		}
	}
	return false
}

// 输出参数
func (p *pp) printArg(arg interface{}, verb rune) {
	p.arg = arg
	p.value = reflect.Value{}
    
	if arg == nil {
		switch verb {
		case 'T', 'v':
			p.fmt.padString(nilAngleString)
		default:
			p.badVerb(verb)
		}
		return
	}

    //特殊处理注意事项。 %T（值类型）和%p（地址）是特殊的； 我们总是无处理他们。
	// Special processing considerations.
	// %T (the value's type) and %p (its address) are special; we always do them first.
	switch verb {
	case 'T':
		p.fmt.fmtS(reflect.TypeOf(arg).String())
		return
	case 'p':
		p.fmtPointer(reflect.ValueOf(arg), 'p')
		return
	}

    // 无需反射即可完成的类型。
	// Some types can be done without reflection.
	switch f := arg.(type) {
	case bool:
		p.fmtBool(f, verb)
	case float32:
		p.fmtFloat(float64(f), 32, verb)
	case float64:
		p.fmtFloat(f, 64, verb)
	case complex64:
		p.fmtComplex(complex128(f), 64, verb)
	case complex128:
		p.fmtComplex(f, 128, verb)
	case int:
		p.fmtInteger(uint64(f), signed, verb)
	case int8:
		p.fmtInteger(uint64(f), signed, verb)
	case int16:
		p.fmtInteger(uint64(f), signed, verb)
	case int32:
		p.fmtInteger(uint64(f), signed, verb)
	case int64:
		p.fmtInteger(uint64(f), signed, verb)
	case uint:
		p.fmtInteger(uint64(f), unsigned, verb)
	case uint8:
		p.fmtInteger(uint64(f), unsigned, verb)
	case uint16:
		p.fmtInteger(uint64(f), unsigned, verb)
	case uint32:
		p.fmtInteger(uint64(f), unsigned, verb)
	case uint64:
		p.fmtInteger(f, unsigned, verb)
	case uintptr:
		p.fmtInteger(uint64(f), unsigned, verb)
	case string:
		p.fmtString(f, verb)
	case []byte:
		p.fmtBytes(f, verb, "[]byte")
	case reflect.Value:
	    // 使用特殊方法处理可提取值，因为printValue不会在深度0处处理它们。
		// Handle extractable values with special methods
		// since printValue does not handle them at depth 0.
		if f.IsValid() && f.CanInterface() { // 合法的接口类型
			p.arg = f.Interface()
			if p.handleMethods(verb) {
				return
			}
		}
		p.printValue(f, verb, 0) // 作为一般的值类型处理
	default:
	    // 如果类型不简单，则可能有方法。
		// If the type is not simple, it might have methods.
		if !p.handleMethods(verb) {
		    // 由于类型没有可用于格式化的接口方法，因此需要使用反射。
			// Need to use reflection, since the type had no
			// interface methods that could be used for formatting.
			p.printValue(reflect.ValueOf(f), verb, 0)
		}
	}
}

// printValue与printArg相似，但以反射值而不是interface {}值开头。 它不处理'p'和'T'动词，因为这些应该已经由printArg处理。
// printValue is similar to printArg but starts with a reflect value, not an interface{} value.
// It does not handle 'p' and 'T' verbs because these should have been already handled by printArg.
func (p *pp) printValue(value reflect.Value, verb rune, depth int) {
    // 如果尚未由printArg处理（深度== 0），则使用特殊方法处理值。
	// Handle values with special methods if not already handled by printArg (depth == 0).
	if depth > 0 && value.IsValid() && value.CanInterface() { // 深度大于0，值是有效的，并且是接口
		p.arg = value.Interface()
		if p.handleMethods(verb) { // 以方法的方式先进行处理，处理成功就返回
			return
		}
	}
	
	// 处理前进行数据标记
	p.arg = nil
	p.value = value

	switch f := value; value.Kind() {
	case reflect.Invalid: 
		if depth == 0 { // 深度是0，直接输出"<invalid reflect.Value>"
			p.buf.writeString(invReflectString)
		} else {
			switch verb { // 深度大于0
			case 'v': 
				p.buf.writeString(nilAngleString)
			default:
				p.badVerb(verb)
			}
		}
	case reflect.Bool:
		p.fmtBool(f.Bool(), verb)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		p.fmtInteger(uint64(f.Int()), signed, verb)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		p.fmtInteger(f.Uint(), unsigned, verb)
	case reflect.Float32:
		p.fmtFloat(f.Float(), 32, verb)
	case reflect.Float64:
		p.fmtFloat(f.Float(), 64, verb)
	case reflect.Complex64:
		p.fmtComplex(f.Complex(), 64, verb)
	case reflect.Complex128:
		p.fmtComplex(f.Complex(), 128, verb)
	case reflect.String:
		p.fmtString(f.String(), verb)
	case reflect.Map:
		if p.fmt.sharpV {
			p.buf.writeString(f.Type().String())
			if f.IsNil() {
				p.buf.writeString(nilParenString)
				return
			}
			p.buf.writeByte('{')
		} else {
			p.buf.writeString(mapString)
		}
		sorted := fmtsort.Sort(f) // 按key排序输出
		for i, key := range sorted.Key { 
			if i > 0 {
				if p.fmt.sharpV {
					p.buf.writeString(commaSpaceString)
				} else {
					p.buf.writeByte(' ')
				}
			}
			p.printValue(key, verb, depth+1)
			p.buf.writeByte(':')
			p.printValue(sorted.Value[i], verb, depth+1)
		}
		if p.fmt.sharpV {
			p.buf.writeByte('}')
		} else {
			p.buf.writeByte(']')
		}
	case reflect.Struct: // 结构体类型
		if p.fmt.sharpV {
			p.buf.writeString(f.Type().String())
		}
		p.buf.writeByte('{')
		for i := 0; i < f.NumField(); i++ {
			if i > 0 {
				if p.fmt.sharpV {
					p.buf.writeString(commaSpaceString)
				} else {
					p.buf.writeByte(' ')
				}
			}
			if p.fmt.plusV || p.fmt.sharpV {
				if name := f.Type().Field(i).Name; name != "" {
					p.buf.writeString(name)
					p.buf.writeByte(':')
				}
			}
			p.printValue(getField(f, i), verb, depth+1) // 递归调用
		}
		p.buf.writeByte('}')
	case reflect.Interface: 
		value := f.Elem()
		if !value.IsValid() { // 值是无效的
			if p.fmt.sharpV {
				p.buf.writeString(f.Type().String())
				p.buf.writeString(nilParenString)
			} else {
				p.buf.writeString(nilAngleString)
			}
		} else { // 值有效
			p.printValue(value, verb, depth+1) // 递归调用
		}
	case reflect.Array, reflect.Slice: // 数组或者切片
		switch verb {
		case 's', 'q', 'x', 'X':
		    // 处理上述动词专用的byte和uint8切片和数组。
			// Handle byte and uint8 slices and arrays special for the above verbs.
			t := f.Type()
			if t.Elem().Kind() == reflect.Uint8 { // 元素是无符号int8
				var bytes []byte
				if f.Kind() == reflect.Slice { // 切片类型
					bytes = f.Bytes()
				} else if f.CanAddr() { // 可取址的指针类型
					bytes = f.Slice(0, f.Len()).Bytes()
				} else {
				    // 我们有一个数组，但是不能对无法寻址的数组Slice()进行切片，因此我们需要手动构建切片。这是一种罕见的情况，但是如果反射可以提供更多帮助，那就太好了。
					// We have an array, but we cannot Slice() a non-addressable array,
					// so we build a slice by hand. This is a rare case but it would be nice
					// if reflection could help a little more.
					bytes = make([]byte, f.Len())
					for i := range bytes { // 不能取址的切片，其元素值要一个一个单独提取出来
						bytes[i] = byte(f.Index(i).Uint())
					}
				}
				p.fmtBytes(bytes, verb, t.String())
				return
			}
		}
		if p.fmt.sharpV { 有#v符号
			p.buf.writeString(f.Type().String())
			if f.Kind() == reflect.Slice && f.IsNil() { // nil切片
				p.buf.writeString(nilParenString)
				return
			}
			p.buf.writeByte('{')
			for i := 0; i < f.Len(); i++ {
				if i > 0 {
					p.buf.writeString(commaSpaceString)
				}
				p.printValue(f.Index(i), verb, depth+1)
			}
			p.buf.writeByte('}')
		} else { 有v符号
			p.buf.writeByte('[')
			for i := 0; i < f.Len(); i++ {
				if i > 0 {
					p.buf.writeByte(' ')
				}
				p.printValue(f.Index(i), verb, depth+1)
			}
			p.buf.writeByte(']')
		}
	case reflect.Ptr: // 指针类型
	    // 指向数组或切片或结构的指针？ 可以，但不能嵌入（避免循环）
		// pointer to array or slice or struct? ok at top level
		// but not embedded (avoid loops)
		if depth == 0 && f.Pointer() != 0 { // 深度为0的指针
			switch a := f.Elem(); a.Kind() { // 元素值类型是数组，切片，结构体，映射
			case reflect.Array, reflect.Slice, reflect.Struct, reflect.Map:
				p.buf.writeByte('&')
				p.printValue(a, verb, depth+1)
				return
			}
		}
		fallthrough // 接着向下处理
	case reflect.Chan, reflect.Func, reflect.UnsafePointer: // 通道，函数，非安全指针
		p.fmtPointer(f, verb)
	default:
		p.unknownType(f)
	}
}

// intFromArg获取a的第argNum个元素。 返回时，isInt报告参数是否具有整数类型。
// intFromArg gets the argNumth element of a. On return, isInt reports whether the argument has integer type.
func intFromArg(a []interface{}, argNum int) (num int, isInt bool, newArgNum int) {
	newArgNum = argNum
	if argNum < len(a) {
		num, isInt = a[argNum].(int) // Almost always OK. 几乎总是OK的
		if !isInt { // 不是整数
			// Work harder.
			switch v := reflect.ValueOf(a[argNum]); v.Kind() { 
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64: // 有符号整数类型
				n := v.Int()
				if int64(int(n)) == n { // 没有溢出
					num = int(n)
					isInt = true
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr: // 无符号整数类型
				n := v.Uint()
				if int64(n) >= 0 && uint64(int(n)) == n { // 没有溢出
					num = int(n)
					isInt = true
				}
			default:
			    // num和isInt已经是0，false
				// Already 0, false.
			}
		}
		newArgNum = argNum + 1 // 下一个元素的位置
		if tooLarge(num) { // 数值是否太大
			num = 0
			isInt = false
		}
	}
	return
}

// parseArgNumber返回带括号的数字的值减去1（显式参数编号为1开始的索引，但我们希望为以0开始索引，#[1] ==> 0）。众所周知，左括号以format[0]出现。返回的值是索引，直到结束括号所消耗的字节数（如果存在）以及该数目是否可以解析。如果没有关闭括号，则要消耗的字节将为1。
// @return index    索引 
// @return wid      直到结束括号所消耗的字节数（如果存在）
// @return ok       该数目是否可以解析
// parseArgNumber returns the value of the bracketed number, minus 1
// (explicit argument numbers are one-indexed but we want zero-indexed).
// The opening bracket is known to be present at format[0].
// The returned values are the index, the number of bytes to consume
// up to the closing paren, if present, and whether the number parsed
// ok. The bytes to consume will be 1 if no closing paren is present.
func parseArgNumber(format string) (index int, wid int, ok bool) {
    // 必须至少有3个字节：[n]
	// There must be at least 3 bytes: [n].
	if len(format) < 3 {
		return 0, 1, false
	}

    // 查找右括号
	// Find closing bracket.
	for i := 1; i < len(format); i++ {
		if format[i] == ']' {
			width, ok, newi := parsenum(format, 1, i)
			if !ok || newi != i { // 解析不成功，或者新的位置和原来的位置不相等
				return 0, i + 1, false
			}
			
			// 参数是以数字1开开始的索引，并跳过括号。
			return width - 1, i + 1, true // arg numbers are one-indexed and skip paren.
		}
	}
	
	// 从当前位置开始不能解析，从下一个位置开始解析
	return 0, 1, false
}

// argNumber返回下一个要求值的参数，它可以是传入的argNum的值，也可以是以format [i：]开头的括号中的整数的值。 它还返回i的新值，即要处理的格式的下一个字节的索引。
// @param   argNum      表示当前格式化参数数目
// @param   format      格式化字符串
// @param   i           格式化字符串索引
// @param   numArgs     参数个数
// @return  newArgNum   表示新的格式化参数数目
// @return  newi        新的处理位置
// @return  found       当前是否找到索引
// argNumber returns the next argument to evaluate, which is either the value of the passed-in
// argNum or the value of the bracketed integer that begins format[i:]. It also returns
// the new value of i, that is, the index of the next byte of the format to process.
func (p *pp) argNumber(argNum int, format string, i int, numArgs int) (newArgNum, newi int, found bool) {
	if len(format) <= i || format[i] != '[' {
		return argNum, i, false
	}
	p.reordered = true
	index, wid, ok := parseArgNumber(format[i:])
	if ok && 0 <= index && index < numArgs {
		return index, i + wid, true
	}
	p.goodArgNum = false
	return argNum, i + wid, ok
}

// 错误参数值
func (p *pp) badArgNum(verb rune) {
	p.buf.writeString(percentBangString)
	p.buf.writeRune(verb)
	p.buf.writeString(badIndexString)
}

// 丢失参数
func (p *pp) missingArg(verb rune) {
	p.buf.writeString(percentBangString)
	p.buf.writeRune(verb)
	p.buf.writeString(missingString)
}

// 格式化输出
// @param format    格式化字符串
// @param a         需要格式化的值
func (p *pp) doPrintf(format string, a []interface{}) {
	end := len(format)
	// argNum，统计参数个数，参数是形如#[1]，#[2]等
	argNum := 0         // we process one argument per non-trivial format // 我们以非平凡的格式处理一个参数
	afterIndex := false // previous item in format was an index like [3]. // 前一格式上是类似[3]的索引，表示是否处理完索引。
	p.reordered = false
formatLoop:
	for i := 0; i < end; {
		p.goodArgNum = true
		lasti := i
		for i < end && format[i] != '%' {
			i++
		}
		if i > lasti {
			p.buf.writeString(format[lasti:i]) // 处理好的一个动词
		}
		if i >= end { // 已经处理到末尾
			// done processing format string
			break
		}
        
        // 定位到%的下一个字符
		// Process one verb
		i++

        // 清理标记
		// Do we have flags?
		p.fmt.clearflags()
	simpleFormat: // 简单格式，下面是设置标记
		for ; i < end; i++ {
			c := format[i]
			switch c {
			case '#':
				p.fmt.sharp = true
			case '0':
				p.fmt.zero = !p.fmt.minus // Only allow zero padding to the left. // 只允许零填充到左边。
			case '+':
				p.fmt.plus = true
			case '-':
				p.fmt.minus = true
				p.fmt.zero = false // Do not pad with zeros to the right. // 不能在右边填充零。
			case ' ':
				p.fmt.space = true
			default:
			    // 不带精度，宽度或参数索引的ascii小写简单动词常见情况的快速处理方式。
				// Fast path for common case of ascii lower case simple verbs
				// without precision or width or argument indices.
				if 'a' <= c && c <= 'z' && argNum < len(a) {
					if c == 'v' { // 如果是v动词
						// Go syntax go语法
						p.fmt.sharpV = p.fmt.sharp
						p.fmt.sharp = false
						// Struct-field syntax 结构体字段语法
						p.fmt.plusV = p.fmt.plus
						p.fmt.plus = false
					}
					p.printArg(a[argNum], rune(c)) // 打印参数
					argNum++ // 参数数目增加
					i++ // 下一个处理字符
					continue formatLoop // 继续格式化循环
				}
				
				// 格式比简单的标记和动词更复杂，或者格式不正确。
				// Format is more complex than simple flags and a verb or is malformed.
				break simpleFormat
			}
		}

        // 我们有一个明确的参数索引？
		// Do we have an explicit argument index?
		argNum, i, afterIndex = p.argNumber(argNum, format, i, len(a)) // 根据当前的格式化参数个数和位置，找下一个格式化参数的值和位置

        // 我们是否有宽度？
		// Do we have width?
		if i < end && format[i] == '*' { // 第i个值是*号，那么些时的argNum格式化参数对应的值就是精度值
			i++
			p.fmt.wid, p.fmt.widPresent, argNum = intFromArg(a, argNum) // a[argNum]表示精度值

			if !p.fmt.widPresent { // 如果此时精度不存在，那么宽度就是有问题的
				p.buf.writeString(badWidthString)
			}

            // 我们有一个负宽度，因此取其值并确保设置减号标志
			// We have a negative width, so take its value and ensure
			// that the minus flag is set
			if p.fmt.wid < 0 {
				p.fmt.wid = -p.fmt.wid
				p.fmt.minus = true
				p.fmt.zero = false // Do not pad with zeros to the right. // 不允许在右边填充0
			}
			
			// TODO ?
			afterIndex = false
		} else { // 当前i所在的位置不是*号
			p.fmt.wid, p.fmt.widPresent, i = parsenum(format, i, end)
			if afterIndex && p.fmt.widPresent { // "%[3]2d"
				p.goodArgNum = false // 最近的重新排序指令无效
			}
		}
        
        // 有小数点要处理
		// Do we have precision?
		if i+1 < end && format[i] == '.' { // 当前值是小数点，且小数点后有值
			i++
			if afterIndex { // "%[3].2d" // 表示索引之后的值
				p.goodArgNum = false  // 最近的重新排序指令无效，方便处理一下参数       
			}
			argNum, i, afterIndex = p.argNumber(argNum, format, i, len(a)) // 处理一个参数
			if i < end && format[i] == '*' { // 当前值是*
				i++ 
				p.fmt.prec, p.fmt.precPresent, argNum = intFromArg(a, argNum) // 从参数中提取整数
				// 负精度参数没有意义
				// Negative precision arguments don't make sense
				if p.fmt.prec < 0 {
					p.fmt.prec = 0
					p.fmt.precPresent = false
				}
				if !p.fmt.precPresent { // 精度不存在
					p.buf.writeString(badPrecString)
				}
				afterIndex = false 
			} else { // 前是非*号
				p.fmt.prec, p.fmt.precPresent, i = parsenum(format, i, end) // 解析格式字符串中的数值
				if !p.fmt.precPresent { // 如果精度不存在，设置精存在，值为0
					p.fmt.prec = 0
					p.fmt.precPresent = true
				}
			}
		}

		if !afterIndex { // 索引未处理完，接着处理
			argNum, i, afterIndex = p.argNumber(argNum, format, i, len(a))
		}

		if i >= end { // 已经处理到最后了，说明没有动词了，可以退出处理
			p.buf.writeString(noVerbString)
			break
		}

        // 处理单个字符
		verb, size := rune(format[i]), 1
		if verb >= utf8.RuneSelf {
			verb, size = utf8.DecodeRuneInString(format[i:])
		}
		i += size

		switch {
		case verb == '%': // Percent does not absorb operands and ignores f.wid and f.prec. // %不吸收操作数，并且忽略f.wid和f.prec。
			p.buf.writeByte('%') // 说明是普通的%
		case !p.goodArgNum: // 最近的重新排序指令无效
			p.badArgNum(verb)
		case argNum >= len(a): // No argument left over to print for the current verb. // 没有剩余参数可为当前动词打印。
			p.missingArg(verb)
		case verb == 'v':
			// Go syntax
			p.fmt.sharpV = p.fmt.sharp
			p.fmt.sharp = false
			// Struct-field syntax
			p.fmt.plusV = p.fmt.plus
			p.fmt.plus = false
			fallthrough
		default:
			p.printArg(a[argNum], verb)
			argNum++
		}
	}

    // 除非调用未按顺序访问参数，否则请检查是否有其他参数，在这种情况下，检测它们是否已全部使用非常昂贵，如果没有使用，则可以确定。
	// Check for extra arguments unless the call accessed the arguments
	// out of order, in which case it's too expensive to detect if they've all
	// been used and arguably OK if they're not.
	if !p.reordered && argNum < len(a) { // 没有重排序，并且格式化参数个数小于参数个数，说明有格式化参数少，传入的参数多
		p.fmt.clearflags()
		p.buf.writeString(extraString)
		for i, arg := range a[argNum:] {
			if i > 0 {
				p.buf.writeString(commaSpaceString)
			}
			if arg == nil {
				p.buf.writeString(nilAngleString)
			} else {
				p.buf.writeString(reflect.TypeOf(arg).String())
				p.buf.writeByte('=')
				p.printArg(arg, 'v')
			}
		}
		p.buf.writeByte(')')
	}
}

// 输出数组对象
func (p *pp) doPrint(a []interface{}) {
	prevString := false // 前一个参数是否是字符串
	for argNum, arg := range a {
		isString := arg != nil && reflect.TypeOf(arg).Kind() == reflect.String // 判断是否是字符串
		// 在两个非字符串参数之间添加空格
		// Add a space between two non-string arguments.
		if argNum > 0 && !isString && !prevString {
			p.buf.writeByte(' ')
		}
		p.printArg(arg, 'v')
		prevString = isString
	}
}

// doPrintln类似于doPrint，但是总是在参数之间添加一个空格，在最后一个参数之后添加换行符。
// doPrintln is like doPrint but always adds a space between arguments
// and a newline after the last argument.
func (p *pp) doPrintln(a []interface{}) {
	for argNum, arg := range a {
		if argNum > 0 {
			p.buf.writeByte(' ')
		}
		p.printArg(arg, 'v')
	}
	p.buf.writeByte('\n')
}
```