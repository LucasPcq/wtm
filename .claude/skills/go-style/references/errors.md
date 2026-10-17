# Errors

This file covers how Go code produces, propagates, inspects and reports errors: checking every error, handling it once, the shape of the error path, wrapping with `%w` or `%v`, `errors.Is`/`errors.As`/`errors.Join`, sentinel vs typed vs opaque errors, error strings, panics, and programming with errors as values. Each rule cites its source. When sources disagree, the Go team wins (Effective Go, Go Code Review Comments — "CRC", the Go blog and wiki, the standard library), then the Google Go Style Guide, then the Uber Go Style Guide, then recognized authors (Dave Cheney, Rob Pike's blog post is the Go team's); the last section lists each disagreement and the side taken. Naming of `err`, `ErrX` and `XError` is in `naming.md`.

## Check every error

- **Never discard an error with `_`; handle it, return it, or — in truly exceptional situations — panic.** An unchecked error lets the function continue on unspecified results. ([CRC — Handle Errors](https://go.dev/wiki/CodeReviewComments#handle-errors), [Google — Handle errors](https://google.github.io/styleguide/go/decisions#handle-errors))
- **When ignoring an error is genuinely safe, say why in a comment on that line.** `n, _ := b.Write(p) // never returns a non-nil error` for `*bytes.Buffer`. ([Google — Handle errors](https://google.github.io/styleguide/go/decisions#handle-errors))
- **Return `error` as the last result, and return the `error` interface, never a concrete error type.** A nil `*os.PathError` stored in an `error` interface is non-nil. ([Google — Returning errors](https://google.github.io/styleguide/go/decisions#returning-errors))
- **When a function returns a non-nil error, treat its other results as unspecified unless documented otherwise.** Callers must not use them. ([Google — Returning errors](https://google.github.io/styleguide/go/decisions#returning-errors), [Cheney — Only handle an error once](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_only_handle_an_error_once))
- **Report failure with an extra result (`error`, or `ok bool` when no explanation is needed), not an in-band value like `-1`, `""` or `nil`.** The compiler then stops `Parse(Lookup(key))`. ([CRC — In-Band Errors](https://go.dev/wiki/CodeReviewComments#in-band-errors), [Google — In-band errors](https://google.github.io/styleguide/go/decisions#in-band-errors))

## Handle each error once

- **Make exactly one decision per error: log it, return it, recover from it, or degrade — not two.** Logging and then returning makes every caller up the stack log it again, and an author who logs often forgets to return. ([Cheney — Only handle an error once](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_only_handle_an_error_once), [Uber — Handle Errors Once](https://github.com/uber-go/guide/blob/master/style.md#handle-errors-once), [Google — Logging errors](https://google.github.io/styleguide/go/best-practices#error-logging))
- **Log (and don't return) only when the operation is optional and the program degrades gracefully.** Failing to emit metrics should not break the request. ([Uber — Handle Errors Once](https://github.com/uber-go/guide/blob/master/style.md#handle-errors-once))

```go
// ❌ logged here, returned, logged again by every caller
if err != nil {
	log.Printf("could not get user %q: %v", id, err)
	return err
}
// ✅ one decision: annotate and return
if err != nil {
	return fmt.Errorf("get user %q: %w", id, err)
}
```

## Return early; keep the happy path left

- **Handle the error first and return; never put the normal path in an `else`.** The successful flow then runs down the page at minimal indentation. ([CRC — Indent Error Flow](https://go.dev/wiki/CodeReviewComments#indent-error-flow), [Effective Go — If](https://go.dev/doc/effective_go#if), [Google — Indent error flow](https://google.github.io/styleguide/go/decisions#indent-error-flow))
- **Use guard clauses: check preconditions as the function goes and return early.** Mat Ryer's "line of sight": the success path is never buried in braces. ([Cheney — Return early rather than nesting deeply](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_return_early_rather_than_nesting_deeply), [Uber — Reduce Nesting](https://github.com/uber-go/guide/blob/master/style.md#reduce-nesting))
- **When a value from the call is used for more than a few lines, declare it on its own line instead of in the `if` initializer.** Otherwise the use ends up in an `else`. ([CRC — Indent Error Flow](https://go.dev/wiki/CodeReviewComments#indent-error-flow), [Google — Indent error flow](https://google.github.io/styleguide/go/decisions#indent-error-flow))
- **Flag an unusual `err == nil` check with a comment.** It looks like the common `err != nil` at a glance. ([Google — Signal boosting](https://google.github.io/styleguide/go/best-practices#signal-boost))

```go
// ❌
if x, err := f(); err != nil {
	return err
} else {
	use(x)
}
// ✅
x, err := f()
if err != nil {
	return err
}
use(x)
```

## Add context when you return

- **Return the error as-is when you have nothing to add; otherwise add what you know that the callee doesn't.** "call service foo: connection refused" beats "connection refused". ([Uber — Error Wrapping](https://github.com/uber-go/guide/blob/master/style.md#error-wrapping), [Google — Adding information to errors](https://google.github.io/styleguide/go/best-practices#error-extra-info))
- **Don't repeat what the wrapped error already says.** `os.Open` errors already carry the path; `could not open settings.txt: open settings.txt: …` says it twice. ([Google — Adding information to errors](https://google.github.io/styleguide/go/best-practices#error-extra-info))
- **Name the operation, not the failure: `"new store: %w"`, not `"failed to create new store: %w"`; never `"failed: %v"`.** The error's presence already says it failed, and "failed to" piles up as the error climbs. ([Uber — Error Wrapping](https://github.com/uber-go/guide/blob/master/style.md#error-wrapping), [Google — Adding information to errors](https://google.github.io/styleguide/go/best-practices#error-extra-info), the Go team's own shape `"decompress %v: %w"` in [Go blog — Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors))
- **Put `%w` at the end, `"…: %w"`, so the text reads newest to oldest like the chain.** ([Google — Placement of %w in errors](https://google.github.io/styleguide/go/best-practices#error-percent-w))
- **Where useful, prefix an error string with its origin (operation or package), like `image: unknown format`.** It is then intelligible far from the call. ([Effective Go — Errors](https://go.dev/doc/effective_go#errors))

## `%w` or `%v`: wrap to expose, not by reflex

- **Wrap with `%w` only when you want callers to inspect the underlying error; wrapping makes it part of your API.** Once a caller writes `errors.Is(err, sql.ErrNoRows)`, you must keep returning it even after switching databases. ([Go blog — Working with Errors in Go 1.13, Whether to Wrap](https://go.dev/blog/go1.13-errors#whether-to-wrap))
- **Use `%v` (or a fresh error) to hide an implementation detail, especially at a system boundary (RPC, storage, a package's public API).** The text is identical; only programs lose access. ([Go blog — Errors and package APIs](https://go.dev/blog/go1.13-errors), [Google — Adding information to errors](https://google.github.io/styleguide/go/best-practices#error-extra-info))
- **Wrap errors that came from something the caller supplied** (an `io.Reader` they passed you): they may legitimately inspect it. ([Go blog — Whether to Wrap](https://go.dev/blog/go1.13-errors#whether-to-wrap))
- **When you do expose a wrapped sentinel or type, document it in the function's doc comment and test it.** It is a contract. ([Uber — Error Wrapping](https://github.com/uber-go/guide/blob/master/style.md#error-wrapping), [Google — Documentation conventions, Errors](https://google.github.io/styleguide/go/best-practices#documentation-conventions-errors))
- **If your contract says "returns an error wrapping `ErrX`", return `fmt.Errorf("…: %w", ErrX)`, never `ErrX` bare.** Callers otherwise come to depend on `==` and you can't add context later. ([Go blog — Errors and package APIs](https://go.dev/blog/go1.13-errors))

## Inspecting errors

- **Test with `errors.Is(err, ErrX)` and `errors.As(err, &target)`, not `==` or a type assertion.** They walk the whole wrap chain. ([Go blog — Examining errors with Is and As](https://go.dev/blog/go1.13-errors), [pkg.go.dev — errors](https://pkg.go.dev/errors))
- **Pass `errors.As` a pointer to a variable of the error's exact type (`var e *QueryError; errors.As(err, &e)`).** The pointer-to-pointer is correct when the error type is a pointer. ([Go blog — Examining errors with Is and As](https://go.dev/blog/go1.13-errors))
- **Never branch on an error's string (`strings.Contains(err.Error(), …)`, regexps).** Give the error structure instead. ([Google — Error structure](https://google.github.io/styleguide/go/best-practices#error-structure))
- **Combine several independent errors with `errors.Join`; it drops nils, returns nil if all are nil, and the result is inspectable with `Is`/`As`.** Its text joins the messages with newlines. ([pkg.go.dev — errors.Join](https://pkg.go.dev/errors#Join))

## Sentinel, typed or opaque

- **Default to opaque: return the error and let callers only check it against nil and print it.** It couples the caller least and leaves you free to add context. ([Cheney — Don't just check errors, handle them gracefully, Opaque errors](https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully), [Go blog — Errors and package APIs](https://go.dev/blog/go1.13-errors))
- **When callers must branch on a condition with a static message, export a sentinel `var ErrX = errors.New("…")`.** ([Uber — Error Types](https://github.com/uber-go/guide/blob/master/style.md#error-types), [Google — Error structure](https://google.github.io/styleguide/go/best-practices#error-structure))
- **When callers must branch and also need data (a path, a line), export an error type `XError` and match it with `errors.As`.** `os.PathError` carries `Op`, `Path`, `Err`. ([Uber — Error Types](https://github.com/uber-go/guide/blob/master/style.md#error-types), [Effective Go — Errors](https://go.dev/doc/effective_go#errors), [Google — Error structure](https://google.github.io/styleguide/go/best-practices#error-structure))
- **When nobody needs to match, `errors.New` (static) or `fmt.Errorf` (dynamic) is enough.** Uber's table: no match + static → `errors.New`; no match + dynamic → `fmt.Errorf`; match + static → top-level `var`; match + dynamic → custom type. ([Uber — Error Types](https://github.com/uber-go/guide/blob/master/style.md#error-types))
- **Remember that every exported sentinel and error type is public API.** ([Uber — Error Types](https://github.com/uber-go/guide/blob/master/style.md#error-types), [Go blog — Whether to Wrap](https://go.dev/blog/go1.13-errors#whether-to-wrap))
- **Document which sentinels and types a function returns, and whether the type is a pointer.** `errors.As` with the wrong pointerness never matches. ([Google — Documentation conventions, Errors](https://google.github.io/styleguide/go/best-practices#documentation-conventions-errors))

## Error strings

- **Start lowercase (unless a proper noun, acronym or exported name) and end without punctuation.** Error strings are embedded in other messages: `log.Printf("reading %s: %v", name, err)`. ([CRC — Error Strings](https://go.dev/wiki/CodeReviewComments#error-strings), [Google — Error strings](https://google.github.io/styleguide/go/decisions#error-strings))
- **The full message shown to a human (log line, UI, test failure) may be capitalized.** The rule is for the composable fragment. ([CRC — Error Strings](https://go.dev/wiki/CodeReviewComments#error-strings), [Google — Error strings](https://google.github.io/styleguide/go/decisions#error-strings))

```go
// ❌
errors.New("Could not open file.")
// ✅
errors.New("open config: no such file")
```

## Panic is for programmer errors

- **Don't use `panic` for normal error handling; return an `error`.** ([CRC — Don't Panic](https://go.dev/wiki/CodeReviewComments#dont-panic), [Effective Go — Panic](https://go.dev/doc/effective_go#panic), [Google — Don't panic](https://google.github.io/styleguide/go/decisions#dont-panic), [Uber — Don't Panic](https://github.com/uber-go/guide/blob/master/style.md#dont-panic))
- **Panic on API misuse and impossible states — bugs that tests and review should catch.** The standard library panics like an out-of-range index does. ([Google — When to panic](https://google.github.io/styleguide/go/best-practices#when-to-panic), [Effective Go — Panic](https://go.dev/doc/effective_go#panic))
- **Panicking during initialization is acceptable when the program cannot set itself up (`MustX` helpers, `init`).** ([Effective Go — Panic](https://go.dev/doc/effective_go#panic), [Google — Must functions](https://google.github.io/styleguide/go/decisions#must-functions), [Uber — Don't Panic](https://github.com/uber-go/guide/blob/master/style.md#dont-panic))
- **In `main`, turn errors into an exit code with a human message, not a stack trace.** Propagate configuration errors up to `main`. ([Google — Program initialization](https://google.github.io/styleguide/go/best-practices#program-init), [Uber — Don't Panic](https://github.com/uber-go/guide/blob/master/style.md#dont-panic))
- **A panic used internally (a parser unwinding) must never cross the package boundary: recover it at the public entry point, re-panic anything that isn't yours.** ([Google — When to panic](https://google.github.io/styleguide/go/best-practices#when-to-panic), [Effective Go — Recover](https://go.dev/doc/effective_go#recover))
- **In tests, fail with `t.Fatal`, not `panic`.** The test is then marked failed. ([Uber — Don't Panic](https://github.com/uber-go/guide/blob/master/style.md#dont-panic))

## Errors are values: program with them

- **When the same check repeats over a sequence of operations, consider a type that records the first error and turns later calls into no-ops (the "sticky error"), checked once at the end.** `bufio.Scanner` (`Scan` then `Err`) and `bufio.Writer` (`Write` then `Flush`) do this. ([Go blog — Errors are values](https://go.dev/blog/errors-are-values), [Cheney — Eliminate error handling by eliminating errors](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_eliminate_error_handling_by_eliminating_errors))
- **Use it only when an all-or-nothing check is enough.** Pike's stated drawback: you can't know how much completed before the error. ([Go blog — Errors are values](https://go.dev/blog/errors-are-values))
- **Don't use it where each step's failure needs its own handling or context, or where the deferred check hides which step failed.** Pike: "this approach is not suitable for all situations"; the point is to remove clutter, not to avoid checking. ([Go blog — Errors are values](https://go.dev/blog/errors-are-values))
- **Prefer an API that has no error to handle (e.g. `bufio.Scanner` over a manual `ReadString` loop with an `io.EOF` special case).** ([Cheney — Eliminate error handling by eliminating errors](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_eliminate_error_handling_by_eliminating_errors))

```go
type errWriter struct {
	w   io.Writer
	err error
}

func (ew *errWriter) write(buf []byte) {
	if ew.err != nil {
		return
	}
	_, ew.err = ew.w.Write(buf)
}

ew := &errWriter{w: fd}
ew.write(p0[a:b])
ew.write(p1[c:d])
if ew.err != nil {
	return ew.err
}
```

## Where the sources disagree

- **Is `%w` the default?** Uber: "`%w` … is a good default for most wrapped errors". The Go blog: "Wrap an error to expose it to callers. Do not wrap an error when doing so would expose implementation details", wrapping makes the error part of your API. Google sides with the blog (`%v` at system boundaries, `%w` when documented). **We follow the Go team: `%w` only to expose deliberately, `%v` otherwise.** ([Uber — Error Wrapping](https://github.com/uber-go/guide/blob/master/style.md#error-wrapping), [Go blog — Whether to Wrap](https://go.dev/blog/go1.13-errors#whether-to-wrap))
- **Where `%w` goes when wrapping a sentinel.** Google puts `%w` at the end except for a sentinel that categorizes the failure, which goes first (`"%w: invalid header: %v"`). The Go blog's only example with a sentinel puts it last (`"%q: %w"`). **We follow the Go team's example: `%w` last.** ([Google — Sentinel error placement](https://google.github.io/styleguide/go/best-practices#error-percent-w-sentinel-placement), [Go blog — Errors and package APIs](https://go.dev/blog/go1.13-errors))
- **Sentinels and error types at all.** Cheney's blog: "avoid sentinel errors", "avoid error types" in public APIs, assert behaviour instead. The Go blog, Google and Uber all present exported sentinels and types as the way to let callers branch. **We follow the Go team: opaque by default, a sentinel or type when callers must branch.** ([Cheney — Don't just check errors](https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully), [Go blog — Errors and package APIs](https://go.dev/blog/go1.13-errors))
- **"failed to" / "could not" prefixes.** Uber says avoid "failed to"; Cheney's examples use "could not write config"; some of Google's examples use "failed to load user count" or "couldn't find…". The Go team's examples name the operation (`"decompress %v: %w"`, `open /etc/passwx: …`). **We follow the Go team and Uber: name the operation.**
- **Recovering panics.** Effective Go shows `recover` in a server goroutine (`safelyDo`) so one failing request doesn't kill the others; Google calls `net/http`'s recovery "a historical mistake" and advises not to recover unexpected panics. Effective Go presents it as possible, not as a rule. **By precedence recovery at a goroutine boundary is allowed, but we only recover panics we raised ourselves, at the package boundary** — the one pattern both accept. ([Effective Go — Recover](https://go.dev/doc/effective_go#recover), [Google — Program checks and panics](https://google.github.io/styleguide/go/best-practices#checks-and-panics))
- **Invariant failures: `panic` or `log.Fatal`.** Google prefers `log.Fatal` for broken invariants because deferred functions may deadlock while panicking; Effective Go and Google's own "When to panic" use `panic` for impossible states. **We follow Effective Go: `panic` for programmer errors**, `log.Fatal`/exit only in `main`.
