# Naming

This file covers how Go code names things: packages, exported identifiers, receivers, getters, interfaces, local variables (the error variable `err` included), error values and types, tests and constants. Each rule cites its source. When sources disagree, the Go team wins (Effective Go, Go Code Review Comments — "CRC", go.dev/doc/comment, the Go wiki and blog, the standard library's own code), then the Google Go Style Guide, then the Uber Go Style Guide, then recognized authors (Dave Cheney's Practical Go); the last section lists each disagreement and the side taken.

## Packages

- **Name a package with one short, lowercase word: no underscores, no mixedCaps.** Every caller types the name, so brevity pays at every call site. ([Effective Go — Package names](https://go.dev/doc/effective_go#package-names), [Go blog — Package names](https://go.dev/blog/package-names))
- **Name it for what it provides, not what it contains.** "This package lets you speak HTTP", not "this package holds the X type". ([Cheney — A good package starts with its name](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_a_good_package_starts_with_its_name))
- **Never `util`, `common`, `misc`, `helper`, `api`, `types`, `interfaces`, `base`, `shared`, `lib`.** A meaningless name tells the caller nothing, accumulates unrelated dependencies and collides with other imports. ([CRC — Package Names](https://go.dev/wiki/CodeReviewComments#package-names), [Go blog — Package names, "Avoid meaningless package names"](https://go.dev/blog/package-names), [Google — Util packages](https://google.github.io/styleguide/go/best-practices#util-packages), [Uber — Package Names](https://github.com/uber-go/guide/blob/master/style.md#package-names))
- **The package name is the base name of its directory.** `encoding/base64` is package `base64`. ([Effective Go — Package names](https://go.dev/doc/effective_go#package-names))
- **Avoid a package name that steals a good variable name.** `usercount` beats `count`, which callers want for a local. ([Google — Package names](https://google.github.io/styleguide/go/decisions#package-names), [Cheney — Don't name your variables for their types](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_dont_name_your_variables_for_their_types))
- **Rename an import only to resolve a collision.** A good package name never needs renaming. ([CRC — Imports](https://go.dev/wiki/CodeReviewComments#imports))

## Exported names: don't repeat the package

- **Let the package name qualify the identifier: `bufio.Reader`, not `bufio.BufReader`; `chubby.File`, not `chubby.ChubbyFile`.** Callers always see `pkg.Name`, so repeating `pkg` inside `Name` is noise. ([Effective Go — Package names](https://go.dev/doc/effective_go#package-names), [CRC — Package Names](https://go.dev/wiki/CodeReviewComments#package-names))
- **A package exporting one type named after itself calls its constructor `New`.** `ring.New`, `widget.New` — not `widget.NewWidget`. ([Effective Go — Package names](https://go.dev/doc/effective_go#package-names), [Google — Package vs. exported symbol name](https://google.github.io/styleguide/go/decisions#repetitive-with-package))
- **Do not repeat the receiver, parameter names or result types in a function name.** `(c *Config) WriteTo`, not `WriteConfigTo`; `yamlconfig.Parse`, not `ParseYAMLConfig`. ([Google — Function and method names, Avoid repetition](https://google.github.io/styleguide/go/best-practices#function-name-repetition))
- **Functions that return something get noun-like names; functions that do something get verb-like names.** The name reads as what the call site gets or does. ([Google — Naming conventions](https://google.github.io/styleguide/go/best-practices#function-name-conventions))

```go
// ❌
package widget
func NewWidget() *Widget
func (p *Project) ProjectName() string
// ✅
func New() *Widget
func (p *Project) Name() string
```

## MixedCaps and initialisms

- **Write multi-word names in `MixedCaps` or `mixedCaps`, never with underscores.** It is the Go convention even where other languages differ. ([Effective Go — MixedCaps](https://go.dev/doc/effective_go#mixed-caps), [CRC — Mixed Caps](https://go.dev/wiki/CodeReviewComments#mixed-caps), [Google — MixedCaps](https://google.github.io/styleguide/go/guide#mixed-caps))
- **Underscores are allowed only in `Test`/`Benchmark`/`Example` function names, packages only imported by generated code, and low-level OS/cgo interop.** These are the three exceptions Google lists. ([Google — Underscores](https://google.github.io/styleguide/go/decisions#underscores))
- **Keep an initialism in one case: `URL` or `url`, `ServeHTTP`, `appID`, `xmlHTTPRequest` — never `Url`, `Http`, `Id`.** Mixed-case initialisms read as different words. ([CRC — Initialisms](https://go.dev/wiki/CodeReviewComments#initialisms), [Google — Initialisms](https://google.github.io/styleguide/go/decisions#initialisms))
- **An initialism with a lowercase letter keeps its prose spelling unless exportedness forces the first letter: `gRPC` → `GRPC`, `iOS` → `IOS`, `DDoS` stays `DDoS`.** ([Google — Initialisms](https://google.github.io/styleguide/go/decisions#initialisms))

## Constants

- **Constants are MixedCaps like everything else: `MaxPacketSize`, `maxLength` — never `MAX_LENGTH` or `kMaxBufferSize`.** ALL_CAPS and `k` prefixes are other languages' conventions. ([CRC — Mixed Caps](https://go.dev/wiki/CodeReviewComments#mixed-caps), [Google — Constant names](https://google.github.io/styleguide/go/decisions#constant-names))
- **Name a constant for its role, not its value; a constant with no role beyond its value should not exist.** `const Twelve = 12` adds nothing. ([Google — Constant names](https://google.github.io/styleguide/go/decisions#constant-names))

## Receivers

- **Name the receiver with one or two letters abbreviating its type (`c` or `cl` for `Client`), never `this`, `self` or `me`.** A receiver is just another parameter; it appears on almost every line, and familiarity admits brevity. ([CRC — Receiver Names](https://go.dev/wiki/CodeReviewComments#receiver-names), [Google — Receiver names](https://google.github.io/styleguide/go/decisions#receiver-names))
- **Use the same receiver name in every method of a type.** `go doc` lists the methods together and variation is needless noise. ([CRC — Receiver Names](https://go.dev/wiki/CodeReviewComments#receiver-names), [Go Doc Comments — Funcs](https://go.dev/doc/comment#func))
- **Omit the receiver name when it is unused rather than writing `_`.** ([Google — Receiver names](https://google.github.io/styleguide/go/decisions#receiver-names))

## Getters, setters and well-known methods

- **A getter for field `owner` is `Owner()`, not `GetOwner()`; a setter is `SetOwner()`.** Export case already distinguishes field from method. ([Effective Go — Getters](https://go.dev/doc/effective_go#Getters), [Google — Getters](https://google.github.io/styleguide/go/decisions#getters))
- **Use `Compute` or `Fetch` instead of `Get` when the call is expensive or remote.** The verb warns the caller it may block or fail. ([Google — Getters](https://google.github.io/styleguide/go/decisions#getters))
- **Don't reuse `Read`, `Write`, `Close`, `Flush`, `String` unless your method has the canonical signature and meaning; when it does, use that name (`String`, not `ToString`).** ([Effective Go — Interface names](https://go.dev/doc/effective_go#interface-names))

## Interfaces

- **Name a one-method interface after its method plus `-er`: `Reader`, `Writer`, `Formatter`, `CloseNotifier`.** It names the agent that does the thing. ([Effective Go — Interface names](https://go.dev/doc/effective_go#interface-names))

## Local variables: length follows distance

- **The further from its declaration a name is used, the more descriptive it must be.** Prefer `c` to `lineCount` and `i` to `sliceIndex` in a short scope; package-level names need words. ([CRC — Variable Names](https://go.dev/wiki/CodeReviewComments#variable-names), [Google — Variable names](https://google.github.io/styleguide/go/decisions#variable-names), [Cheney — Identifier length](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_identifier_length))
- **Use Google's rough scale as a guide: 1–7 lines is small, 8–15 medium, 15–25 large, beyond a page very large.** These are judgement aids, not limits. ([Google — Variable names](https://google.github.io/styleguide/go/decisions#variable-names))
- **Keep the traditional single letters for their traditional jobs: `i`, `j`, `k` loop indices, `n` counts, `k`/`v` map keys/values, `r` an `io.Reader` or `*http.Request`, `w` an `io.Writer` or `http.ResponseWriter`, `ctx` a `context.Context`.** Readers assume these meanings. ([Google — Single-letter variable names](https://google.github.io/styleguide/go/decisions#v), [Cheney — Use a consistent naming style](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_use_a_consistent_naming_style))
- **Don't put the type in the name: `users`, not `userSlice` or `usersMap`; `count`, not `numUsers`.** The compiler already knows the type. Add a qualifier only when two forms coexist (`limitRaw`/`limit`, `ageString`/`age`). ([Google — Variable name vs. type](https://google.github.io/styleguide/go/decisions#repetitive-with-type), [Cheney — Don't name your variables for their types](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_dont_name_your_variables_for_their_types))
- **Drop words the context already supplies.** Inside `UserCount`, the local is `count`, not `userCount`. ([Google — External context vs. local names](https://google.github.io/styleguide/go/decisions#repetitive-in-context))
- **Give one concept one name across the codebase: always `db *sql.DB`, never a mix of `d`, `dbase`, `database`.** Predictable names are read without lookup. ([Cheney — Use a consistent naming style](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_use_a_consistent_naming_style))
- **Don't drop letters to save typing: `Sandbox`, not `Sbx`.** ([Google — Variable names](https://google.github.io/styleguide/go/decisions#variable-names))

## The error variable `err`

- **Name the error you are handling `err`.** Effective Go's `:=` rules exist "making it easy to use a single err value"; Google's "Good" rewrite replaces `dbLoadError` with `err`; the Go wiki's examples all use `err`. ([Effective Go — Redeclaration and reassignment](https://go.dev/doc/effective_go#redeclaration), [Google — External context vs. local names](https://google.github.io/styleguide/go/decisions#repetitive-in-context), [Go wiki — Errors](https://go.dev/wiki/Errors))
- **Reuse one `err` down a sequence of calls: `f, err := os.Open(…)` then `d, err := f.Stat()` reassigns, it does not redeclare.** `:=` reuses a same-scope variable when at least one new variable is declared. ([Effective Go — Redeclaration and reassignment](https://go.dev/doc/effective_go#redeclaration))
- **Give an error a distinct short name only when two errors are alive at once.** The standard library does this: `io.copyBuffer` reads `nr, er := src.Read(buf)` and writes `nw, ew := dst.Write(…)`, then decides which becomes the result `err`; `os.WriteFile` keeps the write's `err` and checks `if err1 := f.Close(); err1 != nil && err == nil`. Google's panic example names the recovered one `sErr`. ([io.go — copyBuffer](https://go.dev/src/io/io.go), [os/file.go — WriteFile](https://go.dev/src/os/file.go), [Google — When to panic](https://google.github.io/styleguide/go/best-practices#when-to-panic))
- **A typed error extracted from `err` gets a short name of its type: `pe` for `*ParseError`, `e` for `*QueryError`.** ([Go wiki — Errors, Naming](https://go.dev/wiki/Errors), [Go blog — Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors))

```go
// ✅ io.copyBuffer: two errors alive, two names, one result
nr, er := src.Read(buf)
if nr > 0 {
	nw, ew := dst.Write(buf[0:nr])
	if ew != nil {
		err = ew
		break
	}
	// ...
}
if er != nil {
	if er != EOF {
		err = er
	}
	break
}
```

### Shadowing `err`

- **Know the trap: `:=` inside a new block (an `if`, a loop, a closure) declares a new `err` that dies at the closing brace.** Code after the block reads the outer one. Google calls this *shadowing*, as opposed to *stomping* (reassignment in the same scope), which is fine. ([Google — Shadowing](https://google.github.io/styleguide/go/best-practices#shadowing), [Effective Go — Redeclaration and reassignment](https://go.dev/doc/effective_go#redeclaration))
- **To set an outer variable from an inner block, declare what is new with `var` and assign with `=`.** Google's fix is exactly this. ([Google — Shadowing](https://google.github.io/styleguide/go/best-practices#shadowing))
- **Scope `err` to its `if` (`if err := f(); err != nil`) when nothing else from the call is needed afterwards; declare it on its own line when the other results are used below.** Short scopes cannot leak a shadowed value; the long form avoids `else`. ([CRC — Indent Error Flow](https://go.dev/wiki/CodeReviewComments#indent-error-flow), [Google — Indent error flow](https://google.github.io/styleguide/go/decisions#indent-error-flow))
- **A named result `err` is in the function body's scope; a `:=` in a nested block shadows it, and a bare `return` there is a compile error ("result parameter err not in scope at return").** The spec allows compilers to reject it, and `gc` does. ([Go spec — Return statements](https://go.dev/ref/spec#Return_statements), [Effective Go — Redeclaration and reassignment](https://go.dev/doc/effective_go#redeclaration))
- **The `shadow` analyzer (`golang.org/x/tools/go/analysis/passes/shadow`) finds suspicious cases; it is not enabled by default because of false positives.** ([pkg.go.dev — shadow](https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/shadow))
- **Don't name a variable after an imported package (`url`, `errors`) outside a tiny scope.** It hides the package for the rest of the scope. ([Google — Shadowing](https://google.github.io/styleguide/go/best-practices#shadowing))

```go
// ❌ the outer ctx is used after the block
if shorten {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
}
// ✅
if shorten {
	var cancel func()
	ctx, cancel = context.WithTimeout(ctx, d)
	defer cancel()
}
```

## Error values and error types

- **Name an error variable `ErrX` (exported) or `errX` (unexported); name an error type `XError`.** `ErrBrokenLink`, `errNotFound`, `NotFoundError`, `resolveError`. ([Go wiki — Errors, Naming](https://go.dev/wiki/Errors), [Uber — Error Naming](https://github.com/uber-go/guide/blob/master/style.md#error-naming))
- **A helper that panics on failure, for package initialization, is named `MustX` (`regexp.MustCompile`, `template.Must`).** The prefix warns that it stops the program. ([Google — Must functions](https://google.github.io/styleguide/go/decisions#must-functions))

## Tests

- **Test functions are `TestXxx(*testing.T)` where `Xxx` does not start with a lowercase letter; benchmarks `BenchmarkXxx`; examples `ExampleF`, `ExampleT_M`, `ExampleF_suffix`.** The `go test` tool finds them by these names. ([testing package](https://pkg.go.dev/testing))
- **Underscores may group related cases (`TestParse_EmptyInput`).** It is one of Google's listed exceptions and Uber's explicit allowance. ([Google — Underscores](https://google.github.io/styleguide/go/decisions#underscores), [Uber — Function Names](https://github.com/uber-go/guide/blob/master/style.md#function-names))
- **Name subtests like identifiers, short and typeable on the `-run` command line; avoid spaces and slashes.** The runner rewrites spaces and `/` splits the filter. ([Google — Subtest names](https://google.github.io/styleguide/go/decisions#subtest-names))

## Where the sources disagree

- **Prefixing unexported globals with `_`.** Uber prefixes unexported package-level `var`/`const` with `_` (`_defaultPort`). Effective Go and Google forbid underscores in names outside the listed exceptions. **We follow the Go team and Google: no `_` prefix**, by precedence. ([Uber — Prefix Unexported Globals with _](https://github.com/uber-go/guide/blob/master/style.md#prefix-unexported-globals-with-_), [Google — Underscores](https://google.github.io/styleguide/go/decisions#underscores))
- **Plural package names.** Uber says "Not plural (`net/url`, not `net/urls`)"; Cheney suggests plurals for utility packages, like `strings`. The standard library has both (`strings`, `bytes`, `errors` vs `net/url`); the Go team states no rule. **We follow Uber's singular default** (the highest source that states a rule) and accept a plural when it mirrors a stdlib precedent for helpers over a builtin type. ([Uber — Package Names](https://github.com/uber-go/guide/blob/master/style.md#package-names), [Cheney — Avoid package names like base, common, or util](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_avoid_package_names_like_base_common_or_util))
- **How short is short.** CRC says "Prefer `c` to `lineCount`"; Cheney titles a section "Choose identifiers for clarity, not brevity" and keeps `sum`/`count` over `s`/`c` in a seven-line function. Both state the same distance rule (Cheney quotes Gerrand). **We follow CRC's distance rule**: a single letter only when declaration and use sit a few lines apart. ([CRC — Variable Names](https://go.dev/wiki/CodeReviewComments#variable-names), [Cheney — Identifier length](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_identifier_length))
- **The word "stutter".** CRC and common usage call `chubby.ChubbyFile` stutter; Google's source carries an editor's note not to use that term and says "repetition" instead. Same rule, different word; we use "repetition" in prose.
