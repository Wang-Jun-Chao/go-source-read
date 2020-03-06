```go
/*
    fmt包实现了类似C语言printf和scanf的格式化I/O。格式化动作（'verb'）源自C语言但更简单。
	Package fmt implements formatted I/O with functions analogous
	to C's printf and scanf.  The format 'verbs' are derived from C's but
	are simpler.

    输出
	Printing

    动作
	The verbs:

    通用：
        %v	值的默认格式表示
        %+v	类似%v，但输出结构体时会添加字段名
        %#v	值的Go语法表示，会附带包名信息
        %T	值的类型的Go语法表示，会附带包名信息
        %%	百分号
	General:
		%v	the value in a default format
			when printing structs, the plus flag (%+v) adds field names
		%#v	a Go-syntax representation of the value
		%T	a Go-syntax representation of the type of the value
		%%	a literal percent sign; consumes no value

    布尔值：
        %t	单词true或false
	Boolean:
		%t	the word true or false
		
	整数：
        %b	表示为二进制
        %c	该值对应的unicode码值
        %d	表示为十进制
        %o	表示为八进制
        %q	该值对应的单引号括起来的go语法字符字面值，必要时会采用安全的转义表示
        %x	表示为十六进制，使用a-f
        %X	表示为十六进制，使用A-F
        %U	表示为Unicode格式：U+1234，等价于"U+%04X"
	Integer:
		%b	base 2
		%c	the character represented by the corresponding Unicode code point
		%d	base 10
		%o	base 8
		%O	base 8 with 0o prefix
		%q	a single-quoted character literal safely escaped with Go syntax.
		%x	base 16, with lower-case letters for a-f
		%X	base 16, with upper-case letters for A-F
		%U	Unicode format: U+1234; same as "U+%04X"
		
	浮点数与复数的两个组分：
        %b	无小数部分、二进制指数的科学计数法，如-123456p-78；参见strconv.FormatFloat
        %e	科学计数法，如-1234.456e+78
        %E	科学计数法，如-1234.456E+78
        %f	有小数部分但无指数部分，如123.456
        %F	等价于%f
        %g	根据实际情况采用%e或%f格式（以获得更简洁、准确的输出）
        %G	根据实际情况采用%E或%F格式（以获得更简洁、准确的输出）
        %x  十六进制表示法（具有两个指数的十进制幂），例如 -0x1.23abcp+20
        %X  大写十六进制表示法，例如 -0X1.23ABCP + 20
	Floating-point and complex constituents:
		%b	decimalless scientific notation with exponent a power of two,
			in the manner of strconv.FormatFloat with the 'b' format,
			e.g. -123456p-78
		%e	scientific notation, e.g. -1.234456e+78
		%E	scientific notation, e.g. -1.234456E+78
		%f	decimal point but no exponent, e.g. 123.456
		%F	synonym for %f
		%g	%e for large exponents, %f otherwise. Precision is discussed below.
		%G	%E for large exponents, %F otherwise
		%x	hexadecimal notation (with decimal power of two exponent), e.g. -0x1.23abcp+20
		%X	upper-case hexadecimal notation, e.g. -0X1.23ABCP+20
	
	字符串和[]byte（下面的动作是同等对待的）：
        %s	直接输出字符串或者[]byte
        %q	该值对应的双引号括起来的go语法字符串字面值，必要时会采用安全的转义表示
        %x	每个字节用两字符十六进制数表示（使用a-f）
        %X	每个字节用两字符十六进制数表示（使用A-F）
	String and slice of bytes (treated equivalently with these verbs):
		%s	the uninterpreted bytes of the string or slice
		%q	a double-quoted string safely escaped with Go syntax
		%x	base 16, lower-case, two characters per byte
		%X	base 16, upper-case, two characters per byte
	
	切片：
        %p  以基数16表示的第0个元素的地址，开头为0x
	Slice:
		%p	address of 0th element in base 16 notation, with leading 0x
		
	指针：
        %p	表示为十六进制，并加上前导的0x 
	Pointer:
		%p	base 16 notation, with leading 0x
		%b，%d，%o，%x和%X动词也可以与指针配合使用，
完全将值格式化为整数。
		The %b, %d, %o, %x and %X verbs also work with pointers,
		formatting the value exactly as if it were an integer.

    %v的默认格式为：
		bool:                    %t
		int, int8等:             %d
		uint, uint8等:           %d, %#x（如果使用%#v打印）
		float32, complex64等:    %g
		string:                  %s
		chan:                    %p
		pointer:                 %p
	The default format for %v is:
		bool:                    %t
		int, int8 etc.:          %d
		uint, uint8 etc.:        %d, %#x if printed with %#v
		float32, complex64, etc: %g
		string:                  %s
		chan:                    %p
		pointer:                 %p
	
	对于复合对象，使用这些规则以递归方式打印元素，
    布局如下：
    	struct:             {field0 field1 ...}
		array, slice:       [elem0 elem1 ...]
		maps:               map[key1:value1 key2:value2 ...]
		pointer to above:   &{}, &[], &map[]
	For compound objects, the elements are printed using these rules, recursively,
	laid out like this:
		struct:             {field0 field1 ...}
		array, slice:       [elem0 elem1 ...]
		maps:               map[key1:value1 key2:value2 ...]
		pointer to above:   &{}, &[], &map[]

    宽度通过一个紧跟在百分号后面的十进制数指定，如果未指定宽度，则表示值时除必需之外不作填充。精度通过（可选的）宽度后跟点号后跟的十进制数指定。如果未指定精度，会使用默认精度；如果点号后没有跟数字，表示精度为0。
    举例如下：
        %f:    默认宽度，默认精度，默认精度是6
        %9f    宽度9，默认精度，默认精度是6
        %.2f   默认宽度，精度2
        %9.2f  宽度9，精度2
        %9.f   宽度9，精度0 
	Width is specified by an optional decimal number immediately preceding the verb.
	If absent, the width is whatever is necessary to represent the value.
	Precision is specified after the (optional) width by a period followed by a
	decimal number. If no period is present, a default precision is used.
	A period with no following number specifies a precision of zero.
	Examples:
		%f     default width, default precision
		%9f    width 9, default precision
		%.2f   default width, precision 2
		%9.2f  width 9, precision 2
		%9.f   width 9, precision 0

    

    

    宽度和精度以Unicode代码点（即，符文=字符）为单位进行度量。 （这与C的printf不同，后者的单位始终以字节为单位。）两个标志中的一个或两个都可以用字符'*'替换，从而使它们的值从下一个操作数获得（在格式化之前） 必须是int类型。
	Width and precision are measured in units of Unicode code points,
	that is, runes. (This differs from C's printf where the
	units are always measured in bytes.) Either or both of the flags
	may be replaced with the character '*', causing their values to be
	obtained from the next operand (preceding the one to format),
	which must be of type int.

    对于大多数值，width是要输出的最小符文数，必要时用空格填充格式化的表单。
	For most values, width is the minimum number of runes to output,
	padding the formatted form with spaces if necessary.

    但是，对于字符串，字节片和字节数组，精度会限制要格式化的输入的长度（而不是输出的大小），并在必要时将其截断。 通常，它以符文为单位进行度量，但是对于这些类型，使用％x或％X格式进行格式化时，将以字节为单位进行度量。
	For strings, byte slices and byte arrays, however, precision
	limits the length of the input to be formatted (not the size of
	the output), truncating if necessary. Normally it is measured in
	runes, but for these types when formatted with the %x or %X format
	it is measured in bytes.

    对于浮点值，width设置字段的最小宽度，而precision设置小数点后的位数（如果适用），除了%g/%G精度设置最大有效位数（除去末尾零位） 。 例如，给定12.345，格式%6.3f打印12.345，而%.3g打印12.3。%e，%f和%#g的默认精度为6；对于%g，它是唯一标识该值所需的最少位数。
	For floating-point values, width sets the minimum width of the field and
	precision sets the number of places after the decimal, if appropriate,
	except that for %g/%G precision sets the maximum number of significant
	digits (trailing zeros are removed). For example, given 12.345 the format
	%6.3f prints 12.345 while %.3g prints 12.3. The default precision for %e, %f
	and %#g is 6; for %g it is the smallest number of digits necessary to identify
	the value uniquely.

    对于复数，宽度和精度分别应用于两个分量，并且将结果括在括号中，因此将%f应用于1.2+3.4i生成（1.200000 + 3.400000i）。
	For complex numbers, the width and precision apply to the two
	components independently and the result is parenthesized, so %f applied
	to 1.2+3.4i produces (1.200000+3.400000i).

    其它flag：

        '+'	总是输出数值的正负号；对%q（%+q）会生成全部是ASCII字符的输出（通过转义）；
        ' '	对数值，正数前加空格而负数前加负号；
        '-'	在输出右边填充空白而不是默认的左边（即从默认的右对齐切换为左对齐）；
        '#'	切换格式：
          	八进制数前加0（%#o），十六进制数前加0x（%#x）或0X（%#X），指针去掉前面的0x（%#p）；
         	对%q（%#q），如果strconv.CanBackquote返回真会输出反引号括起来的未转义字符串；
         	对%U（%#U），输出Unicode格式后，如字符可打印，还会输出空格和单引号括起来的go字面值；
          	对字符串采用%x或%X时（% x或% X）会给各打印的字节之间加空格；
        '0'	使用0而不是空格填充，对于数值类型会把填充的0放在正负号后面；
	Other flags:
		+	always print a sign for numeric values;
			guarantee ASCII-only output for %q (%+q)
		-	pad with spaces on the right rather than the left (left-justify the field)
		#	alternate format: add leading 0b for binary (%#b), 0 for octal (%#o),
			0x or 0X for hex (%#x or %#X); suppress 0x for %p (%#p);
			for %q, print a raw (backquoted) string if strconv.CanBackquote
			returns true;
			always print a decimal point for %e, %E, %f, %F, %g and %G;
			do not remove trailing zeros for %g and %G;
			write e.g. U+0078 'x' if the character is printable for %U (%#U).
		' '	(space) leave a space for elided sign in numbers (% d);
			put spaces between bytes printing strings or slices in hex (% x, % X)
		0	pad with leading zeros rather than spaces;
			for numbers, this moves the padding after the sign

    verb会忽略不支持的flag。例如，因为没有十进制切换模式，所以%#d和%d的输出是相同的。
	Flags are ignored by verbs that do not expect them.
	For example there is no alternate decimal format, so %#d and %d
	behave identically.

    对每一个类似Printf的函数，都有对应的Print型函数，该函数不接受格式字符串，就效果上等价于对每一个参数都是用verb %v。另一个变体Println型函数会在各个操作数的输出之间加空格并在最后换行。
	For each Printf-like function, there is also a Print function
	that takes no format and is equivalent to saying %v for every
	operand.  Another variant Println inserts blanks between
	operands and appends a newline.

    无论verb如何，如果操作数是接口值，那么将使用内部具体值，而不是接口本身。因此下面会输出23
	Regardless of the verb, if an operand is an interface value,
	the internal concrete value is used, not the interface itself.
	Thus:
		var i interface{} = 23
		fmt.Printf("%v\n", i)
	will print 23.

    除了verb %T和%p之外；对实现了特定接口的操作数会考虑采用特殊的格式化技巧。按应用优先级如下：
	Except when printed using the verbs %T and %p, special
	formatting considerations apply for operands that implement
	certain interfaces. In order of application:
    
    1. 如果操作数是reflect.Value，则将操作数替换为其所保存的具体值，并使用下一个规则继续打印。
	1. If the operand is a reflect.Value, the operand is replaced by the
	concrete value that it holds, and printing continues with the next rule.

    2. 如果一个操作数实现了Formatter接口，它将被调用。 Formatter提供了对格式的精细控制。
	2. If an operand implements the Formatter interface, it will
	be invoked. Formatter provides fine control of formatting.

    3. 如果动作%v与#标志（%#v）一起使用，并且操作数实现GoStringer接口，则将调用该接口。如果格式（对于Println等，隐式为%v）对字符串（%s%q%v%x%X）有效，则以下两个规则适用：
	3. If the %v verb is used with the # flag (%#v) and the operand
	implements the GoStringer interface, that will be invoked.

	If the format (which is implicitly %v for Println etc.) is valid
	for a string (%s %q %v %x %X), the following two rules apply:

    4. 如果操作数实现错误接口，则将调用Error方法将对象转换为字符串，然后根据动词的要求对其进行格式化（如果有）。
	4. If an operand implements the error interface, the Error method
	will be invoked to convert the object to a string, which will then
	be formatted as required by the verb (if any).

    5. 如果操作数实现String()字符串方法，则将调用该方法将对象转换为字符串，然后根据动词的要求对其进行格式化（如果有）。
	5. If an operand implements method String() string, that method
	will be invoked to convert the object to a string, which will then
	be formatted as required by the verb (if any).

    对于切片和结构之类的复合操作数（复合类型），格式递归地应用于每个操作数的元素，而不是整个操作数。因此％q将引用字符串切片中的每个元素，而％6.2f将控制浮点数组中每个元素的格式。
	For compound operands such as slices and structs, the format
	applies to the elements of each operand, recursively, not to the
	operand as a whole. Thus %q will quote each element of a slice
	of strings, and %6.2f will control formatting for each element
	of a floating-point array.

    但是，在打印类字符串（string-like）动作（%s%q%x%X）的字节片时，将其与字符串等同地视为一个项。
	However, when printing a byte slice with a string-like verb
	(%s %q %x %X), it is treated identically to a string, as a single item.

    为了避免可能出现的无穷递归，如：
	To avoid recursion in cases such as
		type X string
		func (x X) String() string { return Sprintf("<%s>", x) }
	convert the value before recurring:
		func (x X) String() string { return Sprintf("<%s>", string(x)) }
	无限递归也可以由自引用数据结构触发，例如包含自身作为元素的切片（如果该类型具有String方法）。但是，这种错误（pathologies）情况很少见，而且软件包也无法防止它们。
	Infinite recursion can also be triggered by self-referential data
	structures, such as a slice that contains itself as an element, if
	that type has a String method. Such pathologies are rare, however,
	and the package does not protect against them.

    当打印结构体数据之时，fmt无法并且因此不会在未导出的字段上调用诸如Error或String之类的格式化方法。
	When printing a struct, fmt cannot and therefore does not invoke
	formatting methods such as Error or String on unexported fields.

    显式指定参数索引：
	Explicit argument indexes:

    在Printf、Sprintf、Fprintf三个函数中，默认的行为是对每一个格式化verb依次对应调用时成功传递进来的参数。但是，紧跟在verb之前的[n]符号表示应格式化第n个参数（索引从1开始）。同样的在'*'之前的[n]符号表示采用第n个参数的值作为宽度或精度。在处理完方括号表达式[n]后，除非另有指示，会接着处理参数n+1，n+2，...（就是说移动了当前处理位置）。
	In Printf, Sprintf, and Fprintf, the default behavior is for each
	formatting verb to format successive arguments passed in the call.
	However, the notation [n] immediately before the verb indicates that the
	nth one-indexed argument is to be formatted instead. The same notation
	before a '*' for a width or precision selects the argument index holding
	the value. After processing a bracketed expression [n], subsequent verbs
	will use arguments n+1, n+2, etc. unless otherwise directed.

    例如：
	For example,
		fmt.Sprintf("%[2]d %[1]d\n", 11, 22)
	会生成"22 11"，而：
	will yield "22 11", while
		fmt.Sprintf("%[3]*.[2]*[1]f", 12.0, 2, 6) // TODO 怎么理解*
	等价于
	equivalent to
		fmt.Sprintf("%6.2f", 12.0)
	会生成" 12.00"。因为显式的索引会影响随后的verb，这种符号可以通过重设索引用于多次打印同一个值：
	will yield " 12.00". Because an explicit index affects subsequent verbs,
	this notation can be used to print the same values multiple times
	by resetting the index for the first argument to be repeated:
		fmt.Sprintf("%d %d %#[1]x %#x", 16, 17)
	会生成"16 17 0x10 0x11"
	will yield "16 17 0x10 0x11".

    格式化错误：
	Format errors:

    如果给某个verb提供了非法的参数，如给%d提供了一个字符串，生成的字符串会包含该问题的描述，如下所例：
        错误的类型或未知的verb：            %!verb(type=value)
        	Printf("%d", hi):               %!d(string=hi)
        太多参数（采用索引时会失效）：      %!(EXTRA type=value)
        	Printf("hi", "guys"):           hi%!(EXTRA string=guys)
        太少参数:                           %!verb(MISSING)
        	Printf("hi%d"):                 hi %!d(MISSING)
        宽度/精度不是整数值：               %!(BADWIDTH) or %!(BADPREC)
        	Printf("%*s", 4.5, "hi"):       %!(BADWIDTH)hi
        	Printf("%.*s", 4.5, "hi"):      %!(BADPREC)hi
        没有索引指向的参数：                %!(BADINDEX)
        	Printf("%*[2]d", 7):            %!d(BADINDEX)
        	Printf("%.[2]d", 7):            %!d(BADINDEX)
    
	If an invalid argument is given for a verb, such as providing
	a string to %d, the generated string will contain a
	description of the problem, as in these examples:

		Wrong type or unknown verb: %!verb(type=value)
			Printf("%d", "hi"):        %!d(string=hi)
		Too many arguments: %!(EXTRA type=value)
			Printf("hi", "guys"):      hi%!(EXTRA string=guys)
		Too few arguments: %!verb(MISSING)
			Printf("hi%d"):            hi%!d(MISSING)
		Non-int for width or precision: %!(BADWIDTH) or %!(BADPREC)
			Printf("%*s", 4.5, "hi"):  %!(BADWIDTH)hi
			Printf("%.*s", 4.5, "hi"): %!(BADPREC)hi
		Invalid or invalid use of argument index: %!(BADINDEX)
			Printf("%*[2]d", 7):       %!d(BADINDEX)
			Printf("%.[2]d", 7):       %!d(BADINDEX)

    所有的错误都以字符串"%!"开始，有时会后跟单个字符（verb标识符），并以加小括弧的描述结束。
	All errors begin with the string "%!" followed sometimes
	by a single character (the verb) and end with a parenthesized
	description.

    如果被print系列函数调用时，Error或String方法触发了panic，fmt包会根据panic重建错误信息，用一个字符串说明该panic经过了fmt包。例如，一个String方法调用了panic("bad")，生成的格式化信息差不多是这样的：
        %!s(PANIC=bad)
    %!s指示表示错误（panic）出现时的使用的verb。
	If an Error or String method triggers a panic when called by a
	print routine, the fmt package reformats the error message
	from the panic, decorating it with an indication that it came
	through the fmt package.  For example, if a String method
	calls panic("bad"), the resulting formatted message will look
	like
		%!s(PANIC=bad)

    %!s仅显示失败发生时使用的打印动词(the print verb)。如果panic是由错误或String方法的nil接收器导致，则输出的是未修饰的字符串“ <nil>”。
	The %!s just shows the print verb in use when the failure
	occurred. If the panic is caused by a nil receiver to an Error
	or String method, however, the output is the undecorated
	string, "<nil>".

    扫描
	Scanning

    一组类似的函数会扫描格式化的文本以产生值。Scan，Scanf和Scanln从os.Stdin中读取值；Fscan，Fscanf和Fscanln从指定的io.Reader读取值；Sscan，Sscanf和Sscanln从参数字符串中读取值。
	An analogous set of functions scans formatted text to yield
	values.  Scan, Scanf and Scanln read from os.Stdin; Fscan,
	Fscanf and Fscanln read from a specified io.Reader; Sscan,
	Sscanf and Sscanln read from an argument string.

    Scan，Fscan，Sscan将输入中的换行符视为空格。
	Scan, Fscan, Sscan treat newlines in the input as spaces.

    Scanln、Fscanln、Sscanln会在读取到换行时停止，并要求在这些读取数据之后加上换行符或EOF。
	Scanln, Fscanln and Sscanln stop scanning at a newline and
	require that the items be followed by a newline or EOF.

    Scanf，Fscanf和Sscanf根据类似于Printf的格式字符串解析参数。 在下面的文本中，“空格（space）”表示除换行符外的任何Unicode空格字符。
	Scanf, Fscanf, and Sscanf parse the arguments according to a
	format string, analogous to that of Printf. In the text that
	follows, 'space' means any Unicode whitespace character
	except newline.

    在格式字符串中，由％字符引入的动词消耗并解析输入；这些动词将在下面更详细地描述。格式中除％，空格或换行符以外的其他字符将完全消耗该输入字符，该字符必须存在。 格式字符串中包含零个或多个空格的换行符会在输入中占用零个或多个空格，后跟一个换行符或输入结尾。格式字符串中换行符后的空格在输入中消耗零个或多个空格。否则，格式字符串中任何一个或多个空格的运行都会在输入中消耗尽可能多的空格。除非格式字符串中的空格行与换行符相邻，否则该行必须占用输入中至少一个空格或找到输入的结尾。
	In the format string, a verb introduced by the % character
	consumes and parses input; these verbs are described in more
	detail below. A character other than %, space, or newline in
	the format consumes exactly that input character, which must
	be present. A newline with zero or more spaces before it in
	the format string consumes zero or more spaces in the input
	followed by a single newline or the end of the input. A space
	following a newline in the format string consumes zero or more
	spaces in the input. Otherwise, any run of one or more spaces
	in the format string consumes as many spaces as possible in
	the input. Unless the run of spaces in the format string
	appears adjacent to a newline, the run must consume at least
	one space from the input or find the end of the input.

    空格和换行符的处理与C的scanf系列不同：在C中，换行符被视为其他任何空格，并且当格式字符串中的空格运行在输入中找不到要使用的空格时，这绝不是错误。
	The handling of spaces and newlines differs from that of C's
	scanf family: in C, newlines are treated as any other space,
	and it is never an error when a run of spaces in the format
	string finds no spaces to consume in the input.

    这些动词的行为类似于Printf的行为。例如，%x将扫描一个整数作为十六进制数，%v将扫描默认表示形式的值。未实现Printf动词%p和%T以及标志#和+。对于浮点和复数值，所有有效的格式化动词（%b%e%E%f%F%g%G%x%X和%v）都是等效的，并且接受十进制和十六进制表示法（例如："2.3e+7"，"0x4.5p-8"）和数字分隔的下划线（例如："3.14159_26535_89793"）。
	The verbs behave analogously to those of Printf.
	For example, %x will scan an integer as a hexadecimal number,
	and %v will scan the default representation format for the value.
	The Printf verbs %p and %T and the flags # and + are not implemented.
	For floating-point and complex values, all valid formatting verbs
	(%b %e %E %f %F %g %G %x %X and %v) are equivalent and accept
	both decimal and hexadecimal notation (for example: "2.3e+7", "0x4.5p-8")
	and digit-separating underscores (for example: "3.14159_26535_89793").

    动词处理的输入隐式用空格分隔：除%c外，每个动词的实现都从其余输入中删除前导空格开始，%s动词（和%v读入字符串）在用第一个空格或换行符时停止消费。
	Input processed by verbs is implicitly space-delimited: the
	implementation of every verb except %c starts by discarding
	leading spaces from the remaining input, and the %s verb
	(and %v reading into a string) stops consuming input at the first
	space or newline character.

    扫描不带格式或带有％v动词的整数时，接受熟悉的基本设置前缀0b（二进制），0o和0（八进制）和0x（十六进制），以及以数字分隔的下划线。
	The familiar base-setting prefixes 0b (binary), 0o and 0 (octal),
	and 0x (hexadecimal) are accepted when scanning integers
	without a format or with the %v verb, as are digit-separating
	underscores.

    宽度（width）在输入文本中解释，但是没有用于精确扫描的语法（没有％5.2f，只有％5f）。如果提供了宽度，它将在修剪前导空格后应用，并指定为满足动词而需要阅读的最大字符数。 例如，
	Width is interpreted in the input text but there is no
	syntax for scanning with a precision (no %5.2f, just %5f).
	If width is provided, it applies after leading spaces are
	trimmed and specifies the maximum number of runes to read
	to satisfy the verb. For example,
	   Sscanf(" 1234567 ", "%5s%d", &s, &i)
	会将s设置为"12345"，将i设置为67
	will set s to "12345" and i to 67 while
	   Sscanf(" 12 34 567 ", "%5s%d", &s, &i)
	将s设置为"12"，将i设置为34。
	will set s to "12" and i to 34.

    在所有扫描方法中，回车符后紧跟换行符被视为普通换行符（\r\n表示与\n相同）。
	In all the scanning functions, a carriage return followed
	immediately by a newline is treated as a plain newline
	(\r\n means the same as \n).

    在所有扫描功能中，如果操作数实现了Scan方法（即，它实现了Scanner接口），则该方法将用于扫描该操作数的文本。另外，如果扫描的参数数量小于提供的参数数量，则会返回错误。
	In all the scanning functions, if an operand implements method
	Scan (that is, it implements the Scanner interface) that
	method will be used to scan the text for that operand.  Also,
	if the number of arguments scanned is less than the number of
	arguments provided, an error is returned.

    所有要扫描的参数都必须是指向Scanner接口的基本类型或实现的指针。
	All arguments to be scanned must be either pointers to basic
	types or implementations of the Scanner interface.

    像Scanf和Fscanf一样，Sscanf不需要消耗其全部输入。 无法恢复Sscanf使用了多少输入字符串。
	Like Scanf and Fscanf, Sscanf need not consume its entire input.
	There is no way to recover how much of the input string Sscanf used.

    注意：Fscan等可以在返回的输入之后读取一个字符（符文），这意味着调用扫描例程的循环可能会跳过某些输入。仅当输入值之间没有空格时，这通常是一个问题。 如果提供给Fscan的Reader实现了ReadRune，则该方法将用于读取字符。如果Reader还实现了UnreadRune，则将使用该方法保存字符，并且后续调用不会丢失数据。要将ReadRune和UnreadRune方法附加到没有该功能的Reader，请使用bufio.NewReader。
	Note: Fscan etc. can read one character (rune) past the input
	they return, which means that a loop calling a scan routine
	may skip some of the input.  This is usually a problem only
	when there is no space between input values.  If the reader
	provided to Fscan implements ReadRune, that method will be used
	to read characters.  If the reader also implements UnreadRune,
	that method will be used to save the character and successive
	calls will not lose data.  To attach ReadRune and UnreadRune
	methods to a reader without that capability, use
	bufio.NewReader.
*/
package fmt
```