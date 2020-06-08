```go
// Package context defines the Context type, which carries deadlines,
// cancellation signals, and other request-scoped values across API boundaries
// and between processes.
//
// Incoming requests to a server should create a Context, and outgoing
// calls to servers should accept a Context. The chain of function
// calls between them must propagate the Context, optionally replacing
// it with a derived Context created using WithCancel, WithDeadline,
// WithTimeout, or WithValue. When a Context is canceled, all
// Contexts derived from it are also canceled.
//
// The WithCancel, WithDeadline, and WithTimeout functions take a
// Context (the parent) and return a derived Context (the child) and a
// CancelFunc. Calling the CancelFunc cancels the child and its
// children, removes the parent's reference to the child, and stops
// any associated timers. Failing to call the CancelFunc leaks the
// child and its children until the parent is canceled or the timer
// fires. The go vet tool checks that CancelFuncs are used on all
// control-flow paths.
//
// Programs that use Contexts should follow these rules to keep interfaces
// consistent across packages and enable static analysis tools to check context
// propagation:
//
// Do not store Contexts inside a struct type; instead, pass a Context
// explicitly to each function that needs it. The Context should be the first
// parameter, typically named ctx:
//
// 	func DoSomething(ctx context.Context, arg Arg) error {
// 		// ... use ctx ...
// 	}
//
// Do not pass a nil Context, even if a function permits it. Pass context.TODO
// if you are unsure about which Context to use.
//
// Use context Values only for request-scoped data that transits processes and
// APIs, not for passing optional parameters to functions.
//
// The same Context may be passed to functions running in different goroutines;
// Contexts are safe for simultaneous use by multiple goroutines.
//
// See https://blog.golang.org/context for example code for a server that uses
// Contexts.
//
// 包上下文定义了上下文类型，该类型在API边界之间以及进程之间传递截止日期，取消信号和其他请求范围的值。
//
// 对服务器的传入请求应创建一个上下文，而对服务器的传出调用应接受一个上下文。它们之间的函数调用链必须传播Context，
// 可以选择将其替换为使用WithCancel，WithDeadline，WithTimeout或WithValue创建的派生Context。
// 取消上下文后，从该上下文派生的所有上下文也会被取消。
//
// WithCancel，WithDeadline和WithTimeout函数采用Context（父级）并返回派生的Context（子级）和CancelFunc。
// 调用CancelFunc会取消子级及其子级，删除父级对该子级的引用，并停止所有关联的计时器。
// 未能调用CancelFunc会使子代及其子代泄漏，直到父代被取消或计时器触发。审核工具检查所有控制流路径上是否都使用了CancelFuncs。
//
// 使用上下文的程序应遵循以下规则，以使各个包之间的接口保持一致，并启用静态分析工具来检查上下文传播：
//
// 不要将上下文存储在结构类型中；而是将上下文明确传递给需要它的每个函数。 Context应该是第一个参数，通常命名为ctx：
//
//  func DoSomething（ctx context.Context，arg Arg）error {
//      // ...使用ctx ...
//  }
//
// 即使函数允许，也不要传递nil Context。如果不确定使用哪个上下文，请传递context.TODO。
//
// 仅将上下文值用于传递过程和API的请求范围数据，而不用于将可选参数传递给函数。
//
// 相同的上下文可以传递给在不同goroutine中运行的函数；上下文对于由多个goroutine同时使用是安全的。
//
// 有关使用Context的服务器的示例代码，请参见https://blog.golang.org/context。
package context

import (
	"errors"
	"internal/reflectlite"
	"sync"
	"sync/atomic"
	"time"
)

// A Context carries a deadline, a cancellation signal, and other values across
// API boundaries.
//
// Context's methods may be called by multiple goroutines simultaneously.
//
//上下文在API边界上带有期限，取消信号和其他值。
//
//多个goroutine可以同时调用Context的方法。
type Context interface {
	// Deadline returns the time when work done on behalf of this context
	// should be canceled. Deadline returns ok==false when no deadline is
	// set. Successive calls to Deadline return the same results.
	//
	// 截止日期返回应取消代表该上下文完成的工作的时间。如果未设置截止日期，则截止日期返回ok == false。
	// 连续调用Deadline会返回相同的结果。
	Deadline() (deadline time.Time, ok bool)

	// Done returns a channel that's closed when work done on behalf of this
	// context should be canceled. Done may return nil if this context can
	// never be canceled. Successive calls to Done return the same value.
	// The close of the Done channel may happen asynchronously,
	// after the cancel function returns.
	//
	// WithCancel arranges for Done to be closed when cancel is called;
	// WithDeadline arranges for Done to be closed when the deadline
	// expires; WithTimeout arranges for Done to be closed when the timeout
	// elapses.
	//
	// Done is provided for use in select statements:
	//
	//  // Stream generates values with DoSomething and sends them to out
	//  // until DoSomething returns an error or ctx.Done is closed.
	//  func Stream(ctx context.Context, out chan<- Value) error {
	//  	for {
	//  		v, err := DoSomething(ctx)
	//  		if err != nil {
	//  			return err
	//  		}
	//  		select {
	//  		case <-ctx.Done():
	//  			return ctx.Err()
	//  		case out <- v:
	//  		}
	//  	}
	//  }
	//
	// See https://blog.golang.org/pipelines for more examples of how to use
	// a Done channel for cancellation.
	//
	// Done返回一个通道，当取消代表该上下文的工作时，该通道已关闭。如果此上下文永远无法取消，则可能会返回nil。
	// 连续调用Done将返回相同的值。在取消函数返回后，完成通道的关闭可能异步发生。
    //
    // WithCancel安排在调用cancel时关闭Done；
    // WithDeadline安排在截止日期到期时关闭“Done”；
    // WithTimeout安排超时后关闭“Done”。
    //
    //提供了在select语句中使用的功能：
    //
    // // 流使用DoSomething生成值并将其发送出去，直到DoSomething返回错误或ctx.Done关闭为止。
	//  func Stream(ctx context.Context, out chan<- Value) error {
	//  	for {
	//  		v, err := DoSomething(ctx)
	//  		if err != nil {
	//  			return err
	//  		}
	//  		select {
	//  		case <-ctx.Done():
	//  			return ctx.Err()
	//  		case out <- v:
	//  		}
	//  	}
	//  }
    //
    // 有关如何使用“Done”通道进行取消的更多示例，请参见https://blog.golang.org/pipelines。
	Done() <-chan struct{}

	// If Done is not yet closed, Err returns nil.
	// If Done is closed, Err returns a non-nil error explaining why:
	// Canceled if the context was canceled
	// or DeadlineExceeded if the context's deadline passed.
	// After Err returns a non-nil error, successive calls to Err return the same error.
	//
	// 如果尚未关闭Done，则Err返回nil。
    // 如果Done关闭，则Err返回一个非nil错误，解释原因：如果上下文已取消，则取消；如果上下文的截止日期已过，则DeadlineExceeded。
    // Err返回非nil错误后，对Err的连续调用将返回相同的错误。
	Err() error

	// Value returns the value associated with this context for key, or nil
	// if no value is associated with key. Successive calls to Value with
	// the same key returns the same result.
	//
	// Use context values only for request-scoped data that transits
	// processes and API boundaries, not for passing optional parameters to
	// functions.
	//
	// A key identifies a specific value in a Context. Functions that wish
	// to store values in Context typically allocate a key in a global
	// variable then use that key as the argument to context.WithValue and
	// Context.Value. A key can be any type that supports equality;
	// packages should define keys as an unexported type to avoid
	// collisions.
	//
	// Packages that define a Context key should provide type-safe accessors
	// for the values stored using that key:
	//
	// 	// Package user defines a User type that's stored in Contexts.
	// 	package user
	//
	// 	import "context"
	//
	// 	// User is the type of value stored in the Contexts.
	// 	type User struct {...}
	//
	// 	// key is an unexported type for keys defined in this package.
	// 	// This prevents collisions with keys defined in other packages.
	// 	type key int
	//
	// 	// userKey is the key for user.User values in Contexts. It is
	// 	// unexported; clients use user.NewContext and user.FromContext
	// 	// instead of using this key directly.
	// 	var userKey key
	//
	// 	// NewContext returns a new Context that carries value u.
	// 	func NewContext(ctx context.Context, u *User) context.Context {
	// 		return context.WithValue(ctx, userKey, u)
	// 	}
	//
	// 	// FromContext returns the User value stored in ctx, if any.
	// 	func FromContext(ctx context.Context) (*User, bool) {
	// 		u, ok := ctx.Value(userKey).(*User)
	// 		return u, ok
	// 	}
	//
	// Value返回与此键的上下文关联的值；如果没有值与key关联，则返回nil。使用相同的键连续调用Value会返回相同的结果。
    //
    // 仅将上下文值用于传递过程和API边界的请求范围数据，而不用于将可选参数传递给函数。
    //
    // 关键字标识上下文中的特定值。希望在Context中存储值的函数通常会在全局变量中分配一个键，
    // 然后将该键用作context.WithValue和Context.Value的参数。密钥可以是任何支持相等性的类型。
    // 软件包应将键定义为未导出的类型，以免发生冲突。
    //
    // 定义上下文key的包应为使用该key存储的值提供类型安全的访问器：
	// 	// 包用户定义存储在上下文中的用户类型。
	// 	package user
	//
	// 	import "context"
	//
	// 	// 用户是存储在上下文中的值的类型。
	// 	type User struct {...}
	//
	// 	// key是此软件包中定义的key的未导出类型。这样可以防止与其他包中定义的键冲突。
	// 	type key int
	//
	// 	// userKey是user.User上下文中的值的键。它是未导出的；客户端使用user.NewContext和user.FromContext而不是直接使用此键。
	// 	var userKey key
	//
	// 	// NewContext返回带有值u的新Context。
	// 	func NewContext(ctx context.Context, u *User) context.Context {
	// 		return context.WithValue(ctx, userKey, u)
	// 	}
	//
	// 	// FromContext返回存储在ctx中的User值（如果有）。
	// 	func FromContext(ctx context.Context) (*User, bool) {
	// 		u, ok := ctx.Value(userKey).(*User)
	// 		return u, ok
	// 	}

	Value(key interface{}) interface{}
}

// Canceled is the error returned by Context.Err when the context is canceled.
// Canceled是Context.Err取消上下文时返回的错误。
var Canceled = errors.New("context canceled")

// DeadlineExceeded is the error returned by Context.Err when the context's
// deadline passes.
// DeadlineExceeded是上下文的截止日期过去时Context.Err返回的错误。
var DeadlineExceeded error = deadlineExceededError{}

type deadlineExceededError struct{}

func (deadlineExceededError) Error() string   { return "context deadline exceeded" }
func (deadlineExceededError) Timeout() bool   { return true }
func (deadlineExceededError) Temporary() bool { return true }

// An emptyCtx is never canceled, has no values, and has no deadline. It is not
// struct{}, since vars of this type must have distinct addresses.
// emptyCtx永远不会取消，没有值，也没有截止日期。它不是struct{}，因为此类型的var必须具有不同的地址。
type emptyCtx int

func (*emptyCtx) Deadline() (deadline time.Time, ok bool) {
	return
}

func (*emptyCtx) Done() <-chan struct{} {
	return nil
}

func (*emptyCtx) Err() error {
	return nil
}

func (*emptyCtx) Value(key interface{}) interface{} {
	return nil
}

func (e *emptyCtx) String() string {
	switch e {
	case background:
		return "context.Background"
	case todo:
		return "context.TODO"
	}
	return "unknown empty Context"
}

var (
	background = new(emptyCtx)
	todo       = new(emptyCtx)
)

// Background returns a non-nil, empty Context. It is never canceled, has no
// values, and has no deadline. It is typically used by the main function,
// initialization, and tests, and as the top-level Context for incoming
// requests.
// Background返回一个非空的Context。 它永远不会被取消，没有值，也没有期限。 它通常由主要功能，初始化和测试使用，并用作传入请求的顶级上下文。
func Background() Context {
	return background
}

// TODO returns a non-nil, empty Context. Code should use context.TODO when
// it's unclear which Context to use or it is not yet available (because the
// surrounding function has not yet been extended to accept a Context
// parameter).
// TODO返回非空的Context。 当不清楚要使用哪个上下文或尚不可用时（因为周围的函数尚未扩展为接受Context参数），代码应使用context.TODO。
func TODO() Context {
	return todo
}

// A CancelFunc tells an operation to abandon its work.
// A CancelFunc does not wait for the work to stop.
// A CancelFunc may be called by multiple goroutines simultaneously.
// After the first call, subsequent calls to a CancelFunc do nothing.
// CancelFunc的使用原则
// CancelFunc告诉操作放弃其工作。
// CancelFunc不等待工作停止。
// 多个goroutine可以同时调用CancelFunc。
// 在第一个调用之后，随后对CancelFunc的调用什么都不做。
type CancelFunc func()

// WithCancel returns a copy of parent with a new Done channel. The returned
// context's Done channel is closed when the returned cancel function is called
// or when the parent context's Done channel is closed, whichever happens first.
//
// Canceling this context releases resources associated with it, so code should
// call cancel as soon as the operations running in this Context complete.
//
// WithCancel返回具有新的Done通道的父级副本。 当调用返回的cancel函数或关闭父上下文的Done通道时（以先发生的为准），关闭返回的上下文的Done通道。
//
// 取消此上下文将释放与其关联的资源，因此在此上下文中运行的操作完成后，代码应立即调用cancel。
func WithCancel(parent Context) (ctx Context, cancel CancelFunc) {
	c := newCancelCtx(parent)
	propagateCancel(parent, &c)
	return &c, func() { c.cancel(true, Canceled) }
}

// newCancelCtx returns an initialized cancelCtx.
// newCancelCtx返回一个初始化的cancelCtx。
func newCancelCtx(parent Context) cancelCtx {
	return cancelCtx{Context: parent}
}

// goroutines counts the number of goroutines ever created; for testing.
// goroutines计算曾经创建的goroutine的数量； 用于检测。
var goroutines int32

// propagateCancel arranges for child to be canceled when parent is.
// 当parent取消时，propagateCancel将子context了取消。
func propagateCancel(parent Context, child canceler) {
	done := parent.Done()
	if done == nil {
		return // parent is never canceled // 父级永远不会被取消
	}

	select {
	case <-done:
		// parent is already canceled // 父级已被取消
		child.cancel(false, parent.Err())
		return
	default:
	}

	if p, ok := parentCancelCtx(parent); ok {
		p.mu.Lock()
		if p.err != nil {
			// parent has already been canceled // 父级已被取消
			child.cancel(false, p.err)
		} else {
			if p.children == nil {
				p.children = make(map[canceler]struct{})
			}
			p.children[child] = struct{}{}
		}
		p.mu.Unlock()
	} else {
		atomic.AddInt32(&goroutines, +1)
		go func() {
			select {
			case <-parent.Done():
				child.cancel(false, parent.Err())
			case <-child.Done():
			}
		}()
	}
}

// &cancelCtxKey is the key that a cancelCtx returns itself for.
// &cancelCtxKey是cancelCtx返回其自身的键。
var cancelCtxKey int

// parentCancelCtx returns the underlying *cancelCtx for parent.
// It does this by looking up parent.Value(&cancelCtxKey) to find
// the innermost enclosing *cancelCtx and then checking whether
// parent.Done() matches that *cancelCtx. (If not, the *cancelCtx
// has been wrapped in a custom implementation providing a
// different done channel, in which case we should not bypass it.)
//
// parentCancelCtx返回父级的底层*cancelCtx。 它通过查找parent.Value（&cancelCtxKey）来查找最里面的*cancelCtx，
// 然后检查parent.Done()是否与该* cancelCtx相匹配来实现此目的。
//（如果没有，*cancelCtx已包装在提供不同完成通道的自定义实现中，在这种情况下，我们不应绕过它。）
func parentCancelCtx(parent Context) (*cancelCtx, bool) {
	done := parent.Done()
	if done == closedchan || done == nil {
		return nil, false
	}
	p, ok := parent.Value(&cancelCtxKey).(*cancelCtx)
	if !ok {
		return nil, false
	}
	p.mu.Lock()
	ok = p.done == done
	p.mu.Unlock()
	if !ok {
		return nil, false
	}
	return p, true
}

// removeChild removes a context from its parent.
// removeChild从其父级中删除上下文。
func removeChild(parent Context, child canceler) {
	p, ok := parentCancelCtx(parent)
	if !ok {
		return
	}
	p.mu.Lock()
	if p.children != nil {
		delete(p.children, child)
	}
	p.mu.Unlock()
}

// A canceler is a context type that can be canceled directly. The
// implementations are *cancelCtx and *timerCtx.
// 取消器是可以直接取消的上下文类型。 实现是*cancelCtx和*timerCtx。
type canceler interface {
	cancel(removeFromParent bool, err error)
	Done() <-chan struct{}
}

// closedchan is a reusable closed channel.
// closechan是可重用的关闭通道。
var closedchan = make(chan struct{})

func init() {
	close(closedchan)
}

// A cancelCtx can be canceled. When canceled, it also cancels any children
// that implement canceler.
// cancelCtx可以被取消。 取消后，它也会取消所有实现取消方法的子级。
type cancelCtx struct {
	Context

	mu       sync.Mutex            // protects following fields // 保护以下字段
	done     chan struct{}         // created lazily, closed by first cancel call // 延迟创建，通过第一个取消调用关闭
	children map[canceler]struct{} // set to nil by the first cancel call // 通过第一个取消调用设置为nil
	err      error                 // set to non-nil by the first cancel call // 通过第一个取消调用设置为非零
}

func (c *cancelCtx) Value(key interface{}) interface{} {
	if key == &cancelCtxKey { // 如果是cancelCtxKey的指针，就返回cancelCtx，否则返回Context中的值
		return c
	}
	return c.Context.Value(key)
}

func (c *cancelCtx) Done() <-chan struct{} {
	c.mu.Lock()
	if c.done == nil {
		c.done = make(chan struct{})
	}
	d := c.done
	c.mu.Unlock()
	return d
}

func (c *cancelCtx) Err() error {
	c.mu.Lock()
	err := c.err
	c.mu.Unlock()
	return err
}

type stringer interface {
	String() string
}

func contextName(c Context) string {
	if s, ok := c.(stringer); ok {
		return s.String()
	}
	return reflectlite.TypeOf(c).String()
}

func (c *cancelCtx) String() string {
	return contextName(c.Context) + ".WithCancel"
}

// cancel closes c.done, cancels each of c's children, and, if
// removeFromParent is true, removes c from its parent's children.
// cancel关闭c.done，取消c的每个子级，如果removeFromParent为true，则从其父级的子级中删除c。
func (c *cancelCtx) cancel(removeFromParent bool, err error) {
	if err == nil {
		panic("context: internal error: missing cancel error")
	}
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return // already canceled
	}
	c.err = err
	if c.done == nil {
		c.done = closedchan
	} else {
		close(c.done)
	}
	for child := range c.children {
		// NOTE: acquiring the child's lock while holding parent's lock.
		// 注意：持有父级锁的同时获取子级锁。
		child.cancel(false, err)
	}
	c.children = nil
	c.mu.Unlock()

	if removeFromParent {
		removeChild(c.Context, c)
	}
}

// WithDeadline returns a copy of the parent context with the deadline adjusted
// to be no later than d. If the parent's deadline is already earlier than d,
// WithDeadline(parent, d) is semantically equivalent to parent. The returned
// context's Done channel is closed when the deadline expires, when the returned
// cancel function is called, or when the parent context's Done channel is
// closed, whichever happens first.
//
// Canceling this context releases resources associated with it, so code should
// call cancel as soon as the operations running in this Context complete.
func WithDeadline(parent Context, d time.Time) (Context, CancelFunc) {
	if cur, ok := parent.Deadline(); ok && cur.Before(d) {
		// The current deadline is already sooner than the new one.
		return WithCancel(parent)
	}
	c := &timerCtx{
		cancelCtx: newCancelCtx(parent),
		deadline:  d,
	}
	propagateCancel(parent, c)
	dur := time.Until(d)
	if dur <= 0 {
		c.cancel(true, DeadlineExceeded) // deadline has already passed
		return c, func() { c.cancel(false, Canceled) }
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err == nil {
		c.timer = time.AfterFunc(dur, func() {
			c.cancel(true, DeadlineExceeded)
		})
	}
	return c, func() { c.cancel(true, Canceled) }
}

// A timerCtx carries a timer and a deadline. It embeds a cancelCtx to
// implement Done and Err. It implements cancel by stopping its timer then
// delegating to cancelCtx.cancel.
type timerCtx struct {
	cancelCtx
	timer *time.Timer // Under cancelCtx.mu.

	deadline time.Time
}

func (c *timerCtx) Deadline() (deadline time.Time, ok bool) {
	return c.deadline, true
}

func (c *timerCtx) String() string {
	return contextName(c.cancelCtx.Context) + ".WithDeadline(" +
		c.deadline.String() + " [" +
		time.Until(c.deadline).String() + "])"
}

func (c *timerCtx) cancel(removeFromParent bool, err error) {
	c.cancelCtx.cancel(false, err)
	if removeFromParent {
		// Remove this timerCtx from its parent cancelCtx's children.
		removeChild(c.cancelCtx.Context, c)
	}
	c.mu.Lock()
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.mu.Unlock()
}

// WithTimeout returns WithDeadline(parent, time.Now().Add(timeout)).
//
// Canceling this context releases resources associated with it, so code should
// call cancel as soon as the operations running in this Context complete:
//
// 	func slowOperationWithTimeout(ctx context.Context) (Result, error) {
// 		ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
// 		defer cancel()  // releases resources if slowOperation completes before timeout elapses
// 		return slowOperation(ctx)
// 	}
//
// WithTimeout 返回 WithDeadline(parent, time.Now().Add(timeout)).
//
// 取消此上下文将释放与其关联的资源，因此在此上下文中运行的操作完成后，代码应立即调用cancel：
//
// 	func slowOperationWithTimeout(ctx context.Context) (Result, error) {
// 		ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
// 		defer cancel()  // 如果slowOperation在超时之前完成，则释放资源
// 		return slowOperation(ctx)
// 	}
func WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc) {
	return WithDeadline(parent, time.Now().Add(timeout))
}

// WithValue returns a copy of parent in which the value associated with key is
// val.
//
// Use context Values only for request-scoped data that transits processes and
// APIs, not for passing optional parameters to functions.
//
// The provided key must be comparable and should not be of type
// string or any other built-in type to avoid collisions between
// packages using context. Users of WithValue should define their own
// types for keys. To avoid allocating when assigning to an
// interface{}, context keys often have concrete type
// struct{}. Alternatively, exported context key variables' static
// type should be a pointer or interface.
//
// WithValue返回父项的副本，其中与key关联的值为val。
//
// 仅将上下文值用于传递流程和API的请求范围数据，而不用于将可选参数传递给函数。
//
// 提供的key必须是可比较的，并且不能为字符串类型或任何其他内置类型，以避免使用上下文在程序包之间发生冲突。
// WithValue的用户应定义自己的key类型。 为了避免在分配给interface{}时分配，上下文键通常具有具体的类型struct{}。
// 另外，导出的上下文键变量的静态类型应该是指针或接口。
func WithValue(parent Context, key, val interface{}) Context {
	if key == nil {
		panic("nil key")
	}
	if !reflectlite.TypeOf(key).Comparable() {
		panic("key is not comparable")
	}
	return &valueCtx{parent, key, val}
}

// A valueCtx carries a key-value pair. It implements Value for that key and
// delegates all other calls to the embedded Context.
//
// valueCtx带有一个键值对。 它为该键实现Value，并将所有其他调用委托给嵌入式Context。
type valueCtx struct {
	Context
	key, val interface{}
}

// stringify tries a bit to stringify v, without using fmt, since we don't
// want context depending on the unicode tables. This is only used by
// *valueCtx.String().
//
// stringify在不使用fmt的情况下尝试对v进行字符串化，因为我们不希望上下文依赖于unicode表。仅由* valueCtx.String()使用。
func stringify(v interface{}) string {
	switch s := v.(type) {
	case stringer:
		return s.String()
	case string:
		return s
	}
	return "<not Stringer>"
}

func (c *valueCtx) String() string {
	return contextName(c.Context) + ".WithValue(type " +
		reflectlite.TypeOf(c.key).String() +
		", val " + stringify(c.val) + ")"
}

func (c *valueCtx) Value(key interface{}) interface{} {
	if c.key == key {
		return c.val
	}
	return c.Context.Value(key)
}
```