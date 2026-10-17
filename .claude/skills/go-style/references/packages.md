# Packages: module layout, naming, size and dependencies

This file covers how a Go module is laid out and how its code is split into packages: the module layout, `cmd/` and `internal/`, package names and size, import cycles and layering, package docs, package-level state, dependency injection, and how recognized authors structure applications. Every rule cites its source. When sources disagree, the Go team (go.dev/doc/modules/layout, Effective Go, Go Code Review Comments, the Go blog, the standard library) wins over the Google Go Style Guide, which wins over the Uber Go Style Guide, which wins over individual authors (Dave Cheney, Ben Johnson, Rob Pike's proverbs, Mat Ryer). The last section lists each disagreement and the side we follow.

## Module layout

- Start flat: a small package or command keeps all its files at the module root, split across several files of the same package. Why: the official layout guide grows the structure only as the project does. Source: [Organizing a Go module, Basic package / Basic command](https://go.dev/doc/modules/layout).
- Put supporting packages under `internal/` first. Why: "Since other projects cannot import code from our internal directory, we're free to refactor its API and generally move things around without breaking external users"; "It's recommended to keep packages in internal as much as possible." Source: [Organizing a Go module, Package or command with supporting packages / Multiple packages](https://go.dev/doc/modules/layout).
- Put commands in `cmd/<name>/` when the repository mixes importable packages and commands, or for a server project; it "isn't strictly necessary in a repository that consists only of commands". Source: [Organizing a Go module, Multiple commands / Server project](https://go.dev/doc/modules/layout).
- For a server, keep the Go packages implementing its logic in `internal/`; split a package that becomes useful to other projects into its own module. Source: [Organizing a Go module, Server project](https://go.dev/doc/modules/layout).
- Do not create a `pkg/` directory: the official layout never uses one, and Russ Cox wrote that "the vast majority of packages in the Go ecosystem do not put the importable packages in a pkg subdirectory" and that golang-standards/project-layout is "not a standard Go project layout". Source: [Organizing a Go module](https://go.dev/doc/modules/layout), [rsc, golang-standards/project-layout#117](https://github.com/golang-standards/project-layout/issues/117).
- Keep `package main` small: parse flags, open connections, build the dependencies, then hand off to a high-level object. Why: `main.main` is a singleton and hard to test. Source: [Cheney, Practical Go §5.2](https://dave.cheney.net/practical-go/presentations/qcon-china.html), [Ryer, How I write HTTP services in Go after 13 years ("func main() only calls run()")](https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/).
- Avoid intermediate directories that contain no `.go` files; every package except `cmd/` and `internal/` should contain source. Why: the package hierarchy has no meaning to the go tool (`net/http` is not a child of `net`). Source: [Cheney, Practical Go §5.1](https://dave.cheney.net/practical-go/presentations/qcon-china.html).

```
project/
  go.mod
  cmd/tool/main.go      // wires dependencies, calls run
  internal/auth/        // one concept per package
  internal/store/
```

## Package names

- Name a package for what it provides, in one short lower-case word, no underscores or mixedCaps. Source: [Effective Go, Package names](https://go.dev/doc/effective_go#package-names), [Google decisions, Package names](https://google.github.io/styleguide/go/decisions#package-names), [Cheney, Practical Go §4.1](https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- Do not name a package `util`, `common`, `misc`, `helper`, `model`, `base`, `shared`, `lib`, `api`, `types` or `interfaces`. Why: the name tells the caller nothing, the package grows without bound, accumulates dependencies and collides with other imports. Source: [Go blog, Package names ("Bad package names")](https://go.dev/blog/package-names), [CRC, Package Names](https://go.dev/wiki/CodeReviewComments#package-names), [Google best practices, Util packages](https://google.github.io/styleguide/go/best-practices#util-packages), [Uber, Package Names](https://github.com/uber-go/guide/blob/master/style.md#package-names).
- Do not repeat the package name in its identifiers: `chubby.File`, not `chubby.ChubbyFile`. Source: [CRC, Package Names](https://go.dev/wiki/CodeReviewComments#package-names).
- Fix a `util` package by moving each function to its caller's package, or to a package named for the shared concept, even at the price of some duplication. Why: "A little copying is better than a little dependency." Source: [Cheney, Practical Go §4.2](https://dave.cheney.net/practical-go/presentations/qcon-china.html), [Go Proverbs](https://go-proverbs.github.io/).
- Do not put all the program's interfaces or types in one package (`api`, `types`, `interfaces`). Source: [Go blog, Package names](https://go.dev/blog/package-names).

```go
// ❌ the name says nothing at the call site
b := helper.Marshal(curve, x, y)
// ✅
b := elliptic.Marshal(curve, x, y)
```

## Package size

- A package holds one concept; godoc shows it as one page. Put tightly coupled types in the same package: "if the user must import both packages in order to use either in any meaningful way, combining them together is usually the right thing to do." Source: [Google best practices, Package size](https://google.github.io/styleguide/go/best-practices#package-size).
- Give a conceptually distinct thing its own small package, but do not put the whole project in one package. Source: [Google best practices, Package size](https://google.github.io/styleguide/go/best-practices#package-size).
- Prefer fewer, larger packages; "your default position should be to not create a new package". Why: many small packages force many exported identifiers and a wide, shallow API. Source: [Cheney, Practical Go §5.1](https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- Merge client, server and common code into one package named for its function, with one file each (`net/http`'s `client.go`, `server.go`, `transport.go`), rather than a `common` package. Source: [Cheney, Practical Go §4.2](https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- A Go package is closer to a whole Maven module than to a Java package. Source: [Cheney, Practical Go §5.1](https://dave.cheney.net/practical-go/presentations/qcon-china.html).

## Import cycles and layering

- Import cycles are illegal: "It is illegal for a package to import itself, directly or indirectly." Source: [The Go spec, Import declarations](https://go.dev/ref/spec#Import_declarations).
- Treat an interface introduced only to break a cycle as a smell: it "is often a signal of improperly structured packages. Consolidated packages are often preferred over too many too small packages that fail to stand on their own." Source: [Google best practices, Interfaces (Avoid unnecessary interfaces, Avoiding circular dependencies)](https://google.github.io/styleguide/go/best-practices#interfaces).
- Make dependencies point one way: a domain package depends on nothing in the application, implementation packages depend on the domain, `main` depends on everything. Why: a one-way graph cannot cycle. Source: [Ben Johnson, Standard Package Layout ("The root package should not depend on any other package in your application!")](https://www.gobeyond.dev/standard-package-layout/).
- Do not create a `utils` package to escape a cycle; its name then "doesn't reflect its purpose, only its function of breaking the import cycle." Source: [Cheney, Practical Go §4.2](https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- Enforce boundaries with `internal/`: a package under `a/b/internal/` can be imported only from the tree rooted at `a/b`. Source: [Cheney, Practical Go §5.1.3](https://dave.cheney.net/practical-go/presentations/qcon-china.html), [Organizing a Go module](https://go.dev/doc/modules/layout).

## Package documentation

- Give every package exactly one package comment, adjacent to the `package` clause, starting `Package x ...`; for a command, `Command x ...` or a variant. Source: [CRC, Package Comments](https://go.dev/wiki/CodeReviewComments#package-comments), [Google decisions, Package comments](https://google.github.io/styleguide/go/decisions#package-comments).
- "Design the architecture, name the components, document the details." Source: [Go Proverbs](https://go-proverbs.github.io/).

## Package-level state

- Do not make clients use APIs that rely on global state; let them create instance values (`sidecar.New()`) and pass them explicitly. Why: global state makes tests order-dependent, forbids two configurations in one process, and imposes init-order rules. Source: [Google best practices, Global state](https://google.github.io/styleguide/go/best-practices#global-state).
- Global state is safe only when logically constant, observably stateless (a private cache), not leaking outside the process, or with no expectation of predictable behavior. Source: [Google best practices, Litmus tests](https://google.github.io/styleguide/go/best-practices#litmus-tests).
- A package-level convenience API (`http.Handle` over `http.DefaultServeMux`) must be a thin proxy over an instance API and be used only by binaries, not libraries. Source: [Google best practices, Providing a default instance](https://google.github.io/styleguide/go/best-practices#providing-a-default-instance).
- Avoid package-level state; move variables onto the structs that need them. Source: [Cheney, Practical Go §4.5](https://dave.cheney.net/practical-go/presentations/qcon-china.html), [Uber, Avoid Mutable Globals](https://github.com/uber-go/guide/blob/master/style.md#avoid-mutable-globals).

## Dependency injection

- Pass dependencies "as parameters to constructors, functions, methods, or struct fields on the call chain". Source: [Google best practices, Global state](https://google.github.io/styleguide/go/best-practices#global-state).
- Prefer dependency injection to mutating globals, including function pointers such as `time.Now`. Source: [Uber, Avoid Mutable Globals](https://github.com/uber-go/guide/blob/master/style.md#avoid-mutable-globals).
- Wire the dependencies by hand in `main`: "all the term really means is that we're going to pass dependencies to our objects instead of requiring that the object build or find the dependency itself"; "The main package is what gets to choose which dependencies to inject into which objects." Source: [Ben Johnson, Standard Package Layout, #4](https://www.gobeyond.dev/standard-package-layout/).
- Pass all of a constructor's dependencies as arguments, and the OS fundamentals (args, env, stdio) to `run`. Source: [Ryer, How I write HTTP services in Go after 13 years](https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/).
- No source recommends a DI framework or container; Ben Johnson notes the term "conjures up thoughts of verbose Spring XML files" and shows plain wiring, and Google's least mechanism prefers core constructs. Source: [Ben Johnson](https://www.gobeyond.dev/standard-package-layout/), [Google guide, Least mechanism](https://google.github.io/styleguide/go/guide#least-mechanism).

```go
// ✅ main is the only place that knows the concrete types
func main() {
	db, err := postgres.Open(os.Getenv("DB"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	h := &http.Handler{UserService: &postgres.UserService{DB: db}}
	// start the server with h
}
```

## Structuring an application

- **Ben Johnson's Standard Package Layout** ([gobeyond.dev](https://www.gobeyond.dev/standard-package-layout/)) has four tenets: the root package holds the domain types (plain structs such as `User`, and service interfaces such as `UserService`) and depends on nothing; subpackages are grouped by dependency (`postgres`, `http`, even wrapping the standard library), each "an adapter between your domain and your implementation"; a shared `mock` subpackage holds hand-written mocks; `main` (under `cmd/`) ties the dependencies together and "is also an adapter". He cites the standard library: `io.Reader` is the domain type, `gzip.Reader`, `tar.Reader` the implementations grouped by dependency.
- **Approaches he rejects**: a monolithic package (works up to about 10K SLOC), a Rails-style layout by kind (`controller.UserController`, cycles), and grouping by module (`users`, `accounts`), which leads to "terrible names like users.User" and the same cycles. Source: [Ben Johnson, Common flawed approaches](https://www.gobeyond.dev/standard-package-layout/).
- **Organizing by feature**: none of the Go team, Google or Uber sources prescribes it; Google's coupling test (combine packages a user must import together) and Johnson's warning about `users.User` and cycles are the only relevant statements found.
- **Hexagonal / clean architecture**: none of the sources read (go.dev layout, Effective Go, CRC, Go blog on package names and organizing code, Google, Uber, Cheney, Johnson, Ryer, Proverbs) names hexagonal, ports-and-adapters or clean architecture. Johnson's layout is the closest: a dependency-free domain and adapter packages per dependency, wired in `main`. Mapping it onto ports and adapters is our reading, not a quoted position.

## Where the sources disagree

- **How many packages.** Cheney: "prefer fewer, larger packages", default to not creating one. Ben Johnson: one package per external dependency, the domain at the root. Google: one concept per package, combine tightly coupled ones, avoid one giant package. We follow Google's coupling test; a package per dependency is justified only when it isolates a real dependency.
- **Where interfaces live.** Go team (CRC) and Google: the consumer defines the interface. Ben Johnson: the domain root package declares `UserService` for every implementation. We follow the Go team; a shared domain package is acceptable when it is itself the consumers' vocabulary (Google's "the interface is the product" exception), see `interfaces.md`.
- **Shared mock package.** Ben Johnson: a shared `mock` subpackage. Google: test doubles go in a `<pkg>test` package and are not exported from the API that consumes the interface; CRC: the consumer writes its fake in its own test. We follow the Go team and Google.
- **`cmd/` for a commands-only repository.** Go team: not strictly necessary. Johnson and the server-project guidance use it. We use `cmd/` when the module also has importable packages or several binaries, otherwise the root.
- **`pkg/`.** The golang-standards/project-layout repository (not a recognized source) uses it; the Go team's layout does not, and Russ Cox rejects it as a standard. We follow the Go team.
