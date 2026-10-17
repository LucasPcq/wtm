# Comments

This file covers what Go code says in comments: doc comments (who gets one, how it starts, package and command docs, deprecation, the doc-comment syntax for links, lists, headings and code), comments inside function bodies, comments that excuse code, section separators within a file, TODO/BUG/NOTE notes, `//go:` directives, and keeping comments true. Each rule cites its source. When sources disagree, the Go team wins (go.dev/doc/comment, Effective Go, Go Code Review Comments — "CRC", the standard library's own code), then the Google Go Style Guide, then the Uber Go Style Guide (which has no section on comments), then recognized authors (Dave Cheney's Practical Go); the last section lists each disagreement and the side taken.

## Doc comments: who gets one

- **Give every exported (capitalized) name a doc comment.** Doc comments are the package's documentation in `go doc`, pkg.go.dev and gopls. ([Go Doc Comments](https://go.dev/doc/comment), [CRC — Doc Comments](https://go.dev/wiki/CodeReviewComments#doc-comments), [Google — Doc comments](https://google.github.io/styleguide/go/decisions#doc-comments))
- **Also document non-trivial unexported types and functions — those with unobvious behavior or meaning.** Maintainers are readers too. ([CRC — Doc Comments](https://go.dev/wiki/CodeReviewComments#doc-comments), [Google — Doc comments](https://google.github.io/styleguide/go/decisions#doc-comments))
- **A doc comment sits immediately above the declaration, with no blank line between.** Any blank line detaches it. ([Go Doc Comments](https://go.dev/doc/comment), [Effective Go — Commentary](https://go.dev/doc/effective_go#commentary))
- **One doc comment can introduce a group of related constants or variables, with short end-of-line comments on the members.** ([Go Doc Comments — Consts](https://go.dev/doc/comment#const))

## Doc comments: how they read

- **Write full sentences that begin with the name being declared and end with a period: `// Encode writes the JSON encoding of req to w.`** An article may precede the name (`// A Request represents…`). The explicit subject makes the text clear and searchable with `go doc -all | grep`. ([CRC — Comment Sentences](https://go.dev/wiki/CodeReviewComments#comment-sentences), [Go Doc Comments — Types](https://go.dev/doc/comment#type), [Google — Doc comments](https://google.github.io/styleguide/go/decisions#doc-comments))
- **For a function, say what it returns or, if called for side effects, what it does; refer to parameters and results by name, without backquotes.** ([Go Doc Comments — Funcs](https://go.dev/doc/comment#func))
- **Use "reports whether" for a boolean result, not "returns true if" or "… or not".** `// HasPrefix reports whether the string s begins with prefix.` ([Go Doc Comments — Funcs](https://go.dev/doc/comment#func))
- **For a type, say what each instance represents; state concurrency guarantees beyond single-goroutine use, and the zero value's meaning when it isn't obvious.** ([Go Doc Comments — Types](https://go.dev/doc/comment#type))
- **Leave the algorithm out of the doc comment; it belongs in the body.** Callers need the contract (and maybe complexity bounds), and the implementation stays free to change. ([Go Doc Comments — Funcs](https://go.dev/doc/comment#func))
- **Document special cases and the errors a function returns (sentinels, types, and whether the type is a pointer).** ([Go Doc Comments — Funcs](https://go.dev/doc/comment#func), [Google — Documentation conventions, Errors](https://google.github.io/styleguide/go/best-practices#documentation-conventions-errors))
- **End-of-line comments on struct fields may be phrases with the field as implied subject.** `PageLength int // lines per page when printing`. ([Google — Comment sentences](https://google.github.io/styleguide/go/decisions#comment-sentences))
- **Don't write a doc comment that only says `// Read implements io.Reader.`** It tells the reader to look elsewhere; say what this implementation does. ([Cheney — Always document public symbols](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_always_document_public_symbols))

```go
// ❌
// this function checks the prefix
func HasPrefix(s, prefix string) bool
// ✅
// HasPrefix reports whether the string s begins with prefix.
func HasPrefix(s, prefix string) bool
```

## Package and command comments

- **Every package has one package comment, in one file, starting `// Package name …`, directly above the `package` clause.** Comments in several files are concatenated. ([Go Doc Comments — Packages](https://go.dev/doc/comment#package), [CRC — Package Comments](https://go.dev/wiki/CodeReviewComments#package-comments), [Google — Package comments](https://google.github.io/styleguide/go/decisions#package-comments))
- **Put a long package comment in a `doc.go` that holds only the comment and the `package` clause.** ([Google — Package comments](https://google.github.io/styleguide/go/decisions#package-comments))
- **For a command (`package main`), describe the program's behavior, starting with its name capitalized: `// Gofmt formats Go programs.`** `Command x …`, `Binary x …`, `The x command …` are also fine; a lowercase first word is not. ([Go Doc Comments — Commands](https://go.dev/doc/comment#cmd), [CRC — Package Comments](https://go.dev/wiki/CodeReviewComments#package-comments))
- **Comments for maintainers that concern the whole file go after the imports, not above `package`.** They don't reach godoc. ([Google — Package comments](https://google.github.io/styleguide/go/decisions#package-comments))

## Deprecation

- **Mark a deprecated identifier with a paragraph starting `Deprecated:`, saying why and what to use instead.** Tools warn on use and pkg.go.dev hides the docs; the paragraph need not be last. ([Go Doc Comments — Deprecations](https://go.dev/doc/comment#deprecations))

```go
// Reset zeros the key data and makes the Cipher unusable.
//
// Deprecated: Reset can't guarantee that the key will be entirely removed from
// the process's memory.
func (c *Cipher) Reset()
```

## Doc-comment syntax

- **Separate paragraphs with a blank `//` line.** gofmt keeps your line breaks; semantic linefeeds (one sentence per line) or hand wrapping are both fine. ([Go Doc Comments — Paragraphs](https://go.dev/doc/comment#paragraphs), [Go Doc Comments — Commands](https://go.dev/doc/comment#cmd))
- **Make a heading with `// # Heading` on its own line, set off by blank lines.** The `#` syntax exists since Go 1.19; gofmt rewrites old implicit headings. ([Go Doc Comments — Headings](https://go.dev/doc/comment#headings))
- **Link to identifiers with `[Name]`, `[pkg.Name]`, `[*bytes.Buffer]`; link URLs with `[Text]` plus a `[Text]: URL` definition line at the end.** ([Go Doc Comments — Doc links](https://go.dev/doc/comment#doclinks), [Go Doc Comments — Links](https://go.dev/doc/comment#links))
- **Write lists indented, with `-` (gofmt's canonical bullet) or `1.`; list items hold only paragraphs.** gofmt normalises to two spaces before the dash and four for continuation lines. ([Go Doc Comments — Lists](https://go.dev/doc/comment#lists))
- **Indent code blocks; an unintended indent becomes a code block too.** That is the most common doc-comment mistake. ([Go Doc Comments — Code blocks](https://go.dev/doc/comment#code), [Go Doc Comments — Common mistakes and pitfalls](https://go.dev/doc/comment#mistakes))
- **Prefer a runnable `Example` in a `_test.go` file to a code block in a comment.** It is compiled, tested and shown in godoc. ([Google — Examples](https://google.github.io/styleguide/go/decisions#examples), [CRC — Examples](https://go.dev/wiki/CodeReviewComments#examples))

## Comments inside a function body

- **Explain why, not what: the external constraint, the nuance of the language or the business rule the code can't show.** A comment that restates the code is clutter and a maintenance burden. ([Google — Clarity, Why is the code doing what it does?](https://google.github.io/styleguide/go/guide#clarity-rationale), [Cheney — Comments](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_comments))
- **A "how" comment in a body is acceptable for a non-obvious step (`// queue all dependant actions`).** Cheney gives what/how/why one job each: what for public symbols, how inside a method, why for external factors. ([Cheney — Comments](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_comments))
- **Implementation details (the algorithm) live in body comments, not in the doc comment.** ([Go Doc Comments — Funcs](https://go.dev/doc/comment#func))
- **Before writing a comment, try to make it unnecessary: a better name often hides in the comment.** `// registry of SQL drivers` → rename `registry` to `sqlDrivers` and delete the comment. ([Cheney — Comments on variables and constants should describe their contents not their purpose](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_comments_on_variables_and_constants_should_describe_their_contents_not_their_purpose), [Google — Clarity](https://google.github.io/styleguide/go/guide#clarity-rationale))
- **A comment on a constant or variable describes its contents (where the value comes from), not where it is used.** `StatusOK = 200 // RFC 7231, 6.3.1`. ([Cheney — Comments on variables and constants…](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_comments_on_variables_and_constants_should_describe_their_contents_not_their_purpose))
- **A comment that excuses code ("this is gross") is not enough: rewrite the code, or file the debt as a `TODO`.** ([Cheney — Don't comment bad code, rewrite it](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_dont_comment_bad_code_rewrite_it))
- **If you are commenting a block because it does something separate from the rest of the function, extract it into a function whose name says it.** The name may be all the documentation needed, and the piece can be tested alone. ([Cheney — Rather than commenting a block of code, refactor it](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_rather_than_commenting_a_block_of_code_refactor_it))
- **Use blank lines to break a function into paragraphs.** ([Cheney — Identifier length, TIP](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_identifier_length), [Google — Clarity, What is the code actually doing?](https://google.github.io/styleguide/go/guide#clarity-purpose))

```go
// ❌ the comment restates the code
// increment i
i++
// ✅ the comment gives the reason the code cannot
// Disable HealthyPanicThreshold
HealthyPanicThreshold: &envoy_type.Percent{Value: 0},
```

## Section separators within a file

- **What the standard library does:** a few large files in the Go tree group their declarations under a rule line of `// ` plus 76 dashes, then a title line, then a blank line (or a `//` line and prose). In Go 1.26.1 this appears in 37 of about 4,800 non-test `.go` files, nearly all in `go/ast`, `go/parser`, `go/printer`, `go/token`, `go/types` and its twin `cmd/compile/internal/types2`, plus `math/big` and `text/tabwriter`; the rest of the library uses none. ([go/ast/ast.go](https://go.dev/src/go/ast/ast.go), [go/parser/parser.go](https://go.dev/src/go/parser/parser.go), [go/types/struct.go](https://go.dev/src/go/types/struct.go))

```go
// ----------------------------------------------------------------------------
// Expressions and types

// A Field represents a Field declaration list in a struct type,
```

- **What the guides say:** neither Effective Go, CRC, go.dev/doc/comment, Google nor Uber endorses or forbids separator comments by name. Google's commentary section advises that, in godoc, "apart from indentation, decoration should generally be avoided", and its clarity section lists "break up the code with whitespace and comments" as a tool; Uber orders a file by type, constructor, methods, then helpers, with no banners. ([Google — Commentary](https://google.github.io/styleguide/go/decisions#commentary), [Google — Clarity](https://google.github.io/styleguide/go/guide#clarity-purpose), [Uber — Function Grouping and Ordering](https://github.com/uber-go/guide/blob/master/style.md#function-grouping-and-ordering))
- **Rule we take:** a separator is legitimate only in a long file whose declarations fall into named groups, in the stdlib's exact shape; leave a blank line after the title so it does not become the next declaration's doc comment. In an ordinary file, split the file instead (a file named after what it declares) — the practice of 99% of the standard library. ([Go Doc Comments](https://go.dev/doc/comment) for the doc-comment adjacency rule)
- **Inside a struct, a plain comment line can label a group of fields (`// Dependencies:`).** Google shows it in its `Options` example. ([Google — Doc comments](https://google.github.io/styleguide/go/decisions#doc-comments))

## TODO, BUG and NOTE

- **Write a note as `// MARKER(uid): body`, with `MARKER` two or more uppercase letters (`TODO`, `BUG`) and `uid` a person who can tell more.** Notes are collected and rendered in their own section on pkg.go.dev. ([Go Doc Comments — Notes](https://go.dev/doc/comment#notes))
- **The username is who to ask, not a promise to fix; an issue number or date also works.** `// TODO(dfc) this is O(N^2), find a faster way to do this.` ([Cheney — Don't comment bad code, rewrite it](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_dont_comment_bad_code_rewrite_it))
- **The standard library mostly writes `TODO(name):`; a bare `TODO` is also common, `NOTE` rarer.** Counted in Go 1.26.1's `src`: about 1,500 `// TODO(` against about 900 bare `// TODO`.

## Directives

- **Write a directive with no space after `//`: `//go:generate`, `//go:build`, `//go:embed`, `//go:noinline`; one per line, at line start.** A space turns it into an ordinary comment. ([cmd/compile — Compiler Directives](https://pkg.go.dev/cmd/compile#hdr-Compiler_Directives), [Go Doc Comments — Directives](https://go.dev/doc/comment#directives))
- **Directives are not part of the doc comment; gofmt moves them to its end after a blank `//` line.** Tools may define their own as `//toolname:directive args`; the `go` tool name is reserved. ([Go Doc Comments — Directives](https://go.dev/doc/comment#directives))

## Keep comments true

- **Update or delete a comment in the same change as the code it describes.** A comment that contradicts the code is worse than none; every comment is a maintenance burden. ([Google — Clarity, Why is the code doing what it does?](https://google.github.io/styleguide/go/guide#clarity-rationale))
- **Doc comments describe the contract, not the current algorithm, so they survive refactors.** ([Go Doc Comments — Funcs](https://go.dev/doc/comment#func))
- **Preview docs with `go doc` or `pkgsite` during review.** ([Google — Preview](https://google.github.io/styleguide/go/best-practices#documentation-preview))

## Where the sources disagree

- **Documenting a method that implements an interface.** go.dev/doc/comment: "Every exported (capitalized) name should have a doc comment." Cheney: you don't need to document methods that implement an interface, and should delete `// Read implements the io.Reader interface`; the `io.LimitedReader.Read` method he quotes indeed has none. **We follow the Go team: document it, and make the comment say what this implementation does** rather than only naming the interface. ([Go Doc Comments](https://go.dev/doc/comment), [Cheney — Always document public symbols](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_always_document_public_symbols))
- **Wrapping comment lines.** Google: wrap long comment lines (80 or 100 columns are common) and avoid one very long line per paragraph. go.dev/doc/comment: semantic linefeeds or hand wrapping, "Whatever is best for your code base is fine", since go doc rewraps. **We follow the Go team: either style, consistent within the file.** ([Google — Comment line length](https://google.github.io/styleguide/go/decisions#comment-line-length), [Go Doc Comments — Commands](https://go.dev/doc/comment#cmd))
- **Heading syntax.** Google's best practices still describe the pre-1.19 implicit heading (a capitalized line without punctuation). go.dev/doc/comment defines `# Heading` and says gofmt rewrites implicit ones. **We follow the Go team: `# Heading`.** ([Google — Godoc formatting](https://google.github.io/styleguide/go/best-practices#godoc-formatting), [Go Doc Comments — Headings](https://go.dev/doc/comment#headings))
- **What a body comment may say.** Google says comments should usually explain why, not what; Cheney also accepts "how" inside a method. Both reject comments that restate code. **We follow Google (higher precedence): why first; a how comment only for a step the code can't make obvious.** ([Google — Clarity](https://google.github.io/styleguide/go/guide#clarity-rationale), [Cheney — Comments](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_comments))
