# Testing

How to write Go tests the way the community does: table-driven tests and subtests, failure messages, comparisons, assertion libraries, test doubles, test data, parallelism, examples, fuzzing and the race detector. When sources disagree, the Go team wins (Go Code Review Comments, the Go FAQ and wiki, `go help` / `go doc`, the standard library's own tests), then the Google Go Style Guide, then the Uber Go Style Guide, then recognized authors (Dave Cheney, Mitchell Hashimoto). Every rule names its source; the last section lists where they disagree and which side we follow.

## Failure messages

- **Fail with a message that says what was called, with what input, what it got and what was wanted.** Why: "Assume that the person debugging your failing test is not you, and is not your team." Source: [Go Code Review Comments — Useful Test Failures](https://go.dev/wiki/CodeReviewComments#useful-test-failures).
- **Print got before want, and use the words "got" and "want".** Why: the conventional shape `YourFunc(%v) = %v, want %v` is read the same everywhere; "Some test frameworks encourage writing these backwards … Go does not." Source: [Google decisions — Got before want](https://google.github.io/styleguide/go/decisions#got-before-want), [CRC — Useful Test Failures](https://go.dev/wiki/CodeReviewComments#useful-test-failures).
- **Name the function and the input in the message, not only the subtest.** Why: subtest names are escaped by the runner, and the message must stand on its own. Source: [Google decisions — Identify the function](https://google.github.io/styleguide/go/decisions#identify-the-function), [Identify the input](https://google.github.io/styleguide/go/decisions#identify-the-input).
- **Use `%q` for strings and `%+v` for small structs.** Why: quotes reveal empty and whitespace values; field names make structs readable. Source: [Google decisions — Print diffs / Level of detail](https://google.github.io/styleguide/go/decisions#print-diffs).

```go
// ❌
if got != tt.want {
	t.Errorf("expected %d, got %d", tt.want, got)
}
// ✅
if got := Split(tt.in); got != tt.want {
	t.Errorf("Split(%q) = %d, want %d", tt.in, got, tt.want)
}
```

## t.Error vs t.Fatal

- **Prefer `t.Error` and keep going; use `t.Fatal` only when what follows would be meaningless.** Why: "a developer who is fixing the failing test doesn't have to re-run the test after fixing each bug to find the next bug." Source: [Google decisions — Keep going](https://google.github.io/styleguide/go/decisions#keep-going), [Go FAQ — Where is my favorite helper function for testing?](https://go.dev/doc/faq#testing_framework).
- **Use `t.Fatal` for setup failures; inside a `t.Run` subtest, `t.Fatal` ends only that row.** Why: without subtests, a failing row is reported with `t.Error` followed by `continue`; with subtests `t.Fatal` moves on to the next subtest. Source: [Google best practices — t.Error vs. t.Fatal](https://google.github.io/styleguide/go/best-practices#terror-vs-tfatal).
- **Never call `t.Fatal`/`t.FailNow` (or testify `require`) from a goroutine the test started; use `t.Error` and return.** Why: `FailNow` "must be called from the goroutine running the test or benchmark function, not from other goroutines created during the test" (`go doc testing.T.FailNow`); `go vet`'s `testinggoroutine` reports it. Source: [Google best practices — Don't call t.Fatal from separate goroutines](https://google.github.io/styleguide/go/best-practices#dont-call-tfatal-from-separate-goroutines).

## Comparing values

- **Compare whole structures against a constructed `want`, not field by field.** Why: one deep comparison states the expectation once and fails with all differences. Source: [Google decisions — Full structure comparisons](https://google.github.io/styleguide/go/decisions#full-structure-comparisons).
- **Use `cmp.Diff` / `cmp.Equal` (`github.com/google/go-cmp/cmp`) rather than `reflect.DeepEqual`, and label the diff direction.** Why: `cmp.Diff` "is preferred, particularly for new tests and new code", and since order conventions vary, print a key such as `(-want +got)` matching the argument order. Source: [Google decisions — Print diffs](https://google.github.io/styleguide/go/decisions#print-diffs), [Equality comparison and diffs](https://google.github.io/styleguide/go/decisions#equality-comparison-and-diffs).
- **Keep `cmp` in tests.** Why: "The cmp package is designed for testing, rather than production use" and may panic on comparisons it considers ill-defined. Source: [Google decisions — Equality comparison and diffs](https://google.github.io/styleguide/go/decisions#equality-comparison-and-diffs).
- **Compare semantics, not unstable output you do not own** (parse JSON and compare values instead of comparing bytes). Why: "json.Marshal can change (and has changed in the past) the specific bytes that it emits." Source: [Google decisions — Compare stable results](https://google.github.io/styleguide/go/decisions#compare-stable-results).
- **Test errors by identity (`errors.Is`, `errors.As`, `cmpopts.EquateErrors`) or simply by `err != nil`, never by their string.** Why: string comparison turns the test into a change detector. Source: [Google decisions — Test error semantics](https://google.github.io/styleguide/go/decisions#test-error-semantics).

```go
// ✅
if diff := cmp.Diff(want, got); diff != "" {
	t.Errorf("Parse(%q) mismatch (-want +got):\n%s", in, diff)
}
```

## Assertion libraries (testify and friends)

What the sources say, quoted:

- Go team: "Go's standard testing package makes it easy to write unit tests, but it lacks features provided in other language's testing frameworks such as assertion functions. … testing frameworks tend to develop into mini-languages of their own, with conditionals and controls and printing mechanisms, but Go already has all those capabilities; why recreate them? We'd rather write tests in Go; it's one fewer language to learn and the approach keeps the tests straightforward and easy to understand." Source: [Go FAQ — Where is my favorite helper function for testing?](https://go.dev/doc/faq#testing_framework).
- Google: "Do not create “assertion libraries” as helpers for testing." and "Assertion libraries tend to either stop the test early (if assert calls t.Fatalf or panic) or omit relevant information about what the test got right" … "Instead of creating a domain-specific language for testing, use Go itself." Source: [Google decisions — Assertion libraries](https://google.github.io/styleguide/go/decisions#assertion-libraries).
- Google: "The Go standard library provides the testing package. This is the only testing framework permitted for Go code in the Google codebase. In particular, assertion libraries and third-party testing frameworks are not allowed." Source: [Google decisions — Use package testing](https://google.github.io/styleguide/go/decisions#use-package-testing).
- Google: "Assertion helpers are not considered idiomatic in Go." Source: [Google best practices — Leave testing to the Test function](https://google.github.io/styleguide/go/best-practices#leave-testing-to-the-test-function).
- Uber: its own examples use testify (`require.NoError(t, err)`, `assert.Equal(t, "192.0.2.0", host)`) without discussing the choice. Source: [Uber — Test Tables](https://github.com/uber-go/guide/blob/master/style.md#test-tables).

Rules:

- **In a project with no established choice, use `testing` + `cmp`.** Why: it is the position of the Go team and Google, and needs no dependency. Sources above.
- **In a project that already uses testify, keep using it consistently; do not mix styles in one package.** Why: Google names fragmentation ("which assertion library should I use") as the main cost; consistency within a codebase removes most of it. Source: [Google decisions — Assertion libraries](https://google.github.io/styleguide/go/decisions#assertion-libraries).
- **With testify, use `assert` for checks (it reports and continues, like `t.Error`) and `require` only where the test cannot go on (it stops, like `t.Fatal`).** Why: `require` functions "terminate current test" and "must be called from the goroutine running the test". Source: [testify README — require package](https://github.com/stretchr/testify), [Google decisions — Keep going](https://google.github.io/styleguide/go/decisions#keep-going).
- **With testify, still pass a message naming the call and input when the values alone do not tell the story, and pass `want` first (`assert.Equal(t, want, got)`).** Why: testify's message is generic; the useful-failure rules still apply. Source: [CRC — Useful Test Failures](https://go.dev/wiki/CodeReviewComments#useful-test-failures).
- The trade-off: testify gives terse checks and readable diffs out of the box, is widespread outside Google and linted by `testifylint`; it costs a dependency, a second vocabulary, and messages that say less about the call than a hand-written `Func(in) = got, want want`.

## Table-driven tests and subtests

- **Use a table when many cases share the same logic; run each row with `t.Run`.** Why: "If you ever find yourself using copy and paste when writing a test, think about whether refactoring into a table-driven test … might be a better option." Source: [Go wiki — TableDrivenTests](https://go.dev/wiki/TableDrivenTests), [Google decisions — Table-driven tests](https://google.github.io/styleguide/go/decisions#table-driven-tests).
- **Keep branching out of the loop body; split into several tests when rows need different logic.** Why: "Table tests should NOT be used whenever there needs to be complex or conditional logic inside subtests" (flags such as `shouldCallFoo`, `setupMocks func(*FooMock)` in rows). Source: [Uber — Avoid Unnecessary Complexity in Table Tests](https://github.com/uber-go/guide/blob/master/style.md#avoid-unnecessary-complexity-in-table-tests), [Google decisions — Table-driven tests](https://google.github.io/styleguide/go/decisions#table-driven-tests) (a simple `wantErr` is tolerated; separate tables for success and error outputs are "the best approach").
- **Name subtests like identifiers: short, typeable, no spaces or slashes.** Why: the runner replaces spaces with underscores and `/` is the `-run` separator. Source: [Google decisions — Subtest names](https://google.github.io/styleguide/go/decisions#subtest-names).
- **Subtests must not depend on each other.** Why: each must run alone with `go test -run`. Source: [Google decisions — Subtests](https://google.github.io/styleguide/go/decisions#subtests).
- **Use field names in table rows.** Why: rows stay readable and survive new fields. Source: [Google best practices — Use field names in struct literals](https://google.github.io/styleguide/go/best-practices#use-field-names-in-struct-literals).

```go
func TestSplit(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "", want: nil},
		{name: "two", in: "a,b", want: []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Split(tt.in)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Split(%q) mismatch (-want +got):\n%s", tt.in, diff)
			}
		})
	}
}
```

## Test helpers and cleanup

- **A helper that takes `*testing.T` calls `t.Helper()` first.** Why: failures are then reported at the caller's line. Source: [Google decisions — Test helpers](https://google.github.io/styleguide/go/decisions#test-helpers), `go doc testing.T.Helper`.
- **Helpers do setup and fail with `t.Fatal`; they do not return errors and do not assert.** Why: a helper failure is a failure of the environment, and returning it clutters every caller. Source: [Google best practices — Error handling in test helpers](https://google.github.io/styleguide/go/best-practices#error-handling-in-test-helpers), [Leave testing to the Test function](https://google.github.io/styleguide/go/best-practices#leave-testing-to-the-test-function).
- **Register teardown with `t.Cleanup` inside the helper instead of returning a cleanup func.** Why: cleanups run "when the test (or subtest) and all its subtests complete … in last added, first called order", and the caller cannot forget them. Source: `go doc testing.T.Cleanup`, [Google best practices — Error handling in test helpers](https://google.github.io/styleguide/go/best-practices#error-handling-in-test-helpers) (tip on `t.Cleanup`).
- **Use `t.TempDir()`, `t.Setenv()`, `t.Chdir()` and `t.Context()` rather than hand-rolled versions.** Why: each restores state or cancels automatically; `TempDir` "is automatically removed when the test and all its subtests complete". Source: `go doc testing.T.TempDir`, `go doc testing.T.Setenv`, `go doc testing.T.Chdir`, `go doc testing.T.Context`, [Google decisions — Contexts](https://google.github.io/styleguide/go/decisions#contexts) ("prefer using (testing.TB).Context() over context.Background()").
- **A `context.Context` parameter comes before `t`.** Why: the context-first convention "also applies to test helpers". Source: [Google decisions — Contexts](https://google.github.io/styleguide/go/decisions#contexts).

```go
// ❌ caller must remember to defer
func tempConfig(t *testing.T) (string, func()) { … }
// ✅
func tempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(sample), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	return path
}
```

## Test doubles

- **Prefer the real implementation; then a fake; reach for a mock last.** Why: Google states "our codebase's preference for real objects" and asks to test "via the public API of the real implementation". Source: [Google best practices — Litmus tests](https://google.github.io/styleguide/go/best-practices#litmus-tests), [Google decisions — Interfaces](https://google.github.io/styleguide/go/decisions#interfaces).
- **For HTTP or RPC, use the real client against a test server (`httptest.Server`), not a hand-written client double.** Why: "By using the production client with a test-specific server, you ensure your test is using as much of the real code as possible." Source: [Google best practices — Use real transports](https://google.github.io/styleguide/go/best-practices#use-real-transports); Hashimoto: "Testing networking? Make a real network connection. Don't mock `net.Conn`" ([Advanced Testing with Go](https://speakerdeck.com/mitchellh/advanced-testing-with-go)).
- **Define the interface in the consumer package; do not add interfaces on the producer side "for mocking".** Why: the producer should return concrete types; the consumer declares the small interface it needs and fakes it in its tests. Source: [CRC — Interfaces](https://go.dev/wiki/CodeReviewComments#interfaces), [Google best practices — Avoid unnecessary interfaces](https://google.github.io/styleguide/go/best-practices#avoid-unnecessary-interfaces).
- **Put shared doubles in a separate `<pkg>test` package (`creditcardtest.Stub`), named by behaviour (`AlwaysCharges`, `AlwaysDeclines`).** Why: the base package and its types already say what is doubled, and production code does not export back doors. Source: [Google best practices — Test double and helper packages](https://google.github.io/styleguide/go/best-practices#test-double-and-helper-packages).

## Test data and golden files

- **Put fixtures in `testdata/`.** Why: "The go tool will ignore a directory named "testdata", making it available to hold ancillary data needed by the tests." Source: `go help test`.
- **Compare large outputs to golden files and regenerate them with an `-update` flag; review the diff before committing.** Why: the standard library does it (`var update = flag.Bool("update", false, "update golden files")` in `go/printer/printer_test.go`); "Human eyeball the generated golden data. If it is correct, commit it." Source: Go source `src/go/printer/printer_test.go`, [Hashimoto — Advanced Testing with Go, Golden files](https://speakerdeck.com/mitchellh/advanced-testing-with-go).

```go
var update = flag.Bool("update", false, "update golden files")

func TestRender(t *testing.T) {
	got := Render(input)
	golden := filepath.Join("testdata", t.Name()+".golden")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(string(want), string(got)); diff != "" {
		t.Errorf("Render() mismatch (-want +got):\n%s", diff)
	}
}
```

## Parallel tests

- **Call `t.Parallel()` only in tests that share no mutable state.** Why: a parallel test runs "in parallel with (and only with) other parallel tests". Source: `go doc testing.T.Parallel`, [Go wiki — TableDrivenTests, Parallel Testing](https://go.dev/wiki/TableDrivenTests#parallel-testing).
- **Never combine `t.Parallel` with `t.Setenv` or `t.Chdir`.** Why: they affect the whole process, so they "cannot be used in parallel tests or tests with parallel ancestors" (the test panics with "can not use t.Parallel"). Source: `go doc testing.T.Setenv`, `go doc testing.T.Chdir`.
- **Teardown after parallel subtests goes in `t.Cleanup`, or after a grouping `t.Run`, not in a plain `defer`.** Why: parallel subtests are paused until the parent function returns, so a `defer` in the parent runs before them; a grouping `t.Run` "will not return until its parallel subtests complete". Source: [Go blog — Using Subtests and Sub-benchmarks, Control of Parallelism](https://go.dev/blog/subtests).
- **Do not copy the loop variable (`tt := tt`) on Go ≥ 1.22.** Why: loop variables have per-iteration scope in modules declaring `go 1.22` or later. Source: [Go blog — Fixing For Loops in Go 1.22](https://go.dev/blog/loopvar-preview), [Go wiki — TableDrivenTests](https://go.dev/wiki/TableDrivenTests) ("uncomment for Go < 1.22").

## Test packages: internal vs `_test`

- **Both are idiomatic: `package foo` (internal) to test internals, `package foo_test` (black box) to test through the exported API, to break import cycles, or for examples.** Why: `go help test` compiles `_test` packages separately; Google allows both and names `_test` "black box tests". Source: [Google decisions — Test package](https://google.github.io/styleguide/go/decisions#test-package), `go help test`.
- **When an external test needs one internal, expose it in an `export_test.go` file inside `package foo`.** Why: the standard library does this (`src/strings/export_test.go`, `src/bytes/export_test.go`); the symbol exists only in test builds.
- **Put `Example` functions in the `_test` package.** Why: "when viewed in godoc, the examples have the appropriate package prefix and can be easily copy pasted." Source: [Dave Cheney — Practical Go, 5.1.2](https://dave.cheney.net/practical-go/presentations/qcon-china.html).

## Examples, fuzzing, race detector

- **Give a new package a runnable `Example`, ending with `// Output:`.** Why: it is documentation that `go test` verifies; "An example with no such comment is compiled but not executed." Source: [CRC — Examples](https://go.dev/wiki/CodeReviewComments#examples), `go help testfunc`.
- **Name examples `ExampleFoo`, `ExampleT_M`, `ExampleFoo_suffix`.** Why: godoc attaches them to the symbol by name. Source: `go help testfunc`.
- **Fuzz parsers and decoders with `func FuzzXxx(f *testing.F)`, seeded with `f.Add`; keep targets fast, deterministic and free of global state.** Why: the target runs "in parallel across multiple workers and in nondeterministic order"; failing inputs land in `testdata/fuzz/FuzzXxx/` and replay as regular tests, so commit them. Source: [Go Fuzzing](https://go.dev/doc/security/fuzz/).
- **Run `go test -race ./...` in CI.** Why: "The race detector only finds races that happen at runtime", so it needs the tests to exercise concurrent paths; it costs 5-10x memory and 2-20x time, too much for production builds. Source: [Data Race Detector](https://go.dev/doc/articles/race_detector).

## Where the sources disagree

- **Assertion libraries.** Go FAQ and Google reject them; Uber's examples use testify. We follow the Go team for a project starting fresh (`testing` + `cmp`), and keep testify where a project already standardised on it, because inconsistency is the cost Google names first.
- **Copying the loop variable in parallel tests.** Uber's "Parallel Tests" still asks for it; the Go 1.22 change made it unnecessary. We follow the Go team: no copy on `go ≥ 1.22` (golangci-lint's `copyloopvar` flags leftovers).
- **Cleanup helpers.** Hashimoto (2017) returns a `func()` for the caller to defer; Go 1.14 added `t.Cleanup` and Google recommends it. We follow Google: `t.Cleanup` inside the helper.
- **Fixture directory.** Hashimoto uses `test-fixtures/`; the go tool ignores only `testdata/`. We follow `go help test`: `testdata/`.
- **"expected" vs "want".** Hashimoto's slides print `expected`; CRC and Google use got/want in that order. We follow CRC.
- **Internal vs external tests.** Cheney prefers internal tests for unit tests; Hashimoto tests "only the exported API" unless a function is very complex; Google accepts both. We follow Google and the standard library: internal by default when testing internals, `_test` for the public contract and examples.
- **Exported test doubles.** Hashimoto exports mocks and harnesses from a `testing.go` in the package; Google and CRC forbid test back doors in the production API. We follow Google: doubles live in a separate `<pkg>test` package or in the consumer's test files.
- **`t.Fatal` inside table tests.** Google decisions ("Keep going") prefer `t.Error`, but for rows in `t.Run` subtests Google best practices recommend `t.Fatal`. Not a real conflict: `t.Fatal` inside a subtest stops only that row.
