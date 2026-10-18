# Tooling

Which tools keep Go code conventional and what each one enforces: formatting (`gofmt`, `goimports`), `go vet`, staticcheck, golangci-lint and a linter set aligned with community conventions, gopls, govulncheck, deadcode, the race detector, `go mod tidy` and `go generate`. When sources disagree, the Go team wins (`go help` / `go doc`, go.dev docs and blog, Go Code Review Comments), then the Google Go Style Guide, then the Uber Go Style Guide, then the tools' own maintainers (staticcheck, golangci-lint, revive). Every rule names its source; the last section lists where they disagree.

## Formatting is not negotiable

- **Format every file with `gofmt` (or `goimports`, its superset); never hand-format.** Why: "Almost all Go code in the wild uses gofmt"; Google: "All Go source files must conform to the format outputted by the gofmt tool." Source: [CRC — Gofmt](https://go.dev/wiki/CodeReviewComments#gofmt), [Google guide — Formatting](https://google.github.io/styleguide/go/guide#formatting).
- **If gofmt's layout looks wrong, change the code, not the layout.** Why: "if the answer doesn't seem right, rearrange your program (or file a bug about gofmt), don't work around it." Source: [Effective Go — Formatting](https://go.dev/doc/effective_go#formatting).
- **Prefer `goimports` in editors and CI.** Why: it "additionally adds (and removes) import lines as necessary"; Uber lists it in its minimum linter set. Source: [CRC — Gofmt](https://go.dev/wiki/CodeReviewComments#gofmt), [Uber — Linting](https://github.com/uber-go/guide/blob/master/style.md#linting).
- **Format generated code too.** Why: it is read in code search and diffs ("Generated code should generally also be formatted (e.g., by using format.Source)"). Source: [Google guide — Formatting](https://google.github.io/styleguide/go/guide#formatting).
- **Stricter formatters (`gofumpt`) are a project choice, not a convention.** Why: none of the Go team, Google or Uber requires them; golangci-lint describes gofumpt only as "a stricter format than gofmt". Source: [golangci-lint — Formatters](https://golangci-lint.run/docs/formatters/).

## go vet

- **Run `go vet ./...` in CI; treat its reports as bugs to fix.** Why: vet "reports suspicious constructs, such as Printf calls whose arguments do not align with the format string", errors the compiler does not catch. Source: `go doc cmd/vet`.
- **Know what vet already guards so you do not re-invent it:** `copylocks` (locks passed by value), `lostcancel` (cancel func not called), `printf`, `loopclosure`, `testinggoroutine` (`t.Fatal` from a goroutine), `tests` (malformed test/example names), `unusedresult`, `waitgroup`, `errorsas`. Source: `go doc cmd/vet` (`go tool vet help` lists them).
- **`go test` already runs a high-confidence subset** ("atomic, bool, buildtags, directive, errorsas, ifaceassert, nilfunc, printf, stringintconv, and tests"); a full `go vet` is still needed for the rest. Source: `go help test`.

## staticcheck

- **Run staticcheck in addition to vet.** Why: "Just run staticcheck ./... on your code in addition to go vet ./..."; its checks are "designed to be fast, precise and useful" with "few to no false positives". Source: [Staticcheck — Welcome](https://staticcheck.dev/docs/).
- **What its families cover** ([Checks](https://staticcheck.dev/docs/checks/)):
  - `SA` (staticcheck): bugs — SA1 misuses of the standard library, SA2 concurrency issues, SA3 testing issues, SA4 code that isn't really doing anything, SA5 correctness, SA6 performance, SA9 dubious constructs.
  - `S` (simple): code simplifications (`S1000`…).
  - `ST` (stylecheck): style "primarily derived from the Go wiki and represent community consensus"; "Some checks are very pedantic and disabled by default."
  - `QF` (quickfix): refactorings offered as editor quick fixes (De Morgan, tagged switch…).
  - `U1000` (unused): unused code, exposed in golangci-lint as the `unused` linter ("Checks Go code for unused constants, variables, functions and types"). Source: [golangci-lint — Linters](https://golangci-lint.run/docs/linters/).
- **ST checks on naming and doc comments:** `ST1003` poorly chosen identifier (mixed caps, initialisms), `ST1005` error string format, `ST1006` receiver name, `ST1011` `time.Duration` variable names, `ST1012` error variable names (`ErrFoo`/`errFoo`), `ST1016` consistent receiver names, `ST1000` package comment, `ST1020`/`ST1021`/`ST1022` the doc of an exported function / type / var-const "should start with" its name. Source: [Staticcheck — Checks, ST1](https://staticcheck.dev/docs/checks/#ST1).
- **Turn on the pedantic ST checks if you want the community conventions enforced.** Why: the default configuration is `checks = ["all", "-SA9003", "-ST1000", "-ST1003", "-ST1016", "-ST1020", "-ST1021", "-ST1022", "-ST1023"]`, so naming and doc-comment checks are off out of the box. Source: [Staticcheck — Configuration](https://staticcheck.dev/docs/configuration/).
- **Note that ST1020–22 check the shape of existing doc comments; they do not require one.** Requiring a doc comment on every exported name is revive's `exported` rule (below).

## golangci-lint: one runner, a deliberate set

This section is the general recommendation. wtm does not use golangci-lint: its Makefile runs gofmt, vet, staticcheck, revive, deadcode, go-sumtype and archlint directly (`SKILL.md`, "This repository").

- **Run linters through golangci-lint, configured in a checked-in `.golangci.yml`.** Why: Uber recommends it "as the go-to lint runner for Go code, largely due to its performance in larger codebases and ability to configure and use many canonical linters at once". Source: [Uber — Lint Runners](https://github.com/uber-go/guide/blob/master/style.md#lint-runners).
- **Lint consistently rather than maximally.** Why: "More importantly than any "blessed" set of linters, lint consistently across a codebase." Source: [Uber — Linting](https://github.com/uber-go/guide/blob/master/style.md#linting).
- **Start from the `standard` set: `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`**, and add on purpose; do not use `default: all`. Why: `all` turns on opinionated linters that contradict each other and the guides. Source: [golangci-lint — Configuration file](https://golangci-lint.run/docs/configuration/file/) ("`standard`: the "Default" linters").
- **Do not enable the `comments` exclusion preset if you want doc comments enforced.** Why: it hides `staticcheck(ST1000|ST1020|ST1021|ST1022)` and revive's "exported (.+) should have comment … or be unexported". Source: [golangci-lint — False positives, Preset comments](https://golangci-lint.run/docs/linters/false-positives/).

Recommended set for community conventions (each enforces something a guide asks for):

| Linter | Enforces | Source |
| -- | -- | -- |
| `govet` | the vet analyzers above | [golangci-lint](https://golangci-lint.run/docs/linters/), `go doc cmd/vet` |
| `staticcheck` | SA/S/ST/QF families (enable the ST checks you want) | [staticcheck.dev](https://staticcheck.dev/docs/checks/) |
| `unused` | unused constants, variables, functions and types | [golangci-lint](https://golangci-lint.run/docs/linters/) |
| `errcheck` | "checking for unchecked errors in Go code"; CRC "Handle Errors" | [errcheck](https://github.com/kisielk/errcheck), [CRC — Handle Errors](https://go.dev/wiki/CodeReviewComments#handle-errors) |
| `ineffassign` | "assignments to existing variables are not used" | [golangci-lint](https://golangci-lint.run/docs/linters/) |
| `revive` | golint's successor; with `exported` it warns on "undocumented exported functions and methods" (CRC "Doc Comments": "All top-level, exported names should have doc comments"); also `context-as-argument`, `error-strings`, `receiver-naming`, `var-naming`, `package-comments`, `unused-parameter` | [revive — rules](https://github.com/mgechev/revive/blob/master/RULES_DESCRIPTIONS.md), [revive — Recommended Configuration](https://github.com/mgechev/revive#recommended-configuration) |
| `errorlint` | Go 1.13 wrapping: `%w` instead of `%v` in `fmt.Errorf`, `errors.Is` instead of `==`, `errors.As` instead of type assertions | [go-errorlint](https://github.com/polyfloyd/go-errorlint) |
| `gocritic` | "diagnostics that check for bugs, performance and style issues"; keep the `#diagnostic` group, leave `#opinionated` off | [go-critic](https://github.com/go-critic/go-critic) |
| `unparam` | "unused function parameters and results" | [unparam](https://github.com/mvdan/unparam) |
| `misspell` | "commonly misspelled English words" in comments and strings | [misspell](https://github.com/golangci/misspell) |
| `nolintlint` | ill-formed or unexplained `//nolint` directives | [golangci-lint](https://golangci-lint.run/docs/linters/) |
| `copyloopvar` | leftover `x := x` copies obsolete since Go 1.22 | [golangci-lint](https://golangci-lint.run/docs/linters/), [Go blog — loopvar](https://go.dev/blog/loopvar-preview) |
| `thelper` | test helpers that do not start with `t.Helper()` | [golangci-lint](https://golangci-lint.run/docs/linters/) |
| `usetesting` | `os.MkdirTemp`/`os.CreateTemp`/`os.Setenv`/`os.Chdir` in tests where `t.TempDir`/`t.Setenv`/`t.Chdir` exist; set `context-background: true` (off by default) for `t.Context` | [golangci-lint — usetesting settings](https://golangci-lint.run/docs/linters/configuration/#usetesting) |
| `testifylint` | correct testify usage (only if the project uses testify) | [golangci-lint](https://golangci-lint.run/docs/linters/) |
| formatters `gofmt` + `goimports` | formatting and imports | [golangci-lint — Formatters](https://golangci-lint.run/docs/formatters/) |

```yaml
version: "2"
linters:
  default: standard
  enable: [revive, errorlint, gocritic, unparam, misspell, nolintlint, copyloopvar, thelper, usetesting]
  settings:
    revive:
      rules:
        - name: exported
    staticcheck:
      checks: ["all"]
formatters:
  enable: [gofmt, goimports]
```

## gopls

- **Use gopls (through the editor) for renames; never rename with search-and-replace.** Why: "Gopls' renaming algorithm takes great care to detect situations in which renaming might introduce a compilation error" (shadowing, a method no longer satisfying an interface). Source: [gopls — Rename](https://go.dev/gopls/features/transformation#rename).
- **Inlay hints show implicit parameter names and struct field names; use them instead of comments that restate arguments.** Source: [gopls — Inlay Hint](https://go.dev/gopls/features/passive#inlay-hint).

## Security and dead code

- **Run `govulncheck ./...` in CI.** Why: it "only surfaces vulnerabilities that actually affect you, based on which functions in your code are transitively calling vulnerable functions". Source: [Go Vulnerability Management](https://go.dev/doc/security/vuln/).
- **Find unreachable functions with `deadcode` (`golang.org/x/tools/cmd/deadcode`); add `-test` for libraries.** Why: it uses Rapid Type Analysis from `main`, so it sees what `unused` cannot (exported but never called); it is sound — "if it reports a function as dead code, it means the function cannot be called" — and "-whylive" explains why a function is kept. With `-test`, a dead function in a library is "a sign that your test coverage could be improved". Source: [Go blog — Finding unreachable functions with deadcode](https://go.dev/blog/deadcode).

## Tests and modules in CI

- **Run `go test -race ./...`.** Why: the race detector "only finds races that happen at runtime"; CI is where tests exercise them. Source: [Data Race Detector](https://go.dev/doc/articles/race_detector).
- **Check `go mod tidy` in CI with `go mod tidy -diff`.** Why: it "causes tidy not to modify go.mod or go.sum but instead print the necessary changes as a unified diff. It exits with a non-zero code if the diff is not empty." Source: `go help mod tidy`.
- **Pin developer tools in `go.mod` with `go get -tool` and run them with `go tool <name>`.** Why: the `-tool` flag "instructs go to add a matching tool line to go.mod", so every contributor and CI run the same version. Source: `go help get`.

## go generate

- **Write directives as `//go:generate command args` (no space after `//`) and run `go generate` explicitly.** Why: "Go generate is never run automatically by go build, go test, and so on." Source: `go help generate`.
- **Mark generated files with `// Code generated … DO NOT EDIT.` before the first non-comment line.** Why: the line must match `^// Code generated .* DO NOT EDIT\.$` so humans and tools (linters, gopls) recognise and skip it. Source: `go help generate`.
- **Commit generated files.** Why: go generate "is for package authors, not clients", so the output "must be checked into the" repository for users who `go get` the package. Source: [Go blog — Generating code](https://go.dev/blog/generate).
- **Check in CI that `go generate ./... && git diff --exit-code` is clean.** Why: it is the only way to catch stale output, since nothing runs the generator automatically. (Project practice that follows from `go help generate`, not a quoted rule.)

## Where the sources disagree

- **Doc comments on every exported name.** CRC requires them ("All top-level, exported names should have doc comments") and revive's `exported` enforces it; staticcheck ships its doc-comment checks off and they only check the shape; golangci-lint's `comments` preset can hide both. We follow CRC: enable revive `exported`, enable ST1020–22, do not use the `comments` preset.
- **How many linters.** Uber's minimum is errcheck, goimports, revive, govet, staticcheck; golangci-lint's standard set is errcheck, govet, ineffassign, staticcheck, unused. They agree in spirit; we take the union plus the linters above that enforce a guide's rule, and refuse `default: all`.
- **Stricter formatters.** gofumpt is offered by golangci-lint but asked for by no style guide. We follow the Go team: `gofmt`/`goimports` only, unless the project already uses gofumpt.
- **Buffered channels as semaphores** (Effective Go) vs Uber's "size one or none" is a concurrency question; see `concurrency.md`.
