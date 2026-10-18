---
name: go-style
description: How to write Go the way the Go community does — naming, errors, comments and doc comments, readability (paragraphs, function and file shape, parameters), packages and project layout, interfaces, tests, tooling and linters, concurrency — every rule taken from recognized sources (the Go team's Effective Go, Code Review Comments and go.dev docs, the standard library, the Google and Uber Go style guides, Dave Cheney's Practical Go) with its citation. Use this skill whenever you write, review, refactor or explain Go code, in any project — a new function, a test, a package layout, an error message, a doc comment, a linter configuration — and whenever someone asks what is idiomatic in Go, what "the convention" is, or how Go compares with what they know from TypeScript or other languages, even if they don't say "style". For wtm's next/ architecture (where code goes), also use go-architecture.
---

# go-style — Go as the community writes it

Every rule here comes from a source the Go community recognizes, cited in the reference files. When sources disagree, follow them in this order — and each reference's last section says which side we took and why:

1. **The Go team**: [Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) (CRC), [Go Doc Comments](https://go.dev/doc/comment), [module layout](https://go.dev/doc/modules/layout), the Go blog, and the standard library's own code.
2. **The [Google Go Style Guide](https://google.github.io/styleguide/go/)** (guide, decisions, best practices).
3. **The [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).**
4. **Recognized authors**: Dave Cheney ([Practical Go](https://dave.cheney.net/practical-go/presentations/qcon-china.html)), Rob Pike ([Go Proverbs](https://go-proverbs.github.io/), "Errors are values"), Ben Johnson, Mitchell Hashimoto.

A project's established convention wins over a general rule when the two conflict (consistency inside a codebase is itself a rule of every guide); say so when you follow the project against the community, and propose aligning it when the project rule has no reason left.

A rule a linter already checks is marked with it — `[gofmt]`, `[vet]`, `[staticcheck STxxxx]`, `[revive rule]` — as configured in this repository ("This repository" below); an unmarked rule is caught only in review.

## The rules that matter most

Naming — `references/naming.md`
- A name's length follows the distance between its declaration and its uses: short for a local in a small scope, descriptive for a package-level name.
- Call the error `err` and reuse it; give distinct short names only when two errors are alive at once (`er`/`ew` in `io.copyBuffer`). Watch `:=` shadowing inside a block. A sentinel is `ErrX`, an error type `XError` `[staticcheck ST1012]` `[revive error-naming]`.
- Don't repeat the package in what it exports (`bufio.Reader`, `widget.New`) `[revive exported]`; no `util`/`common`/`misc` packages; initialisms keep one case (`URL`, `ID`) and no ALL_CAPS constants `[staticcheck ST1003]` `[revive var-naming]`.
- Receivers are one or two letters, the same in every method, never `this`/`self` `[staticcheck ST1006, ST1016]` `[revive receiver-naming]`; getters are `Owner()`, not `GetOwner()`.

Errors — `references/errors.md`
- Check every error and handle it once: return it or log it, never both.
- Handle the error first and return; the happy path stays at the left margin, never in an `else` `[revive indent-error-flow, early-return, superfluous-else]`.
- Add context naming the operation (`"open config: %w"`); error strings are lowercase with no final punctuation `[staticcheck ST1005]` `[revive error-strings]`.
- Wrap with `%w` only when callers should inspect the cause — wrapping makes it part of your API; inspect with `errors.Is`/`errors.As`, never by string.
- Errors are opaque by default; export an `ErrX` sentinel or an `XError` type only when callers must branch on it. Panic only for programmer errors.

Comments — `references/comments.md`
- Every exported name has a doc comment: a full sentence that starts with the name (`// Run carries out …`), "reports whether" for booleans, the contract not the algorithm `[revive exported]` `[staticcheck ST1020–ST1022]`; every package has a package comment `[revive package-comments]` `[staticcheck ST1000]`.
- A comment inside a body says why (or how, for a step the code can't make obvious); a block that needs a comment because it does something separate becomes a function named after it.
- Section separators (`// ` + 76 dashes, then a title line, then a blank line) only in a long file whose declarations fall into named groups; otherwise split the file.

Readability — `references/readability.md`
- Use blank lines to break a function into paragraphs, the way paragraphs break a document (Cheney).
- Return early, reduce nesting, keep variables in the smallest scope.
- Positional parameters are the norm; reach for an options struct when a function takes many inputs, when two values of the same type could be swapped (Cheney's alternative there is a helper type, `Source.CopyTo(dest)`, not a struct), or when the list will grow. A struct for two inputs of different types is not a Go convention.
- Prefer a nil slice to `[]T{}`, useful zero values, field names in composite literals of other packages' types; no mutable globals, no `init()` doing work.
- No hard line-length limit; a file is focused enough that a maintainer can tell which file holds something — neither thousands of lines nor many tiny files.

Packages — `references/packages.md`
- A package provides one concept and is named after it; merge packages that are always used together; put supporting code in `internal/`; `cmd/` holds mains; there is no `pkg/` convention.
- Layering prevents import cycles; dependencies are passed through constructors and wired by hand in `main` — no DI framework, no package-level state.

Interfaces — `references/interfaces.md`
- The consumer declares the interface, with only the methods it calls; don't define one before it is used or to mock a producer.
- Keep interfaces small; accept interfaces, return concrete types (with Google's listed exceptions).
- Pointer receivers when in doubt, and don't mix pointer and value receivers on one type.

Tests — `references/testing.md`
- Failures read `Func(in) = got, want want`; compare with `cmp.Diff` labelled `(-want +got)`; errors with `errors.Is`/`errors.As`.
- `t.Error` to keep going, `t.Fatal` for setup or inside a subtest, never from a goroutine the test started `[vet testinggoroutine]`; helpers call `t.Helper()` and register teardown with `t.Cleanup`; use `t.TempDir`, `t.Setenv`.
- Table-driven tests with `t.Run`; a subtest name is a short identifier, never a sentence — a longer description goes in a field of the row (Google, "Subtest names"); fixtures in `testdata/`; golden files behind an `-update` flag.
- Prefer real implementations, then fakes; the Go team and Google advise against assertion libraries — a project already on testify keeps it consistently (`require` where the test cannot go on, `assert` otherwise).

Tooling — `references/tooling.md`
- `gofmt`/`goimports` are not negotiable `[gofmt]`; `go vet`, staticcheck with `checks: ["all"]` (its defaults leave the naming and doc-comment checks off), `go test -race` and `go mod tidy` in CI.
- In general, golangci-lint with a deliberate set: the standard set + revive `exported` (doc comments), errorlint, gocritic (diagnostic), unparam, misspell, nolintlint, thelper, usetesting. This repository runs the linters directly instead ("This repository" below).

Concurrency — `references/concurrency.md`
- `context.Context` is the first parameter `[revive context-as-argument]`, never stored in a struct; always call `cancel` `[vet lostcancel]`.
- Every goroutine has a known exit and someone waiting for it (`sync.WaitGroup`, `errgroup`); prefer synchronous functions and let the caller add concurrency.
- Mutexes use their zero value, are named fields, are never copied `[vet copylocks]`; channels are unbuffered or size one unless a reason is written down.

## This repository

What wtm runs instead of the general setup of `tooling.md`, and the conventions it adds; `next/` follows this skill, `internal/` follows `CLAUDE.md` where the two differ.

- **No golangci-lint.** The Makefile runs each tool directly: `gofmt -l` (fails, never rewrites), `go vet`, `staticcheck` (`next/staticcheck.conf`: `checks = ["all"]`), `revive` on `next/` (`next/revive.toml`: `exported`, `package-comments`, `var-naming`, `receiver-naming`, `error-strings`, `error-naming`, `context-as-argument`, `indent-error-flow`, `early-return`, `superfluous-else`, and the other rules listed there), `deadcode -test` (exceptions in `.deadcode-ignore`), `go-sumtype` (every type switch over an interface declared `//go-sumtype:decl` handles each variant), and `tools/archlint` (the import table of `next/README.md` and the repository's own rules, among them `subtestname` and `testlayout` for the two test rules below). errcheck, errorlint, unparam and thelper are not run: those rules are caught in review.
- **Gates.** `make lint` and `make test` (`-race`) for the whole repository; `make lint-next` and `make test-next` for a change that leaves `internal/`, the root package and `go.mod` alone. The pre-commit hook runs the lint gate, `build-validator` adds the tests.
- **testify**, already standard here: `require` where the test cannot go on, `assert` otherwise, `want` first. Each test file holds only `Test…` functions; the fixtures and fakes go in `fixture_test.go`, the helpers shared by a package's tests in `helpers_test.go`, what an external test reaches in `export_test.go`.
- **Fakes** of an adapter live in `adapter/<name>test` and keep its contract; the go-architecture skill (`references/ports-adapters.md`) has the details.
- **Sealed interfaces** (an unexported method, one type per variant) carry `//go-sumtype:decl <Name>` on a line of their own, outside the doc comment, so gofmt keeps it.

## Read next

Open the reference whose subject you are touching before writing or reviewing it; each one is short, lists its rules with their why and their source, and ends with "Where the sources disagree".

| Reference | Read it when |
| -- | -- |
| `references/naming.md` | naming anything: package, type, function, receiver, variable, error, constant, test |
| `references/errors.md` | creating, returning, wrapping, inspecting or printing an error; deciding to panic |
| `references/comments.md` | writing a doc comment, a package comment, a comment in a body, a section separator, a TODO, a directive |
| `references/readability.md` | shaping a function or a file: paragraphs, nesting, parameters, named results, zero values, globals |
| `references/packages.md` | creating or splitting a package, laying out a module, wiring dependencies, structuring an application |
| `references/interfaces.md` | declaring an interface or a port, choosing receivers, embedding, writing a test double |
| `references/testing.md` | writing or reviewing a test, a helper, a fixture, a golden file |
| `references/tooling.md` | setting up or running formatters, vet, staticcheck, golangci-lint, gopls, govulncheck, deadcode |
| `references/concurrency.md` | contexts, goroutines, channels, mutexes, errgroup |
