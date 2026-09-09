# The output layer

How a `wtm` command speaks. `CLAUDE.md` carries the rules in short form; this is the reasoning behind them, and the reference to read before adding a command or changing what one prints.

## The one question

A block earns its place when it changes what the reader does next. Not when it is true, not when it was expensive to compute, not when it is interesting — when it changes what they do. Everything below follows from that.

The question has a corollary that is easy to get backwards, so it is worth stating on its own: **success contracts, an anomaly expands.** A pass that did exactly what was asked is a count. A refusal, a conflict, a link matching nothing is named one by one, because the reader can only fix the one they can see. Giving the nominal path as much room as the actionable one is what makes a CLI read as noise — and it is the shape most reports drift into, because listing what happened is easier than deciding what matters.

Two more consequences, in the order they bite:

**Detail belongs to the command whose subject it is.** Ports are the subject of `wtm env` and `wtm run init`; in `create` and `extract` they are a side effect, so they collapse to a count on the recap's env line. A reader who wants the values runs the command that is about them, or opens the file the run just wrote. The file is the record; the command says how many and where.

**A successful run has a fixed shape.** What makes output feel bloated is not its size but its variance: a conclusion whose height depends on what happened can never be recognised at a glance, so it has to be read. `wtm create` is the same six lines whether it settled three ports or thirty.

## Two streams, three registers

stdout is the result — what a script would read. stderr is everything about getting there.

Three registers, and only the third may grow with what happened:

| Register | Where | Lives for | Examples |
| -- | -- | -- | -- |
| **Result** | stdout | the scrollback | the framed conclusion, a table, a state readout |
| **Progress** | stderr | until it is replaced | spinners, a hook's tail, a job's raw output |
| **Attention** | stderr | the scrollback | warnings, refusals, anomalies, callouts |

Progress is erased, so it is never barred and never framed — the bar marks what stays. Attention is the only register allowed one line per item.

## The frame

Every human conclusion is framed **exactly once**, with `output.Frame` or — for a command writing across two streams — the `FrameStart`/`FrameEnd` pair. The frame owns two things at once: the single blank line above and below the block, and the accent bar down its left edge.

```go
output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
    output.Success(w, "Created worktree feat/x")
})
```

The body writes to the writer it is **handed**, never to the one `Frame` was given. That is what puts the bar on every line, in the one place the padding is already applied. A formatter therefore emits a raw body: no leading blank, no trailing blank, `output.Blank` only as a genuine separator between sections inside the block.

A streaming pair wraps its own body writer: `output.Barred(w)`. When a command writes across two streams — `sync`'s plan on stderr, its recap on stdout — there is one rule rather than two mechanisms: **every section opens with exactly one blank line on the stream it is about to write to**, the first of them being the frame's leading blank, and `FrameEnd` closes. Same call, same output, one mechanism.

JSON and machine output are never framed and therefore never barred. They emit flush.

### The bar

`┃`, in column zero — left of everything else the CLI prints, which is what makes it a marker rather than one more indent. In a terminal running `git`, `pnpm` and `docker`, it says *this block is wtm speaking*.

It goes on a **terminal only** (`output.IsTerminal`). A pipe, a CI log or a redirection gets the bare text, so `wtm create | tee log` stays clean and a grep over that log never has to know about the bar.

## The four levels, and what each is for

A visual system holds by its contrasts, not by its repetitions. If everything is marked, nothing is.

| Level | For | Where |
| -- | -- | -- |
| **A flat line** | an act you just performed | `create`, `clean`, `checkout`, `run job add` |
| **A table** | an inventory you consult | `list`, `tree`, `run ps`, `run list` |
| **A pill-titled block** (`styles.RenderRecap`) | a state you come back to | `init`, `run up`, `run down` |
| **A callout** (`output.Callout`, bordered) | something still to act on | port isolation, proxy hints, withheld bindings |

The pill is the contrast element and stays rare. A one-line conclusion in a box is five lines of chrome around one line of content — that is the reductio, and it is why the box is not the standard.

## The two shapes of a conclusion

**Form A — the act.** One subject:

```
┃  ✓ Created worktree feat/x
┃
┃  from  main
┃  env   main · 4 port(s) shifted (+10)
┃  path  .worktrees/feat-x
┃
┃  → wtm go feat/x
```

A `✓` headline, nought to three aligned fields, at most one next step. Budget: 8 lines.

**Form B — the readout.** Several objects:

```
┃  ✓ 3 pruned · 1 skipped
┃  feat/a, feat/b, feat/c
┃
┃  ! docs/api skipped — open PR #42
```

`output.Tally` counts, zero counts dropped; then **one line per exception only**, never per success. Budget: 6 lines plus the exceptions.

One nuance that is not a matter of taste: a **destructive** run names what it destroyed — knowing what is gone is actionable — but on one line, because the picker and the recap have already shown that list twice. A non-destructive run counts.

## The glyph vocabulary

Exhaustive. One glyph per line, at its head; never two vocabularies in one block.

| Glyph | Means | Helper |
| -- | -- | -- |
| `✓` | changed state, and it worked | `output.Success` |
| `=` | was already in the desired state | `output.Unchanged` |
| `↻` | an existing thing was replaced | `output.Update` |
| `!` | needs attention; the run continues | `output.Warning` / `output.Danger` |
| `✗` | failed | `output.Error` |
| `›` | in progress — ephemeral only | `output.Loading` |
| `→` | what to do next | `output.NextStep` |

**"Nothing to do" is `=`, everywhere.** It used to be a bare line for some commands, a green `✓` for others, an `=` for two and a neutral field for one — the same non-event in turn a victory, a no-op and a piece of trivia.

**A hint is `output.NextStep`, everywhere**: one arrow, one bold command, an optional muted note. A reader learns once where to look for what to do next.

**`output.Message` — the bare line, carrying no status — is not for a conclusion.** It is the most-called helper in the tree, and that is the symptom it names: when nothing in the vocabulary fits, people fall back to a line that says nothing. A conclusion line carries a glyph or is an aligned field.

## `--quiet`

The output axis, and nothing else. It replaces the command's writers with `io.Discard`, so every framed conclusion, notice and progress line goes nowhere — while the error and the exit code still arrive, because `Execute` prints those to `os.Stderr` rather than through the command.

It never touches a machine contract: `--output json` still emits its document, and a command whose stdout **is** the answer declares `domain.AnnotationMachineOutput` and is left alone. Asking for less noise is not asking for less answer.

It is orthogonal to `--yes`, like the two bypass axes: `--quiet` still asks, `--yes` still reports, and a script that wants neither passes both.

## Showing without keeping

A hook that runs for forty seconds has to be visible while it runs — silence reads as a hang — and must not survive in a scrollback nobody rereads. `output.HookView` is the shape: a bounded tail redrawn in place, erased and replaced by one `✓ <hook> (12.4s)` line, with the whole stream written to `<state-dir>/hooks/<phase>-<branch>.log` regardless, and the tail kept on screen when the hook failed.

It applies to a terminal this process may repaint. A pipe, a CI log or `--output json` gets the raw stream, unconditionally.

The seam that makes it possible is worth copying for anything similar: `service/hooks` reports `domain.HookBeat` values through `flow.HookSink` — the raw output *and* the beat of each hook starting and finishing — so the surface decides what to draw and the service formats only the fallback for a caller that installed no reporter.

## Adding a command

1. Pick the form: an act (A) or a readout (B). If it is neither, it is a table or a machine contract.
2. Frame once. Write to the writer the frame hands you.
3. For each block you are about to add, answer the one question. If it does not change what the reader does next, it is a count.
4. Use the glyph vocabulary. If none fits, the line is probably accounting.
5. If stdout is the command's contract, annotate it with `domain.AnnotationMachineOutput`.
