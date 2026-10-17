# Concurrency

Contexts, goroutine lifetimes, channels, mutexes, `errgroup` and the race detector. When sources disagree, the Go team wins (Go Code Review Comments, the `context` package doc, go.dev docs and blog), then the Google Go Style Guide, then the Uber Go Style Guide, then recognized authors (Dave Cheney). Every rule names its source; the last section lists where they disagree.

## Contexts

- **Take `ctx context.Context` as the first parameter of any function that does I/O, blocks or may be cancelled.** Why: Go "programs pass Contexts explicitly along the entire function call chain"; the convention lets tools check propagation. Source: [CRC — Contexts](https://go.dev/wiki/CodeReviewComments#contexts), `go doc context`.
- **Never store a context in a struct; pass it to each method that needs it.** Why: a stored context outlives the call it belongs to and hides which operations are cancellable; the only exception is a method whose signature must match an interface you do not own. Source: [CRC — Contexts](https://go.dev/wiki/CodeReviewComments#contexts), `go doc context` ("Do not store Contexts inside a struct type"), [Google decisions — Contexts](https://google.github.io/styleguide/go/decisions#contexts).
- **Create a root context (`context.Background()`) only at entry points (`main`, `init`, a test via `t.Context()`); everywhere else take it from the caller.** Why: "It is very rare for code in the middle of a callchain to require creating a base context of its own." Source: [Google decisions — Contexts](https://google.github.io/styleguide/go/decisions#contexts). With cobra, use `cmd.Context()`. Source: [Google best practices — Complex command-line interfaces](https://google.github.io/styleguide/go/best-practices#complex-command-line-interfaces).
- **Never pass a nil context; use `context.TODO()` when unsure.** Source: `go doc context`.
- **Call the `cancel` returned by `WithCancel`/`WithTimeout`/`WithDeadline` on every path, usually with `defer`.** Why: "Failing to call the CancelFunc leaks the child and its children until the parent is canceled"; `go vet`'s `lostcancel` checks it. Source: `go doc context`.
- **Put request-scoped data in context values, never optional parameters.** Source: `go doc context`, [CRC — Contexts](https://go.dev/wiki/CodeReviewComments#contexts).
- **Do not define custom context types or accept an interface other than `context.Context`.** Source: [CRC — Contexts](https://go.dev/wiki/CodeReviewComments#contexts).
- **For work that must outlive cancellation (a cleanup after the request is cancelled), derive with `context.WithoutCancel(ctx)`** rather than `context.Background()`, so values are kept. Source: `go doc context.WithoutCancel`.

```go
// ❌
type Client struct{ ctx context.Context }
func (c *Client) Fetch(id string) error

// ✅
type Client struct{ http *http.Client }
func (c *Client) Fetch(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	…
}
```

## Prefer synchronous functions

- **Return results directly; let the caller add concurrency.** Why: synchronous functions "keep goroutines localized within a call, making it easier to reason about their lifetimes and avoid leaks and data races", and "it is quite difficult - sometimes impossible - to remove unnecessary concurrency at the caller side." Source: [CRC — Synchronous Functions](https://go.dev/wiki/CodeReviewComments#synchronous-functions), [Google decisions — Synchronous functions](https://google.github.io/styleguide/go/decisions#synchronous-functions), [Dave Cheney — Practical Go, 8.2 Leave concurrency to the caller](https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- **Do not return a channel that a hidden goroutine fills when a slice or a callback will do.** Why: the caller cannot tell an empty result from an error and must drain the channel to let the goroutine exit. Source: [Dave Cheney — Practical Go, 8.2](https://dave.cheney.net/practical-go/presentations/qcon-china.html).

```go
// ❌ starts a goroutine the caller must drain
func List(dir string) <-chan string
// ✅
func List(dir string) ([]string, error)
```

## Goroutine lifetimes

- **Whenever you write `go`, know when that goroutine exits and how the spawner waits for it.** Why: goroutines "can leak by blocking on channel sends or receives: the garbage collector will not terminate a goroutine"; in-flight goroutines cause panics on closed channels, races and unbounded memory. Source: [CRC — Goroutine Lifetimes](https://go.dev/wiki/CodeReviewComments#goroutine-lifetimes), [Google decisions — Goroutine lifetimes](https://google.github.io/styleguide/go/decisions#goroutine-lifetimes), [Dave Cheney — Practical Go, 8.3](https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- **Do not fire and forget: every goroutine has a predictable stop or a stop signal, and a way to wait for it** (`sync.WaitGroup`, a `done` channel, `errgroup`). Source: [Uber — Don't fire-and-forget goroutines](https://github.com/uber-go/guide/blob/master/style.md#dont-fire-and-forget-goroutines), [Wait for goroutines to exit](https://github.com/uber-go/guide/blob/master/style.md#wait-for-goroutines-to-exit).
- **Never start goroutines in `init()`; a background worker is an object with a `Close`/`Stop` that signals and waits.** Source: [Uber — No goroutines in init()](https://github.com/uber-go/guide/blob/master/style.md#no-goroutines-in-init).
- **If the lifetime is not obvious from the code, document when and why the goroutine exits.** Source: [CRC — Goroutine Lifetimes](https://go.dev/wiki/CodeReviewComments#goroutine-lifetimes).
- **Test for leaks with `go.uber.org/goleak` in packages that spawn goroutines.** Source: [Uber — Don't fire-and-forget goroutines](https://github.com/uber-go/guide/blob/master/style.md#dont-fire-and-forget-goroutines).

```go
// ✅ lifetime bound to ctx, spawner waits (WaitGroup.Go: Go 1.25+)
func (w *Worker) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for item := range w.items {
		wg.Go(func() { w.process(ctx, item) })
	}
	wg.Wait()
	return ctx.Err()
}
```

## errgroup

- **Use `golang.org/x/sync/errgroup` for a group of goroutines that each return an error.** Why: it adds error propagation and cancellation to `sync.WaitGroup`: with `WithContext`, "The derived Context is canceled the first time a function passed to Go returns a non-nil error", and `Wait` returns that first error. Source: [errgroup package doc](https://pkg.go.dev/golang.org/x/sync/errgroup).
- **Bound fan-out with `g.SetLimit(n)` instead of a hand-rolled semaphore.** Source: [errgroup — SetLimit](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.SetLimit).
- **Inside `g.Go`, use the group's derived `ctx`, not the parent.** Why: otherwise the other goroutines are not cancelled when one fails. Source: [errgroup — WithContext](https://pkg.go.dev/golang.org/x/sync/errgroup#WithContext).

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(4)
for _, url := range urls {
	g.Go(func() error { return fetch(ctx, url) })
}
if err := g.Wait(); err != nil {
	return fmt.Errorf("fetching: %w", err)
}
```

## Channels

- **Make channels unbuffered or of size one; any other size needs a justification of how it is bounded and what happens when it fills.** Why: a large buffer hides back-pressure until production load. Source: [Uber — Channel Size is One or None](https://github.com/uber-go/guide/blob/master/style.md#channel-size-is-one-or-none).
- **The sender closes a channel, never the receiver; never send after close.** Why: "Sending on a channel that has been closed causes a panic." Source: [Google decisions — Goroutine lifetimes](https://google.github.io/styleguide/go/decisions#goroutine-lifetimes).
- **Declare channel direction in parameters (`<-chan T`, `chan<- T`).** Source: [Google best practices — Channel direction](https://google.github.io/styleguide/go/best-practices#channel-direction).

## Mutexes

- **Use the zero value of `sync.Mutex`/`sync.RWMutex`; do not allocate a pointer to one.** Why: "The zero-value of sync.Mutex and sync.RWMutex is valid, so you almost never need a pointer to a mutex." Source: [Uber — Zero-value Mutexes are Valid](https://github.com/uber-go/guide/blob/master/style.md#zero-value-mutexes-are-valid).
- **Make the mutex a named unexported field (`mu sync.Mutex`), never embedded.** Why: embedding puts `Lock`/`Unlock` in the type's API. Source: same Uber section.
- **Never copy a struct holding a mutex; give it pointer receivers.** Why: the copy has its own lock state; `go vet`'s `copylocks` reports it. Source: [CRC — Copying](https://go.dev/wiki/CodeReviewComments#copying), [Google decisions — Copying](https://google.github.io/styleguide/go/decisions#copying), [Receiver type](https://google.github.io/styleguide/go/decisions#receiver-type).
- **Document whether a type is safe for concurrent use when it is not obvious.** Source: [Google best practices — Documentation, Concurrency](https://google.github.io/styleguide/go/best-practices#concurrency).

```go
// ❌
type Cache struct {
	sync.Mutex
	data map[string]string
}
// ✅
type Cache struct {
	mu   sync.Mutex
	data map[string]string
}
```

## Race detector

- **Run tests with `-race` in CI, and run a `-race` build under realistic load when chasing a bug.** Why: "The race detector only finds races that happen at runtime"; it costs 5-10x memory and 2-20x execution time, too much for production binaries. Source: [Data Race Detector](https://go.dev/doc/articles/race_detector).

## Where the sources disagree

- **Channel buffer size.** Uber: "one or none". Effective Go shows a buffered channel used "like a semaphore, for instance to limit throughput". We follow Uber's default (it is stricter and compatible), and use `errgroup.SetLimit` rather than a semaphore channel for bounding goroutines.
- **Context stored in a struct.** CRC allows it only for methods matching an interface you do not own; Google says the same but asks for a style-list discussion first. Same rule, we follow CRC.
- **Copying the loop variable before `go func()`.** Older code and Uber's "Parallel Tests" section copy it; since Go 1.22 each iteration has its own variable ([Go blog — Fixing For Loops in Go 1.22](https://go.dev/blog/loopvar-preview)). We follow the Go team: no copy.
