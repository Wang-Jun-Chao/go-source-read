
```go
package fmt

import (
    "strconv"
    "unicode/utf8"
)

const (
    ldigits = "0123456789abcdefx"
    udigits = "0123456789ABCDEFX"
)

const (
    signed   = true
    unsigned = false
)

// 放置在单独的结构中的标志以便于清除。
type fmtFlags struct {
    widPresent  bool
    precPresent bool
    minus       bool
    plus        bool
    sharp       bool
    space       bool
    zero        bool
    
    // 对于%+v,%#v格式，我们设置plusV/sharpV标志并清除plus/sharp标志，
    // 因为%+v,%#v实际上是在顶层设置的不同的无标志格式。
    // For the formats %+v %#v, we set the plusV/sharpV flags 
    // and clear the plus/sharp flags since %+v 
    // and %#v are in effect different, flagless formats set at the top level.
    plusV  bool
    sharpV bool
}
// fmt是被使用的原始格式化器。
// 它打印到必须单独设置的缓冲区中。
// A fmt is the raw formatter used by Printf etc.
// It prints into a buffer that must be set up separately.
type fmt struct {
    buf *buffer

    fmtFlags

    wid  int // 宽度
    prec int // 精度

    // intbuf足够大，可以存储带符号的int64的％b格式
    // 并且避免在32位架构上的结构末尾进行填充
    // intbuf is large enough to store %b of an int64 with a sign and
    // avoids padding at the end of the struct on 32 bit architectures.
    intbuf [68]byte
}

func (f *fmt) clearflags() {
    f.fmtFlags = fmtFlags{}
}

func (f *fmt) init(buf *buffer) {
    f.buf = buf
    f.clearflags()
}

// writePadding生成n个字节的填充。
// writePadding generates n bytes of padding.
func (f *fmt) writePadding(n int) {
    // 不需要填充
    if n <= 0 { // No padding bytes needed.
        return
    }
    buf := *f.buf
    oldLen := len(buf)
    newLen := oldLen + n
    // Make enough room for padding.
    // 新长度>原有的容量
    if newLen > cap(buf) {
        // 原有容量*2+n
        // TODO buffer是哪来的？
        buf = make(buffer, cap(buf)*2+n)
        // 拷贝数据
        copy(buf, *f.buf)
    }
    // Decide which byte the padding should be filled with.
    // 确定填充哪种字节，默认是空格字符，如果f.zero被设置就填充0字符
    padByte := byte(' ')
    if f.zero {
        padByte = byte('0')
    }
    // Fill padding with padByte.
    // [oldLen:newLen]字节填充
    padding := buf[oldLen:newLen]
    for i := range padding {
        padding[i] = padByte
    }
    
    // 取子数组
    *f.buf = buf[:newLen]
}

// 将b填充到f.buf，!f.minus为真，填充到左边，f.miuns为真填充到右边
// pad appends b to f.buf, padded on left (!f.minus) or right (f.minus).
func (f *fmt) pad(b []byte) {
    // 宽度不存在，或者宽度为0，直接在追加在f.buf后面
    if !f.widPresent || f.wid == 0 {
        f.buf.write(b)
        return
    }
    // 求填充宽度
    // utf8.RuneCount(b):求b中的字符数
    width := f.wid - utf8.RuneCount(b)
    if !f.minus {
        // 左边填充
        // left padding
        f.writePadding(width)
        f.buf.write(b)
    } else {
        // 右边填充
        // right padding
        f.buf.write(b)
        f.writePadding(width)
    }
}

// 将p字符串s填充到f.buf，!f.minus为真，填充到左边，f.miuns为真填充到右边
// 参见func (f *fmt) pad(b []byte)方法
func (f *fmt) padString(s string) {
    if !f.widPresent || f.wid == 0 {
        f.buf.writeString(s)
        return
    }
    width := f.wid - utf8.RuneCountInString(s)
    if !f.minus {
        // left padding
        f.writePadding(width)
        f.buf.writeString(s)
    } else {
        // right padding
        f.buf.writeString(s)
        f.writePadding(width)
    }
}

// 格式化布尔值。
// fmtBoolean formats a boolean.
func (f *fmt) fmtBoolean(v bool) {
    if v {
        f.padString("true")
    } else {
        f.padString("false")
    }
}

// fmtUnicode将uint64格式设置为"U+0078"或将f.sharp设置为 "U+0078 'x'"。
// fmtUnicode formats a uint64 as "U+0078" or with f.sharp set as "U+0078 'x'".
func (f *fmt) fmtUnicode(u uint64) {
    buf := f.intbuf[0:]

    // 使用默认精度设置，用%#U ("U+FFFFFFFFFFFFFFFF")格式化-1所需的最大buf长度为18，
    // 该长度适合已分配的intbuf，容量为68个字节。
    // With default precision set the maximum needed buf length is 18
    // for formatting -1 with %#U ("U+FFFFFFFFFFFFFFFF") which fits
    // into the already allocated intbuf with a capacity of 68 bytes.
    prec := 4 // 默认精度是4
    // 存在比默认精度大的精度
    if f.precPresent && f.prec > 4 {
        prec = f.prec
        // 计算"U+"，数字，" '"，字符，"'"所需的空间。
        // Compute space needed for "U+" , number, " '", character, "'".
        width := 2 + prec + 2 + utf8.UTFMax + 1
        // 容量不足就扩容
        if width > len(buf) {
            buf = make([]byte, width)
        }
    }

    // 格式化为buf，以buf [i]结尾。 从右到左，格式化数字更容易。
    // Format into buf, ending at buf[i]. Formatting numbers is easier right-to-left.
    i := len(buf)

    // //对于%#U格式，我们要在缓冲区的末尾添加一个空格和一个带引号的字符。
    // For %#U we want to add a space and a quoted character at the end of the buffer.
    // f.sharp存在，并且u是合法的unicode编码，并且是可以打印的字符
    if f.sharp && u <= utf8.MaxRune && strconv.IsPrint(rune(u)) {
        i-- // 最后一个字符位置
        buf[i] = '\'' // 添加'字符
        i -= utf8.RuneLen(rune(u))
        utf8.EncodeRune(buf[i:], rune(u)) // 写unicode编码
        i--
        buf[i] = '\'' // 写'字符
        i--
        buf[i] = ' ' // 写空格字符
    }
    // 将Unicode代码点u格式化为十六进制数。
    // Format the Unicode code point u as a hexadecimal number.
    for u >= 16 { // 一个字节一个字节写，除了最后一个字节
        i--
        buf[i] = udigits[u&0xF]
        prec--
        u >>= 4
    }
    i--
    buf[i] = udigits[u] // 写最后一个字节
    prec--
    // 在数字前加零，直到达到要求的精度。
    // Add zeros in front of the number until requested precision is reached.
    for prec > 0 {
        i--
        buf[i] = '0'
        prec--
    }
    // 添加前导的"U+字符"
    // Add a leading "U+".
    i--
    buf[i] = '+'
    i--
    buf[i] = 'U'
    
    oldZero := f.zero
    f.zero = false
    f.pad(buf[i:])
    f.zero = oldZero
}

// fmtInteger格式化有符号和无符号整数。
// digits string 这个参数代表：ldigits，或者udigits异量字符串
// fmtInteger formats signed and unsigned integers.
func (f *fmt) fmtInteger(u uint64, base int, isSigned bool, verb rune, digits string) {
    // 处理有符号数
    negative := isSigned && int64(u) < 0
    if negative {
        u = -u
    }

    buf := f.intbuf[0:]
    // 已经分配的f.intbuf，容量为68个字节，如果未设置精度或宽度，则足以用于整数格式。
    // The already allocated f.intbuf with a capacity of 68 bytes
    // is large enough for integer formatting when no precision or width is set.
    if f.widPresent || f.precPresent {
        // 记入3个额外的字节，以便可能添加符号和“ 0x”。
        // Account 3 extra bytes for possible addition of a sign and "0x".
        width := 3 + f.wid + f.prec // wid and prec are always positive.
        if width > len(buf) {
            // We're going to need a bigger boat.
            buf = make([]byte, width)
        }
    }

    // 要求额外的前导零位的两种方式：％.3d或％03d。
    // 如果同时指定了，f.zero标志会被忽略，用空格填充。
    // Two ways to ask for extra leading zero digits: %.3d or %03d.
    // If both are specified the f.zero flag is ignored and
    // padding with spaces is used instead.
    prec := 0
    if f.precPresent { // 精度存在
        prec = f.prec // 记录精度
        // 精度为0，值为0表示除填充外，“不打印任何内容”。
        // Precision of 0 and value of 0 means "print nothing" but padding.
        if prec == 0 && u == 0 {
            oldZero := f.zero
            f.zero = false
            f.writePadding(f.wid)
            f.zero = oldZero
            return
        }
    } else if f.zero && f.widPresent { // 0存在，并且宽度存在
        prec = f.wid // 精度就是宽度
        // 如果是负数，或者格式中有+号或者空格，精度减一，给符号标记留空间
        if negative || f.plus || f.space {
            prec-- // leave room for sign
        }
    }
    
    // 因为从右到左打印更容易：将u格式化为buf，以buf [i]结尾。
    // 通过拆分32位的大小写，可以稍微加快速度
    // 放入一个单独的块中，但这不值得重复操作，因此u是64位的。
    // Because printing is easier right-to-left: format u into buf, ending at buf[i].
    // We could make things marginally faster by splitting the 32-bit case out
    // into a separate block but it's not worth the duplication, so u has 64 bits.
    i := len(buf)
    // 使用常数进行除法，并使用模数以获得更有效的代码。
    // switch case流行程序排序。
    // Use constants for the division and modulo for more efficient code.
    // Switch cases ordered by popularity.
    switch base {
    case 10: // 10进制
        for u >= 10 {
            i--
            next := u / 10
            buf[i] = byte('0' + u - next*10)
            u = next
        }
    case 16: // 16进制
        for u >= 16 {
            i--
            buf[i] = digits[u&0xF]
            u >>= 4
        }
    case 8: // 8进制
        for u >= 8 {
            i--
            buf[i] = byte('0' + u&7)
            u >>= 3
        }
    case 2:
        for u >= 2 {
            i--
            buf[i] = byte('0' + u&1)
            u >>= 1
        }
    default: // 其他进制
        panic("fmt: unknown base; can't happen")
    }
    i--
    buf[i] = digits[u] // 最后一个值
    for i > 0 && prec > len(buf)-i { // 补前导0
        i--
        buf[i] = '0'
    }
    
    // 各种前缀：0x，-等
    // Various prefixes: 0x, -, etc.
    if f.sharp { 如果有"#"符号
        switch base { 
        case 2: // 二进制
            // Add a leading 0b.
            i--
            buf[i] = 'b'
            i--
            buf[i] = '0'
        case 8: // 8进制
            if buf[i] != '0' { // 最高位已经是0，就不需要补了
                i--
                buf[i] = '0'
            }
        case 16: // 16进制
            // 添加前缀0x或0X。
            // Add a leading 0x or 0X.
            i--
            buf[i] = digits[16]
            i--
            buf[i] = '0'
        }
    }
    // 有O
    if verb == 'O' {
        i--
        buf[i] = 'o'
        i--
        buf[i] = '0'
    }

    if negative { // 负数
        i--
        buf[i] = '-'
    } else if f.plus { // 有+号标记
        i--
        buf[i] = '+'
    } else if f.space { // 有空格标记
        i--
        buf[i] = ' '
    }

    // 左填充为零已经处理过，如之前的精度处理
    // f.zero标记也因为显示设置精度而被忽略
    // Left padding with zeros has already been handled like precision earlier
    // or the f.zero flag is ignored due to an explicitly set precision.
    oldZero := f.zero
    f.zero = false
    f.pad(buf[i:])
    f.zero = oldZero
}

// truncate truncates the string s to the specified precision, if present.
func (f *fmt) truncateString(s string) string {
    if f.precPresent {
        n := f.prec
        for i := range s {
            n--
            if n < 0 {
                return s[:i]
            }
        }
    }
    return s
}

// 如果精度存在，truncate将字符串s截断为指定的精度
// truncate truncates the byte slice b as a string of the specified precision, if present.
func (f *fmt) truncate(b []byte) []byte {
    if f.precPresent { // 精度存在
        n := f.prec
        for i := 0; i < len(b); {
            n--
            if n < 0 { // 已经处理完
                return b[:i]
            }
            wid := 1
            if b[i] >= utf8.RuneSelf {// 如果是大于一个字节表示的UTF8编码
                _, wid = utf8.DecodeRune(b[i:])
            }
            i += wid // 移到指定字节
        }
    }
    
    // 说明原字符串精度比指定的精度小，可以全部返回
    return b
}

// fmtS格式化字符串
// fmtS formats a string.
func (f *fmt) fmtS(s string) {
    s = f.truncateString(s) // 进行字符串截断
    f.padString(s)          // 再进行对齐
}

// fmtBs格式化字节切片b，就像将其格式化为字符串一样
// fmtBs formats the byte slice b as if it was formatted as string with fmtS.
func (f *fmt) fmtBs(b []byte) {
    b = f.truncate(b)
    f.pad(b)
}

// fmtSbx将字符串或字节切片格式化为其字节的十六进制编码。
// @param digits 表示ldigits或者udigits
// fmtSbx formats a string or byte slice as a hexadecimal encoding of its bytes.
func (f *fmt) fmtSbx(s string, b []byte, digits string) {
    length := len(b)
    if b == nil {
        // 没有字节片。 则假设字符串s应该被编码。
        // No byte slice present. Assume string s should be encoded.
        length = len(s)
    }
    // 设置处理长度，不超出精度要求。超出精度部分不处理。
    // Set length to not process more bytes than the precision demands.
    if f.precPresent && f.prec < length {
        length = f.prec
    }
    // 考虑到f.sharp和f.space标志，计算编码宽度。
    // Compute width of the encoding taking into account the f.sharp and f.space flag.
    width := 2 * length // 宽度是长度的2倍，因为一个字符需要用两个位来表示
    if width > 0 {
        if f.space { // 如果有空格
            // 每个由两个十六进制编码的元素将获得前缀0x或0X。
            // Each element encoded by two hexadecimals will get a leading 0x or 0X.
            if f.sharp { // 如果#标记存在
                width *= 2 // 宽度在原来的基础上再加倍
            }
            // 元素将由空格分隔。
            // Elements will be separated by a space.
            width += length - 1
        } else if f.sharp { // 没有空格，只有#标记
            // 对于整个字符串，只会添加前导0x或0X。
            // Only a leading 0x or 0X will be added for the whole string.
            width += 2 // 宽加倍
        }
    } else { // The byte slice or string that should be encoded is empty.
        // 应该编码的字节片或字符串为空。
        if f.widPresent {
            f.writePadding(f.wid)
        }
        return
    }
    // 处理左侧的填充。
    // Handle padding to the left.
    if f.widPresent && f.wid > width && !f.minus {
        // 指定的宽度存在，没有-号标记
        f.writePadding(f.wid - width)
    }
    // 将编码直接写入输出缓冲区。
    // Write the encoding directly into the output buffer.
    buf := *f.buf
    if f.sharp {
        // 添加前导0x或0X。
        // Add leading 0x or 0X.
        buf = append(buf, '0', digits[16]) // buf = '0x'或者'0X'
    }
    var c byte
    for i := 0; i < length; i++ { // 处理每个字符
        if f.space && i > 0 { // 从第二个元素开始，如果空格存在
            // Separate elements with a space.
            buf = append(buf, ' ') // buf[len-1] = ' '
            if f.sharp {
                // 为每个元素添加前导0x或0X
                // Add leading 0x or 0X for each element.
                // buf[len-3] = '0x '或者'0X '
                buf = append(buf, '0', digits[16])
            }
        }
        if b != nil { // 说明是从输入的字节切片中进行处理
            // 从输入字节片中取出一个字节。
            c = b[i] // Take a byte from the input byte slice.
        } else { // 说明是从输入的字符串中进行处理
            // 从输入字符串中取出一个字节。
            c = s[i] // Take a byte from the input string.
        }
        // 将每个字节编码为两个十六进制数字。
        // Encode each byte as two hexadecimal digits.
        buf = append(buf, digits[c>>4], digits[c&0xF])
    }
    *f.buf = buf
    // 处理右填充。
    // Handle padding to the right.
    if f.widPresent && f.wid > width && f.minus {
        // 指定的宽度比较当前宽度大，并且有-号标记
        f.writePadding(f.wid - width)
    }
}

// fmtSx将字符串格式化为其字节的十六进制编码。
// fmtSx formats a string as a hexadecimal encoding of its bytes.
func (f *fmt) fmtSx(s, digits string) {
    f.fmtSbx(s, nil, digits)
}

// fmtBx将字节切片格式化为其字节的十六进制编码。
// fmtBx formats a byte slice as a hexadecimal encoding of its bytes.
func (f *fmt) fmtBx(b []byte, digits string) {
    f.fmtSbx("", b, digits)
}

// fmtQ将字符串格式化为双引号，转义的Go字符串常量。
// 如果设置了f.sharp，则该字符串可以返回原始（带反引号）的字符串。
// 条件是该字符串不包含Tab以外的任何控制字符
// fmtQ formats a string as a double-quoted, escaped Go string constant.
// If f.sharp is set a raw (backquoted) string may be returned instead
// if the string does not contain any control characters other than tab.
func (f *fmt) fmtQ(s string) {
    // 字符串截断
    s = f.truncateString(s)
    // 如果有#标记，并且可以使用反引号，就使用引号返回字字符串
    if f.sharp && strconv.CanBackquote(s) {
        f.padString("`" + s + "`")
        return
    }
    buf := f.intbuf[:0]
    if f.plus { // 有+号标记
        f.pad(strconv.AppendQuoteToASCII(buf, s))
    } else {
        f.pad(strconv.AppendQuote(buf, s))
    }
}

// fmtC formats an integer as a Unicode character.
// If the character is not valid Unicode, it will print '\ufffd'.
func (f *fmt) fmtC(c uint64) {
    r := rune(c)
    if c > utf8.MaxRune {
        r = utf8.RuneError
    }
    buf := f.intbuf[:0]
    w := utf8.EncodeRune(buf[:utf8.UTFMax], r)
    f.pad(buf[:w])
}

// fmtQc formats an integer as a single-quoted, escaped Go character constant.
// If the character is not valid Unicode, it will print '\ufffd'.
func (f *fmt) fmtQc(c uint64) {
    r := rune(c)
    if c > utf8.MaxRune {
        r = utf8.RuneError
    }
    buf := f.intbuf[:0]
    if f.plus {
        f.pad(strconv.AppendQuoteRuneToASCII(buf, r))
    } else {
        f.pad(strconv.AppendQuoteRune(buf, r))
    }
}

// fmtFloat formats a float64. It assumes that verb is a valid format specifier
// for strconv.AppendFloat and therefore fits into a byte.
func (f *fmt) fmtFloat(v float64, size int, verb rune, prec int) {
    // Explicit precision in format specifier overrules default precision.
    if f.precPresent {
        prec = f.prec
    }
    // Format number, reserving space for leading + sign if needed.
    num := strconv.AppendFloat(f.intbuf[:1], v, byte(verb), prec, size)
    if num[1] == '-' || num[1] == '+' {
        num = num[1:]
    } else {
        num[0] = '+'
    }
    // f.space means to add a leading space instead of a "+" sign unless
    // the sign is explicitly asked for by f.plus.
    if f.space && num[0] == '+' && !f.plus {
        num[0] = ' '
    }
    // Special handling for infinities and NaN,
    // which don't look like a number so shouldn't be padded with zeros.
    if num[1] == 'I' || num[1] == 'N' {
        oldZero := f.zero
        f.zero = false
        // Remove sign before NaN if not asked for.
        if num[1] == 'N' && !f.space && !f.plus {
            num = num[1:]
        }
        f.pad(num)
        f.zero = oldZero
        return
    }
    // The sharp flag forces printing a decimal point for non-binary formats
    // and retains trailing zeros, which we may need to restore.
    if f.sharp && verb != 'b' {
        digits := 0
        switch verb {
        case 'v', 'g', 'G', 'x':
            digits = prec
            // If no precision is set explicitly use a precision of 6.
            if digits == -1 {
                digits = 6
            }
        }

        // Buffer pre-allocated with enough room for
        // exponent notations of the form "e+123" or "p-1023".
        var tailBuf [6]byte
        tail := tailBuf[:0]

        hasDecimalPoint := false
        // Starting from i = 1 to skip sign at num[0].
        for i := 1; i < len(num); i++ {
            switch num[i] {
            case '.':
                hasDecimalPoint = true
            case 'p', 'P':
                tail = append(tail, num[i:]...)
                num = num[:i]
            case 'e', 'E':
                if verb != 'x' && verb != 'X' {
                    tail = append(tail, num[i:]...)
                    num = num[:i]
                    break
                }
                fallthrough
            default:
                digits--
            }
        }
        if !hasDecimalPoint {
            num = append(num, '.')
        }
        for digits > 0 {
            num = append(num, '0')
            digits--
        }
        num = append(num, tail...)
    }
    // We want a sign if asked for and if the sign is not positive.
    if f.plus || num[0] != '+' {
        // If we're zero padding to the left we want the sign before the leading zeros.
        // Achieve this by writing the sign out and then padding the unsigned number.
        if f.zero && f.widPresent && f.wid > len(num) {
            f.buf.writeByte(num[0])
            f.writePadding(f.wid - len(num))
            f.buf.write(num[1:])
            return
        }
        f.pad(num)
        return
    }
    // No sign to show and the number is positive; just print the unsigned number.
    f.pad(num[1:])
}
```