# Interfaces, receivers and embedding

This file covers where an interface is declared and how big it is, what a constructor returns, compile-time compliance checks, pointer and value receivers, embedding, how consumer-side interfaces play the role of ports, and test doubles. Every rule cites its source. When sources disagree, the Go team (Effective Go, Go Code Review Comments, the Go blog, the standard library) wins over the Google Go Style Guide, which wins over the Uber Go Style Guide, which wins over individual authors (Dave Cheney, Rob Pike's proverbs, Ben Johnson). The last section lists each disagreement and the side we follow.

## Where an interface is declared

- Declare an interface in the package that uses it, not the package that implements it. Why: the implementing package can then add methods without a refactoring. Source: [CRC, Interfaces](https://go.dev/wiki/CodeReviewComments#interfaces), [Google decisions, Interfaces](https://google.github.io/styleguide/go/decisions#interfaces), [Google best practices, Interface ownership and visibility](https://google.github.io/styleguide/go/best-practices#interface-ownership-and-visibility).
- The consumer lists only the methods it calls. Source: [Google best practices, Interface ownership and visibility](https://google.github.io/styleguide/go/best-practices#interface-ownership-and-visibility).
- The producer may export the interface when "the interface is the product": a protocol many implementations follow (`io.Writer`, `hash.Hash`), a generated contract, or to avoid many packages mirroring the same interface. Source: [Google best practices, Interface ownership and visibility](https://google.github.io/styleguide/go/best-practices#interface-ownership-and-visibility).
- Keep an interface unexported when only its own package uses it. Source: [Google decisions, Interfaces](https://google.github.io/styleguide/go/decisions#interfaces).
- Let a function state only the behavior it needs: `Save(w io.Writer, doc *Document)` rather than `Save(f *os.File, ...)`. Why: the narrowest requirement makes the function the most general and the easiest to test. Source: [Cheney, Practical Go §6.3](https://dave.cheney.net/practical-go/presentations/qcon-china.html).

```go
// ✅ consumer.go — the consumer owns the interface
package consumer

type Thinger interface{ Thing() bool }

func Foo(t Thinger) string { /* ... */ }

// ❌ producer.go — interface on the implementor side, constructor returns it
package producer

type Thinger interface{ Thing() bool }

func NewThinger() Thinger { return defaultThinger{} }
```

## When to create one

- Do not define an interface before it is used: "without a realistic example of usage, it is too difficult to see whether an interface is even necessary, let alone what methods it ought to contain." Source: [CRC, Interfaces](https://go.dev/wiki/CodeReviewComments#interfaces).
- Do not define interfaces on the implementor side "for mocking"; design the API so it can be tested through the real implementation's public API. Source: [CRC, Interfaces](https://go.dev/wiki/CodeReviewComments#interfaces), [Google best practices, Avoid unnecessary interfaces](https://google.github.io/styleguide/go/best-practices#avoid-unnecessary-interfaces).
- Do not create a named interface just because the thing is a "service" or a "repository"; start from the concrete type. Source: [Google best practices, Avoid unnecessary interfaces](https://google.github.io/styleguide/go/best-practices#avoid-unnecessary-interfaces).
- Create one for a real need: two or more concrete types handled by the same logic, a consumer that needs one or two methods of a large type, or (as a last resort) breaking a dependency cycle. Source: [Google best practices, Avoid unnecessary interfaces](https://google.github.io/styleguide/go/best-practices#avoid-unnecessary-interfaces).
- Reuse an existing interface (`io.Reader`, a generated RPC client) instead of wrapping it in a new one. Source: [Google best practices, Avoid unnecessary interfaces](https://google.github.io/styleguide/go/best-practices#avoid-unnecessary-interfaces).

## Size

- Keep interfaces small; one or two methods is common, named after the method (`io.Writer`). Source: [Effective Go, Interfaces](https://go.dev/doc/effective_go#interfaces), [Google best practices, Designing effective interfaces](https://google.github.io/styleguide/go/best-practices#designing-effective-interfaces).
- "The bigger the interface, the weaker the abstraction." Source: [Go Proverbs](https://go-proverbs.github.io/).
- Build larger interfaces by embedding small ones (`io.ReadWriter` embeds `io.Reader` and `io.Writer`). Source: [Effective Go, Embedding](https://go.dev/doc/effective_go#embedding).
- Document an interface's contract, edge cases and errors; each method of a multi-method interface needs its own doc, and unexported ones deserve docs too. Source: [Google best practices, Designing effective interfaces](https://google.github.io/styleguide/go/best-practices#designing-effective-interfaces).
- `interface{}` (`any`) says nothing: type at the source. Source: [Go Proverbs](https://go-proverbs.github.io/).

## Accept interfaces, return concrete types

- Constructors and functions return concrete types (usually a pointer or a struct). Why: the caller gets every method and can still pass the value to anything that expects an interface. Source: [CRC, Interfaces](https://go.dev/wiki/CodeReviewComments#interfaces), [Google decisions, Interfaces](https://google.github.io/styleguide/go/decisions#interfaces) (citing GoTip #49).
- Its limits: return an interface for `error`; when the function picks one of several concrete types at run time (factory, strategy, command, chaining); to hide methods whose use would break the type's integrity; or to avoid an import cycle. Source: [Google best practices, Designing effective interfaces](https://google.github.io/styleguide/go/best-practices#designing-effective-interfaces).
- If a type exists only to implement an interface and has no other exported methods, export the interface and keep the type unexported. Source: [Effective Go, Generality](https://go.dev/doc/effective_go#generality).
- Never take a pointer to an interface; pass the interface value. Source: [Uber, Pointers to Interfaces](https://github.com/uber-go/guide/blob/master/style.md#pointers-to-interfaces), [CRC, Pass Values](https://go.dev/wiki/CodeReviewComments#pass-values).

## Compile-time compliance checks

- Assert `var _ I = (*T)(nil)` only when nothing else in the code converts `T` to `I` statically: "Don't do this for every type that satisfies an interface ... such declarations are only used when there are no static conversions already present in the code, which is a rare event." Source: [Effective Go, Interface checks](https://go.dev/doc/effective_go#blank_implements).
- Typical cases: an exported type that must implement an interface as part of its API contract, or a family of types implementing the same interface. The right-hand side is the zero value of the asserted type. Source: [Uber, Verify Interface Compliance](https://github.com/uber-go/guide/blob/master/style.md#verify-interface-compliance).

```go
type Handler struct{ /* ... */ }

var _ http.Handler = (*Handler)(nil) // fails to compile if *Handler stops implementing it
```

## Receivers

- Use a pointer receiver when the method mutates the receiver, when the struct holds a `sync.Mutex` or other non-copyable field, or when the struct is large; when in doubt, use a pointer. Source: [CRC, Receiver Type](https://go.dev/wiki/CodeReviewComments#receiver-type), [Google decisions, Receiver type](https://google.github.io/styleguide/go/decisions#receiver-type).
- Use a value receiver for maps, funcs, chans, slices not resliced, small immutable value types (`time.Time`) and basic types. Source: [CRC, Receiver Type](https://go.dev/wiki/CodeReviewComments#receiver-type).
- Do not mix receiver kinds on one type: all pointer or all value methods. Source: [CRC, Receiver Type](https://go.dev/wiki/CodeReviewComments#receiver-type), [Google decisions, Receiver type](https://google.github.io/styleguide/go/decisions#receiver-type) ("prefer").
- Remember method sets: value methods are callable on values and pointers, pointer methods only on pointers or addressable values; only `*T` satisfies an interface whose methods have pointer receivers. Source: [Effective Go, Pointers vs. Values](https://go.dev/doc/effective_go#pointers_vs_values), [Uber, Receivers and Interfaces](https://github.com/uber-go/guide/blob/master/style.md#receivers-and-interfaces).
- Choose by correctness, not by guessed performance; profile before choosing a value receiver for speed. Source: [Google decisions, Receiver type](https://google.github.io/styleguide/go/decisions#receiver-type), [CRC, Receiver Type](https://go.dev/wiki/CodeReviewComments#receiver-type).
- Name the receiver with one or two letters of its type, consistently; never `this` or `self`. Source: [CRC, Receiver Names](https://go.dev/wiki/CodeReviewComments#receiver-names).

## Embedding

- Embedding promotes the inner type's methods, but the receiver of a promoted method is the inner value, not the outer: it is not subclassing. Source: [Effective Go, Embedding](https://go.dev/doc/effective_go#embedding).
- Do not embed types in exported structs: the embedded type becomes part of the API, so adding a method to an embedded interface, removing one from an embedded struct, or replacing the embedded type are all breaking changes. Write delegating methods on a named field instead. Source: [Uber, Avoid Embedding Types in Public Structs](https://github.com/uber-go/guide/blob/master/style.md#avoid-embedding-types-in-public-structs).
- Embed only for a tangible benefit with no user-facing side effect; litmus test: would every exported inner method be added to the outer type by hand? If not, use a field. Never embed a mutex. Put embedded fields first, separated by a blank line. Source: [Uber, Embedding in Structs](https://github.com/uber-go/guide/blob/master/style.md#embedding-in-structs).

```go
// ❌ A.Lock and A.Unlock become part of A's API
type A struct{ sync.Mutex }

// ✅
type A struct{ mu sync.Mutex }
```

## Interfaces as ports

- In ports-and-adapters terms, a port is the interface the core declares for what it needs, and an adapter is the concrete type that satisfies it implicitly. Go's consumer-side rule produces exactly that: the package that needs storage declares a small interface with the methods it calls, the storage package returns a concrete type, and `main` wires them. The mapping is our synthesis: none of the sources read uses the words hexagonal or ports; Ben Johnson calls his dependency subpackages "an adapter between your domain and your implementation" ([Standard Package Layout](https://www.gobeyond.dev/standard-package-layout/)).
- Consequences drawn from the rules above: one port per consumer need, not one per implementation; the adapter's constructor returns its concrete type; a port is not created until a consumer calls it; a port with many methods is a weak abstraction; keep it unexported when only its package uses it. Sources: [CRC, Interfaces](https://go.dev/wiki/CodeReviewComments#interfaces), [Google best practices, Interfaces](https://google.github.io/styleguide/go/best-practices#interfaces), [Go Proverbs](https://go-proverbs.github.io/).

## Test doubles

- A test double may be a stub, fake, mock or spy. Put reusable doubles in a package named `<pkg>test` (`creditcardtest`), and name a double by the behavior it emulates (`AlwaysCharges`, `AlwaysDeclines`) or, for a single type, simply `Stub`. Source: [Google best practices, Test double and helper packages](https://google.github.io/styleguide/go/best-practices#test-double-and-helper-packages).
- The consumer writes its fake in its own test file and passes it through its own interface. Source: [CRC, Interfaces](https://go.dev/wiki/CodeReviewComments#interfaces).
- Do not export a test double from the API that consumes the interface; export one only for a material need of substitution. Source: [Google best practices, Avoid unnecessary interfaces](https://google.github.io/styleguide/go/best-practices#avoid-unnecessary-interfaces).
- Prefer real objects; for HTTP or RPC, use the real client connected to a test server rather than a hand-written client double. Source: [Google best practices, Use real transports](https://google.github.io/styleguide/go/best-practices#use-real-transports), [Google best practices, Litmus tests ("our codebase's preference for real objects")](https://google.github.io/styleguide/go/best-practices#litmus-tests).
- Fakes versus mocks: Google names the four kinds and gives no ranking between fakes and mocks; Ben Johnson prefers hand-written function-field mocks over generated ones ("I find many of the mocking tools to be overly complicated"). Source: [Google best practices, Test double and helper packages](https://google.github.io/styleguide/go/best-practices#test-double-and-helper-packages), [Ben Johnson, #3](https://www.gobeyond.dev/standard-package-layout/).

```go
// ✅ consumer_test.go
type fakeThinger struct{ ok bool }

func (f fakeThinger) Thing() bool { return f.ok }

if Foo(fakeThinger{ok: true}) != "x" { t.Errorf(...) }
```

## Where the sources disagree

- **Compliance checks.** Effective Go: rare, only when no static conversion exists. Uber: "where appropriate", including every type of a family. We follow Effective Go (higher precedence) and use Uber's list as the cases where a static conversion is usually missing.
- **Returning an interface.** CRC and Google: return concrete types. Effective Go, Generality: export only the interface when the type exists only to implement it. Google reconciles them with explicit exceptions (encapsulation, runtime choice, cycles). We return concrete types by default and an interface only in those cases.
- **Who declares the interface.** CRC and Google: the consumer. Ben Johnson: the domain root package declares service interfaces shared by all implementations. We follow the Go team; a producer-side or shared interface only when it is the product.
- **Shared mocks.** Ben Johnson: a shared `mock` subpackage. Google: `<pkg>test` packages, never exported from the consuming API, and real objects preferred. We follow Google and CRC.
- **Embedding for convenience.** Effective Go: "Embedding can also be a simple convenience" (`Job` embedding `*log.Logger`). Uber: embedding "should not be purely cosmetic or convenience-oriented" and never in public structs. Effective Go does not address API evolution; we embed for convenience in unexported types only and follow Uber for exported ones, where it adds a constraint Effective Go does not contradict.
- **Mixing receivers.** CRC: "Don't mix receiver types." Google: "prefer" all pointer or all value. Same direction, different strength; we follow CRC.
