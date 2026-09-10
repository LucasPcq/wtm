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

The corollary is that **everything a run says while it is still running, and keeps, is one block**. A migrated command's status lines and hook phases used to write straight to stderr, which left them the only human output outside the bar; `CLIPresenter.phase` opens that block on its first line and lets whoever writes next close it — every terminal block in the tree, the error path included, opens with its own blank. The conclusion is a second block on stdout, which is what "exactly once" already allows.

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
| `!` | needs attention; the run continues | `output.Warning` |
| `✗` | failed | `output.Error` |
| `›` | in progress — ephemeral only | `output.Loading` |
| `→` | what to do next | `output.NextStep` |

The runes live in `domain` (`GlyphSuccess`, `GlyphAttention`, …), not as literals in `output/`, so a seventh cannot be introduced by typing one.

### Three rules that make the vocabulary hold

The table above was already written, and the surface diverged anyway — because it fixes the rune and says nothing about the rest of the row. These are the parts that were missing.

**1. The glyph carries the only colour on its line.** The message beside it stays in the terminal's own foreground. Green, yellow and red are a margin of signals down the left of a block, not a property of the text: a reader scans the margin and reads the words. Two registers are the exception, and for one reason — `=` and `›` mute their line **whole**, because there the line itself is the non-event.

Which kills `output.Danger`, and with it the third failure register. `!` is something left to do, `✗` is a failure; a refusal and a crash are the same register, and which of the two it was belongs in the sentence. The old boundary was decided file by file — `sync` and `relocate` called a blockage `Danger`, `fast-forward` called the same idea `Warning`.

**2. Every glyph is one column.** `!` used to render as a filled chip carrying its own padding, so an attention line sat two columns wider — and read louder — than the failure line under it. `internal/output/env.go` had already left the vocabulary over this, rendering a bare `!` because the badge "made the rows wander a column apart". Badges belong to the TUI, where a chip is a widget; a line of CLI output is text.

**3. `Muted` has exactly two jobs, and detail is not one of them.**

- **Chrome**: what is never content — a field's label, a table's header row, a tree's connectors, the note glossing a `NextStep` command.
- **A non-event, whole**: the `=` and `›` lines above.

Secondary detail is expressed by **indentation, not by colour**. A branch list under a count, the lines of a failure's captured output, an address under a job: they are content, they sit one indent in, and they keep the foreground. Muting them was the third job, and it is the one that made the same class of information read at three different densities depending on the command.

These three are checked by `make lint` (`tools/archlint`, rules `glyph`, `tuistyle`, `mutedline`) over `output/`, `styles/` and `tui/` — the layers that put glyphs on a screen. What a linter cannot check it cannot hold, and the first version of this document proved that a table alone does not survive sixty commands.

### What follows from the three rules

**"Nothing to do" is `=`, everywhere** — and "everywhere" includes the places that are not a conclusion. An **empty inventory** is a non-event: `output.UnchangedLine` is `Unchanged` for a formatter that returns a body, so an empty table takes the same glyph as a command that found nothing to do. So does **backing out**: an abort changed nothing, and it is `=` with one wording (`domain.AbortedMessage`) rather than a bare sentence in four.

A **state readout** may not hide a non-event as a field value either. `not running` and `not installed` are the `=` register; a `Section` line is where the detail goes, under a conclusion, never instead of one.

**A conclusion is not optional.** Every human command ends on exactly one, in one of the four shapes of the section above. A readout with no line over it makes the reader infer the outcome from a field.

**A hint is `output.NextStep`, everywhere**: one arrow, one bold command, an optional muted note. A reader learns once where to look for what to do next. Prose telling someone what to run — backticked in a `Message`, muted in a box, inline after a `›` — is the same information in a place nobody looks twice.

**The four block helpers, arbitrated.** They overlapped for as long as nothing said which was which, so two sibling readouts ended up aligned two different ways and two sibling previews titled two different ways.

| Helper | Shape | For |
| -- | -- | -- |
| `SectionTitle` | the bold title alone | a caller that draws its own body — a table, a stream, glyphed lines |
| `Announce` | title + `label  value` rows, labels aligned and muted | anything a reader looks *up*: a plan before a picker, a state readout |
| `Section` | title + indented free lines | a script, a file's contents, a listing |
| `Callout` | a bordered box | **only** something the reader still has to act on |

`flow.Notice` carries that last distinction across the seam: `NoticeNote` is what the reader has nothing to do about — a property of the machine, or of the file that was just written — and takes `Section`; a warning carrying lines is what wtm declined to do, and keeps the border. The port pass is both at once: the links it left alone are bordered, `Addresses carry the proxy's port` is not.

The alignment belongs to `Announce`, never to the wording: a format string spelling `"State      %s"` hand-aligns one block against nothing, and its sibling three files away picks a different column.

**A `--dry-run` answers on stdout.** A preview is what the caller asked for, so it is the result and not a diagnostic. A plan shown *before* a real run — `sync`'s — is a preamble and stays on stderr.

**"Exactly once" counts uninterrupted blocks, not frames.** A command frames each block of human output once; a prompt between two blocks makes two, because there are two blocks. So does a split across streams — `run down`'s failures on stderr and its recap on stdout. What the rule forbids is a second frame around the same block, or a helper emitting its own padding inside one.

**A diff is not a register.** `wtm env` prints `+` / `!` / `−` per key, and that is deliberate: those runes describe a *change to a line of a file*, not the state of a run, and they read as a column down the left of a file block rather than as the head of a conclusion. It is the one vocabulary outside the table, it is confined to `output/env.go`, and adding a second one is a decision to argue for here first.

**The status palette names states, never identities.** `run logs` used to cycle green and yellow across job prefixes, so in the one command whose body is job output, yellow meant "job 3". A label saying where a line came from is chrome.

**`output.Message` — the bare line, carrying no status — is not for a conclusion.** It is the most-called helper in the tree, and that is the symptom it names: when nothing in the vocabulary fits, people fall back to a line that says nothing. A conclusion line carries a glyph or is an aligned field.

## `--quiet`

The output axis, and nothing else. It replaces the command's writers with `io.Discard`, so every framed conclusion, notice and progress line goes nowhere — while the error and the exit code still arrive, because `Execute` prints those to `os.Stderr` rather than through the command.

It never touches a machine contract: `--output json` still emits its document, and a command whose stdout **is** the answer declares `domain.AnnotationMachineOutput` and is left alone. Asking for less noise is not asking for less answer.

The corollary is easy to lose. `domain.ErrAborted` means *the command already printed its own report*, and that stops being true the moment the report went to `io.Discard`: a run that exits non-zero having written nothing to either stream cannot be told from one that hung. So `--quiet` records that it silenced the writers, `Execute` prints the error even for `ErrAborted` when it did, and a site returning that sentinel over a refusal wraps its cause (`fmt.Errorf("%w: %s", domain.ErrAborted, …)`) so there is something to print. The same rule reaches the hook runner: with no reporter installed nobody has drawn the hook's result line, so `service/hooks` names the failing command in the error rather than leaving it anonymous.

It is orthogonal to `--yes`, like the two bypass axes: `--quiet` still asks, `--yes` still reports, and a script that wants neither passes both.

## Showing without keeping

A hook that runs for forty seconds has to be visible while it runs — silence reads as a hang — and must not survive in a scrollback nobody rereads. `output.HookView` is the shape: a bounded tail redrawn in place, erased and replaced by one `✓ <hook> (12.4s)` line, and the tail kept on screen when the hook failed.

It applies to a terminal this process may repaint. A pipe, a CI log or `--output json` gets the raw stream, unconditionally.

Both paths go through one function, `commands/shared.DrawHookPhase`, and it is one function on purpose: the two callers — the migrated commands through `CLIPresenter`, `extract` and `checkout` through `RunCreateHooksPhase` — drifted apart once, and a hook has to read the same whichever command ran it. It owns the log rather than the view, opening `<state-dir>/hooks/<phase>-<branch>.log` and teeing the raw stream into it on **every** path: the run whose output the reader could not watch is exactly the one whose record has to survive. And it always hands the sink a real writer — the command's own — because a sink left nil falls back to `os.Stderr` in the runner, which is how a hook finds its way onto a terminal that asked for `--quiet`.

A hook's own bytes are never barred, for the same reason progress is not: `barWriter` re-marks the row after every carriage return, so a bar drawn over a redrawing progress line lands on top of its content. The rule reaches the run module too — `output.RunPrinter` bars the lines it composes and writes a job's chunks through untouched.

What the phase *keeps* is barred, and that is the whole of the distinction: `HookView` composes every line it prints — the tail included — so those go through the bar, while the cursor moves of `clear()` go to the raw stream. A bar written before one lands on the row the cursor is about to leave, survives the erase below it, and leaves every repaint one column off. `HookViewParams.Bar` says which of the two a caller is: a migrated command opens its mid-run block and sets it; `extract` and `checkout` have no block yet, and a bar with no frame around it is half a block.

The seam that makes it possible is worth copying for anything similar: `service/hooks` reports `domain.HookBeat` values through `flow.HookSink` — the raw output *and* the beat of each hook starting and finishing — so the surface decides what to draw and the service formats only the fallback for a caller that installed no reporter.

## Adding a command

1. Pick the form: an act (A) or a readout (B). If it is neither, it is a table or a machine contract.
2. Frame once. Write to the writer the frame hands you.
3. For each block you are about to add, answer the one question. If it does not change what the reader does next, it is a count.
4. Use the glyph vocabulary. If none fits, the line is probably accounting.
5. If stdout is the command's contract, annotate it with `domain.AnnotationMachineOutput`.
