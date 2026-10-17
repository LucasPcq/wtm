# Readability: inside a file and a function

This file covers how a Go function and a Go file read: paragraphs, early returns, nesting, scope, declaration order, file size, line length, parameter lists, named results, zero values, slices, literals, declarations, globals, panics and `init`. Every rule cites its source. When sources disagree, the Go team (Effective Go, Go Code Review Comments, the Go blog, the standard library) wins over the Google Go Style Guide, which wins over the Uber Go Style Guide, which wins over individual authors (Dave Cheney, Rob Pike's proverbs, Mat Ryer). The last section lists each disagreement and the side we follow.

## Paragraphs and function length

- Separate the steps of a function with blank lines, as paragraphs separate the steps of an argument. Why: a reader sees the precondition, the work and the result as three blocks. Source: [Cheney, Practical Go §2.2 (tip on blank lines)](https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- Do not count lines to decide a function is too long; move its boundaries when it reads as too long or too repetitive. Why: "There's no rule 'never have a function more than N lines long', but there is definitely such a thing as too long of a function, and of too repetitive tiny functions." Source: [CRC, Line Length](https://go.dev/wiki/CodeReviewComments#line-length).
- Rather than commenting a block of code, extract it into a function whose name says what the block does. Why: a name is checked by the compiler and survives refactoring; a comment drifts. Source: [Cheney, Practical Go §3.2.2](https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- Prefer the simplest construct that works: a core language construct, then the standard library, then a dependency. Why: complexity is easy to add and hard to remove. Source: [Google guide, Least mechanism](https://google.github.io/styleguide/go/guide#least-mechanism).

```go
// ✅ three paragraphs: guard, accumulate, compute (Cheney's AverageAge)
func AverageAge(people []Person) int {
	if len(people) == 0 {
		return 0
	}

	var count, sum int
	for _, p := range people {
		sum += p.Age
		count += 1
	}

	return sum / count
}
```

## Early return and nesting

- Handle the error or the special case first and return; the normal path continues unindented down the page. Why: "This improves the readability of the code by permitting visually scanning the normal path quickly." Source: [CRC, Indent Error Flow](https://go.dev/wiki/CodeReviewComments#indent-error-flow), [Google decisions, Indent error flow](https://google.github.io/styleguide/go/decisions#indent-error-flow).
- Omit the `else` after a branch that ends in `return`, `break`, `continue` or `goto`. Why: the Go libraries do so, and the else only indents the normal path. Source: [Effective Go, If](https://go.dev/doc/effective_go#if).
- In a loop, `continue` past the invalid item instead of nesting the valid case. Why: it reduces the amount of code nested multiple levels. Source: [Uber, Reduce Nesting](https://github.com/uber-go/guide/blob/master/style.md#reduce-nesting).
- Use guard clauses that assert preconditions on entry ("line of sight", Mat Ryer's term). Why: the successful exit is not found by matching closing braces. Source: [Cheney, Practical Go §4.3](https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- When a variable is set in both branches of an `if`, set the default first and override it in a single `if`. Source: [Uber, Unnecessary Else](https://github.com/uber-go/guide/blob/master/style.md#unnecessary-else).

```go
// ❌ normal code looks abnormal
if err != nil {
	return err
} else {
	use(x)
}

// ✅
if err != nil {
	return err
}
use(x)
```

## Scope of variables

- Reduce the scope of variables and constants, for instance `if err := f(); err != nil`, unless that conflicts with reducing nesting. Why: a name that lives fewer lines is less to track. Source: [Uber, Reduce Scope of Variables](https://github.com/uber-go/guide/blob/master/style.md#reduce-scope-of-variables).
- Do not use the if-with-initializer form when the variable is used for more than a few lines after it; declare it on its own line. Why: otherwise the rest of the function ends up in an `else`. Source: [Google decisions, Indent error flow](https://google.github.io/styleguide/go/decisions#indent-error-flow), [CRC, Indent Error Flow](https://go.dev/wiki/CodeReviewComments#indent-error-flow).
- The further from its declaration a name is used, the more descriptive it must be. Source: [CRC, Variable Names](https://go.dev/wiki/CodeReviewComments#variable-names).

## Order of declarations in a file

- Put types, constants and variables first, then the constructor `NewT`/`newT` right after its type, then the methods grouped by receiver, then plain helper functions at the end; sort functions in rough call order, exported ones first. Why: a reader meets a type before its methods and the entry points before their helpers. Source: [Uber, Function Grouping and Ordering](https://github.com/uber-go/guide/blob/master/style.md#function-grouping-and-ordering).
- Neither Effective Go, CRC nor the Google guide set an order of declarations; Uber's is the only explicit rule among the sources.

## File size and splitting a package into files

- Split a large package into several files, each grouping related code (`client.go`, `server.go`, `cookie.go` in `net/http`); there is no "one type, one file" convention. Why: "files should be focused enough that a maintainer can tell which file contains something, and the files should be small enough that it will be easy to find once there." Source: [Google best practices, Package size](https://google.github.io/styleguide/go/best-practices#package-size).
- Avoid both a single file of many thousands of lines and many tiny files. Source: [Google best practices, Package size](https://google.github.io/styleguide/go/best-practices#package-size).
- Start a package with one file named after the package; split by responsibility as it grows; if two files have similar imports, consider merging them; prefer nouns for file names. Source: [Cheney, Practical Go §5.1.1](https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- A long package doc may live alone in `doc.go`; this is optional. Source: [Google best practices, Package size](https://google.github.io/styleguide/go/best-practices#package-size).

## Line length

- There is no line length limit; break a line for the semantics of what you write, not for its length, and when a line is too long, shorten names or factor out a local variable instead of wrapping. Source: [Effective Go, Formatting](https://go.dev/doc/effective_go#formatting) ("Go has no line length limit"), [CRC, Line Length](https://go.dev/wiki/CodeReviewComments#line-length), [Google guide, Line length](https://google.github.io/styleguide/go/guide#line-length).
- Do not split a line before an indentation change (a function signature, an `if` condition), nor a long string such as a URL. Why: wrapped lines then read as part of the body. Source: [Google guide, Line length](https://google.github.io/styleguide/go/guide#line-length), [Google decisions, Function formatting](https://google.github.io/styleguide/go/decisions#function-formatting).

## Function parameters

- Keep signatures short: as parameters are added, "adjacent parameters of the same type become easier to confuse"; consider splitting a highly configurable function into simpler ones. Source: [Google best practices, Function argument lists](https://google.github.io/styleguide/go/best-practices#function-argument-lists).
- Introduce an option struct "where a function requires many inputs"; choose it when all callers set one or more options, when many callers set many options, or when the options are shared by several functions. The struct is the last argument, `context.Context` is never in it, and it is exported only if its function is. Why: the literal names every value (self-documenting, hard to swap), omits defaults, and grows without touching call sites. Source: [Google best practices, Option structure](https://google.github.io/styleguide/go/best-practices#option-structure).
- Use variadic (functional) options when most callers set none, options are many and rare, take arguments, can fail, or third parties may define them; they cost "a substantial amount of additional code". Source: [Google best practices, Variadic options](https://google.github.io/styleguide/go/best-practices#variadic-options), [Uber, Functional Options](https://github.com/uber-go/guide/blob/master/style.md#functional-options) ("especially if you already have three or more arguments").
- These recommendations "primarily apply to exported APIs" and "may be unnecessary for your use case"; balance clarity against least mechanism. Source: [Google best practices, Function argument lists](https://google.github.io/styleguide/go/best-practices#function-argument-lists).
- Two parameters of the same type whose order matters are a hazard (`CopyFile(to, from string)`); Cheney's fix is a helper type (`Source.CopyTo(dest)`), not a struct; symmetric pairs (`Max(a, b int)`) are fine. Source: [Cheney, Practical Go §6.1.1](https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- Replace a naked `bool` parameter with a named type when its meaning is unclear at the call site. Source: [Uber, Avoid Naked Parameters](https://github.com/uber-go/guide/blob/master/style.md#avoid-naked-parameters).
- Do not annotate arguments with inline comments; use an option struct or better function docs. Source: [Google decisions, Function formatting](https://google.github.io/styleguide/go/decisions#function-formatting).
- Design for the default use case: do not force callers to pass values they do not care about, and do not mix nil-able and non-nil-able parameters. Source: [Cheney, Practical Go §6.2](https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- Pass values, not pointers, to save a few bytes; a pointer to a string or to an interface is almost never right. Source: [CRC, Pass Values](https://go.dev/wiki/CodeReviewComments#pass-values), [Uber, Pointers to Interfaces](https://github.com/uber-go/guide/blob/master/style.md#pointers-to-interfaces).

What the sources say about a struct for 2 or 3 parameters: none recommends it as a rule. Google ties the option struct to "many inputs" and to call-site shape; its single-field example `server.New(ctx, server.Options{Port: 42})` shows a struct is acceptable when a positional value would be unreadable. Uber's threshold for functional options is "three or more arguments" for APIs "you foresee needing to expand". Mat Ryer prefers long positional dependency lists in constructors because "I can make a struct skipping any fields that I don't like, but a function forces my hand" ([How I write HTTP services in Go after 13 years](https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/)). A struct for two inputs is therefore a house rule when adopted, not community idiom; it is justified when the two are swappable values of the same type or the list is expected to grow.

```go
// ❌ nine positional arguments, two bools side by side
func EnableReplication(ctx context.Context, cfg *Config, primary, readonly []string, existing, overwrite bool, every time.Duration, workers int, w Watcher)

// ✅ option struct: named at the call site, defaults omitted
func EnableReplication(ctx context.Context, opts ReplicationOptions)
storage.EnableReplication(ctx, storage.ReplicationOptions{Config: cfg, PrimaryRegions: regions})
```

## Named results and naked returns

- Name results only when two or more have the same type, when the meaning is unclear (`(lat, long float64, err error)`), when the caller must act on one (`cancel func()`), or when a deferred closure must change it. Why: names that repeat the type are noise in godoc. Source: [CRC, Named Result Parameters](https://go.dev/wiki/CodeReviewComments#named-result-parameters), [Google decisions, Named result parameters](https://google.github.io/styleguide/go/decisions#named-result-parameters).
- Do not name results to avoid declaring a variable, nor to enable naked returns; naked returns are fine only in a function of a handful of lines. Source: [CRC, Named Result Parameters](https://go.dev/wiki/CodeReviewComments#named-result-parameters).

## Zero values, slices and literals

- Make the zero value useful: a type whose zero value is ready to use (`bytes.Buffer`, `sync.Mutex`) needs no constructor. Source: [Effective Go, Allocation with new](https://go.dev/doc/effective_go#allocation_new), [Go Proverbs](https://go-proverbs.github.io/), [Cheney, Practical Go §4.4](https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- Declare an empty slice as `var t []string`, not `t := []string{}`; return `nil`, not `[]T{}`; test emptiness with `len(s) == 0`. Exception: a non-nil empty slice when JSON must encode `[]` instead of `null`. Source: [CRC, Declaring Empty Slices](https://go.dev/wiki/CodeReviewComments#declaring-empty-slices), [Uber, nil is a valid slice](https://github.com/uber-go/guide/blob/master/style.md#nil-is-a-valid-slice), [Google decisions, Nil slices](https://google.github.io/styleguide/go/decisions#nil-slices).
- Never make an API distinguish a nil slice from an empty one. Source: [Google decisions, Nil slices](https://google.github.io/styleguide/go/decisions#nil-slices).
- Name the fields of a struct literal for any type from another package; for a package-local type it is optional but common, and expected when the struct has many fields. Why: field order is not part of a struct's API. Source: [Google decisions, Field names](https://google.github.io/styleguide/go/decisions#field-names), [Uber, Use Field Names to Initialize Structs](https://github.com/uber-go/guide/blob/master/style.md#use-field-names-to-initialize-structs).
- Omit zero-value fields from literals unless they add meaning. Source: [Uber, Omit Zero Value Fields in Structs](https://github.com/uber-go/guide/blob/master/style.md#omit-zero-value-fields-in-structs).

## var and :=

- Use `:=` when initializing to a non-zero value; use `var` to declare a zero value "ready for later use". Source: [Google best practices, Initialization](https://google.github.io/styleguide/go/best-practices#initialization) and [Declaring variables with zero values](https://google.github.io/styleguide/go/best-practices#declaring-variables-with-zero-values), [Uber, Local Variable Declarations](https://github.com/uber-go/guide/blob/master/style.md#local-variable-declarations), [Cheney, Practical Go §2.5](https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- A reassigned variable is idiomatic Go (accumulators, `err` reuse, the default-then-override pattern Uber recommends); none of the sources asks to avoid reassignment. Source: [Effective Go, Redeclaration and reassignment](https://go.dev/doc/effective_go#redeclaration), [Uber, Unnecessary Else](https://github.com/uber-go/guide/blob/master/style.md#unnecessary-else).

## Globals, panics and init

- Avoid mutable package-level variables, function pointers included; inject the dependency as a struct field or constructor argument. Why: globals make tests order-dependent and forbid two configurations in one process. Source: [Uber, Avoid Mutable Globals](https://github.com/uber-go/guide/blob/master/style.md#avoid-mutable-globals), [Google best practices, Global state](https://google.github.io/styleguide/go/best-practices#global-state), [Cheney, Practical Go §4.5](https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- Do not panic for normal error handling; return an error. Panic is acceptable for API misuse that tests must catch, for `MustXxx` helpers called at program start, and inside a package that always recovers its own panic. Source: [CRC, Don't Panic](https://go.dev/wiki/CodeReviewComments#dont-panic), [Effective Go, Panic](https://go.dev/doc/effective_go#panic), [Google decisions, Don't panic](https://google.github.io/styleguide/go/decisions#dont-panic) and [Must functions](https://google.github.io/styleguide/go/decisions#must-functions), [Google best practices, When to panic](https://google.github.io/styleguide/go/best-practices#when-to-panic), [Uber, Don't Panic](https://github.com/uber-go/guide/blob/master/style.md#dont-panic), [Go Proverbs](https://go-proverbs.github.io/).
- Do not recover panics to avoid crashes. Source: [Google best practices, Program checks and panics](https://google.github.io/styleguide/go/best-practices#program-checks-and-panics).
- Exit only from `main` (`os.Exit`/`log.Fatal`), ideally once, with `main` calling a `run` that returns an error. Source: [Uber, Exit in Main](https://github.com/uber-go/guide/blob/master/style.md#exit-in-main), [Ryer, How I write HTTP services](https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/).
- Avoid `init()`; when used, keep it deterministic, independent of other `init` order, free of environment access and I/O. Prefer `var x = defaultX()`. Source: [Uber, Avoid init()](https://github.com/uber-go/guide/blob/master/style.md#avoid-init).

```go
// ❌ mutable global swapped in tests
var _timeNow = time.Now

// ✅ dependency held by the value that uses it
type signer struct{ now func() time.Time }
func newSigner() *signer { return &signer{now: time.Now} }
```

## Where the sources disagree

- **Line length.** Go team and Google: no limit, break by meaning. Uber: soft limit of 99 characters. We follow the Go team (higher precedence); gofmt never wraps.
- **Annotating naked arguments.** Uber suggests `printInfo("foo", true /* isLocal */)`. Google says avoid inline argument comments and use an option struct or docs. We follow Google, and Uber's own "better yet": a named type instead of `bool`.
- **When to switch to options.** Uber: functional options for expandable APIs "especially if you already have three or more arguments". Google: option struct for "many inputs" chosen by call-site shape, variadic options only when most callers pass none. Ryer: positional dependencies even when long, for type safety. We follow Google: decide by how the call reads, prefer the option struct over variadic options, and do not adopt a parameter-count threshold from the sources.
- **Functional options implementation.** Uber prefers an `Option` interface with an unexported `apply` method (comparable in tests); Google shows closures. Either is accepted; Uber's form when options must be compared or printed.
- **Field names in literals.** Uber: "almost always". Google: required for other packages' types, optional for local ones. We follow Google and name fields whenever it helps.
- **`init()`.** Effective Go presents `init` neutrally (for initializations that cannot be declarations, and to verify program state). Uber says avoid it where possible. They do not contradict; we apply Uber's restrictions because nothing in Effective Go encourages I/O or environment access in `init`.
- **Panic at startup.** Uber allows panic at program initialization; Google prefers `log.Exit` with an actionable message in `main` and `Must` helpers only at startup. We return errors to `main` and reserve `Must` for package-level values such as `regexp.MustCompile`.
- **Order of declarations.** Only Uber states one; nothing contradicts it, so we follow it.
