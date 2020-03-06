```go
package fmt

import "errors"

// Errorf根据格式说明进行格式化，并将字符串作为满足错误的值返回。
//
// 如果格式说明符包含带有错误操作数的%w动词，则返回的错误将实现Unwrap方法，以返回操作数。 
// 包含多个%w动词或向其提供未实现错误接口的操作数是无效的。 另外，%w动词是%v的同义词。
func Errorf(format string, a ...interface{}) error {
    p := newPrinter()
    p.wrapErrs = true
    p.doPrintf(format, a)
    s := string(p.buf)
    var err error
    // 没有包装错误（底层错误）就创建一个普通错误，否则就创建一个包装错误
    if p.wrappedErr == nil {
        err = errors.New(s)
    } else {
        err = &wrapError{s, p.wrappedErr}
    }
    p.free()
    return err
}

// 包装错误
type wrapError struct {
    msg string  // 错误描述
    err error   // 底层错误
}

func (e *wrapError) Error() string {
    return e.msg
}

func (e *wrapError) Unwrap() error {
    return e.err
}
```
