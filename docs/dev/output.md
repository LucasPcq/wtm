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

The corollary is that **everything a run says while it is still running, and keeps, is one block**. A migrated command's status lines and hook phases used to write straight to stderr, which left them the only human output outside the bar; `shared.OpenBlock` puts them inside one, and the conclusion is a second block on stdout — which is what "exactly once" already allows.

**A surface remembers where its last block left the cursor**, and that is why the bookkeeping lives in `render` rather than in the presenter. The blank closing a block and the blank opening the next are the same line on screen, so a caller deciding whether to open one cannot answer from what it did itself: the frame beside it is written by code that never sees it — `run up`'s own frame around a job's output is the case that made this necessary. `FrameStart` therefore writes no blank on a surface already at a boundary, writes the separator on one whose block is still open, and `BlockOpen` is what `OpenBlock` and `syncPresenter.section` both read. stdout and stderr are **one** surface when both are the same terminal: the reader sees one column of blocks, whichever stream wrote them.

The consequence for a flow: **never report from inside a `Stage`**. A spinner owns the stream while it runs, so a line written under it is repainted over — and the block it opened is then marked open with nothing on screen to show for it. Collect what happened and report it after the stage returns (`internal/flow/run/up/up.go`, `clearOthers`).

## The frame

Every human conclusion is framed **exactly once**, with `render.Frame` or — for a command writing across two streams — the `FrameStart`/`FrameEnd` pair. The frame owns two things at once: the single blank line above and below the block, and the accent bar down its left edge.

```go
render.Frame(cmd.OutOrStdout(), func(w io.Writer) {
    render.Success(w, "Created worktree feat/x")
})
```

The body writes to the writer it is **handed**, never to the one `Frame` was given. That is what puts the bar on every line, in the one place the padding is already applied. A formatter therefore emits a raw body: no leading blank, no trailing blank, `render.Blank` only as a genuine separator between sections inside the block.

A streaming pair wraps its own body writer: `render.Barred(w)`. When a command writes across two streams — `sync`'s plan on stderr, its recap on stdout — there is one rule rather than two mechanisms: **every section opens with exactly one blank line on the stream it is about to write to**, the first of them being the frame's leading blank, and `FrameEnd` closes. Same call, same output, one mechanism.

JSON (`--output json`) and machine output (shell-eval: `resolve` success, `shell-init`, `run url`, `run export`) are never framed and therefore never barred. They emit flush. A command routes on `rules.IsHumanFormat(format)`. `wtm events` is the one stream with no frame at all: it never ends, so there is no block to close, and each human line carries the bar on its own.

### The bar

`┃`, in column zero — left of everything else the CLI prints, which is what makes it a marker rather than one more indent. In a terminal running `git`, `pnpm` and `docker`, it says *this block is wtm speaking*.

It goes on a **terminal only** (`render.IsTerminal`). A pipe, a CI log or a redirection gets the bare text, so `wtm create | tee log` stays clean and a grep over that log never has to know about the bar.

## The four levels, and what each is for

A visual system holds by its contrasts, not by its repetitions. If everything is marked, nothing is.

| Level | For | Where |
| -- | -- | -- |
| **A flat line** | an act you just performed | `create`, `clean`, `checkout`, `run job add` |
| **A table** | an inventory you consult | `list`, `tree`, `run ps`, `run list` |
| **A pill-titled block** (`styles.RenderRecap`) | a state you come back to | `init`, `run up`, `run down` |
| **A callout** (`render.Callout`, bordered) | something still to act on | port isolation, proxy hints, withheld bindings |

The pill is the contrast element and stays rare. A one-line conclusion in a box is five lines of chrome around one line of content — that is the reductio, and it is why the box is not the standard.

## The two shapes of a conclusion

**Form A — the act.** One subject:

```
┃  ✓ Created worktree feat/x
┃
┃  from  main
┃  env   main · 4 ports settled (offset +10)
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

`rules.Tally` counts, zero counts dropped (the CLI and the dashboard share it); then **one line per exception only**, never per success. Budget: 6 lines plus the exceptions.

One nuance that is not a matter of taste: a **destructive** run names what it destroyed — knowing what is gone is actionable — but on one line, because the picker and the recap have already shown that list twice. A non-destructive run counts.

### A run's addresses

A run is the one conclusion that lists addresses, and it does it once: each job line carries a single fragment (`rules.ReachSummary` — the URL, `:5432`, `3 urls`, `6 ports`), and the full list is the **Where to reach it** block (`rules.ReachBlock`) the run ends on, the run view shows behind `a`, and its recap keeps. A port list on a job line is how `docker-compose` came to take 160 columns; see [run-addressing.md](run-addressing.md#where-to-reach-it--one-model-for-every-surface).

## The glyph vocabulary

Exhaustive. One glyph per line, at its head; never two vocabularies in one block.

| Glyph | Means | Helper |
| -- | -- | -- |
| `✓` | changed state, and it worked | `render.Success` |
| `=` | was already in the desired state | `render.Unchanged` |
| `~` | an existing thing was replaced | `render.Update` |
| `!` | needs attention; the run continues | `render.Warning` |
| `✗` | failed | `render.Error` |
| `›` | in progress — ephemeral only | `render.Loading` |
| `→` | what to do next | `render.NextStep` |

The runes live in `domain` (`GlyphSuccess`, `GlyphAttention`, …), not as literals in `surface/cli/render/`, so a seventh cannot be introduced by typing one.

### Three rules that make the vocabulary hold

The table above was already written, and the surface diverged anyway — because it fixes the rune and says nothing about the rest of the row. These are the parts that were missing.

**1. The glyph carries the only colour on its line.** The message beside it stays in the terminal's own foreground. Green, yellow and red are a margin of signals down the left of a block, not a property of the text: a reader scans the margin and reads the words. Two registers are the exception, and for one reason — `=` and `›` mute their line **whole**, because there the line itself is the non-event.

Which kills `render.Danger`, and with it the third failure register. `!` is something left to do, `✗` is a failure; a refusal and a crash are the same register, and which of the two it was belongs in the sentence. The old boundary was decided file by file — `sync` and `relocate` called a blockage `Danger`, `fast-forward` called the same idea `Warning`.

**2. Every glyph is one column.** `!` used to render as a filled chip carrying its own padding, so an attention line sat two columns wider — and read louder — than the failure line under it. `internal/surface/cli/render/env.go` had already left the vocabulary over this, rendering a bare `!` because the badge "made the rows wander a column apart". Badges belong to the TUI, where a chip is a widget; a line of CLI output is text. One column is also a property of the **font**, not only of the rune: a glyph the terminal's font lacks is drawn from a fallback face, often wider, and overflows onto the space after it. `↻` did exactly that under JetBrains Mono (Ghostty's default), which is why the update glyph is `~`. `make lint` holds this through `archlint`'s `fontcover` rule: a non-letter rune in any string of `internal/` must belong to `fontSafe`, measured as present in thirteen common monospace fonts (box drawing and block elements are exempt, terminals draw those themselves). The runes that predated the rule — `▸`, `⚠`, `●`… — are listed in `fontLegacy`, report as migrating, and that list may only shrink.

**3. `Muted` has exactly two jobs, and detail is not one of them.**

- **Chrome**: what is never content — a field's label, a table's header row, a tree's connectors, the note glossing a `NextStep` command.
- **A non-event, whole**: the `=` and `›` lines above.

Secondary detail is expressed by **indentation, not by colour**. A branch list under a count, the lines of a failure's captured output, an address under a job: they are content, they sit one indent in, and they keep the foreground. Muting them was the third job, and it is the one that made the same class of information read at three different densities depending on the command.

These three are checked by `make lint` (`tools/archlint`, rules `glyph`, `tuistyle`, `mutedline` — see [lint.md](lint.md#the-output-vocabulary)) over `surface/cli/render/`, `styles/` and `surface/tui/` — the layers that put glyphs on a screen. What a linter cannot check it cannot hold, and the first version of this document proved that a table alone does not survive sixty commands.

### What follows from the three rules

**"Nothing to do" is `=`, everywhere** — and "everywhere" includes the places that are not a conclusion. An **empty inventory** is a non-event: `render.UnchangedLine` is `Unchanged` for a formatter that returns a body, so an empty table takes the same glyph as a command that found nothing to do. So does **backing out**: an abort changed nothing, and it is `=` with one wording (`domain.AbortedMessage`) rather than a bare sentence in four. It still exits `19` (`ExitCodeCancelled`): `CLIPresenter.Notice` marks the command when it draws that line, and the root ends the process on the mark, so a shell chaining `wtm create x && wtm go x` stops there while the dashboard, which never reads an exit code, is left alone.

A **state readout** may not hide a non-event as a field value either. `not running` and `not installed` are the `=` register; a `Section` line is where the detail goes, under a conclusion, never instead of one.

**A conclusion is not optional.** Every human command ends on exactly one, in one of the four shapes of the section above. A readout with no line over it makes the reader infer the outcome from a field.

**A hint is `render.NextStep`, everywhere**: one arrow, one bold command, an optional muted note. A reader learns once where to look for what to do next. Prose telling someone what to run — backticked in a `Message`, muted in a box, inline after a `›` — is the same information in a place nobody looks twice.

**The four block helpers, arbitrated.** They overlapped for as long as nothing said which was which, so two sibling readouts ended up aligned two different ways and two sibling previews titled two different ways.

| Helper | Shape | For |
| -- | -- | -- |
| `SectionTitle` | the bold title alone | a caller that draws its own body — a table, a stream, glyphed lines |
| `Announce` | title + `label  value` rows, labels aligned and muted | anything a reader looks *up*: a plan before a picker, a state readout |
| `Section` | title + indented free lines | a script, a file's contents, a listing |
| `Callout` | a bordered box | **only** something the reader still has to act on |

`flow.Notice` carries that last distinction across the seam: `NoticeNote` is what the reader has nothing to do about — a property of the machine, or of the file that was just written — and takes `Section`; a warning carrying lines is what wtm declined to do, and keeps the border. The port pass is both at once: the links it left alone are bordered, `Addresses carry the proxy's port` is not — and that one is said by `wtm env` and `wtm run addressing`, whose subject it is, never by a creation (`rules.EnvPortNoticesOnCreate`).

The alignment belongs to `Announce`, never to the wording: a format string spelling `"State      %s"` hand-aligns one block against nothing, and its sibling three files away picks a different column.

**A `--dry-run` answers on stdout.** A preview is what the caller asked for, so it is the result and not a diagnostic. A plan shown *before* a real run — `sync`'s — is a preamble and stays on stderr.

**"Exactly once" counts uninterrupted blocks, not frames.** A command frames each block of human output once; a prompt between two blocks makes two, because there are two blocks. So does a split across streams — `run down`'s failures on stderr and its recap on stdout. What the rule forbids is a second frame around the same block, or a helper emitting its own padding inside one.

**A diff is not a register.** `wtm env` prints `+` / `!` / `−` per key — every key under `--check`, and under an apply only what it left for the reader, what it did being one counted line per file (`rules.EnvFileTally`) — and that is deliberate: those runes describe a *change to a line of a file*, not the state of a run, and they read as a column down the left of a file block rather than as the head of a conclusion. It is the one vocabulary outside the table, it is confined to `surface/cli/render/env.go`, and adding a second one is a decision to argue for here first.

**The status palette names states, never identities.** `run logs` used to cycle green and yellow across job prefixes, so in the one command whose body is job output, yellow meant "job 3". A label saying where a line came from is chrome.

**`render.Message` — the bare line, carrying no status — is not for a conclusion.** It is the most-called helper in the tree, and that is the symptom it names: when nothing in the vocabulary fits, people fall back to a line that says nothing. A conclusion line carries a glyph or is an aligned field.

## `--quiet`

The output axis, and nothing else. It replaces the command's writers with `io.Discard`, so every framed conclusion, notice and progress line goes nowhere — while the error and the exit code still arrive, because `Execute` prints those to `os.Stderr` rather than through the command.

It never touches a machine contract: `--output json` still emits its document, and a command whose stdout **is** the answer declares `domain.AnnotationMachineOutput` and is left alone. Asking for less noise is not asking for less answer.

The corollary is easy to lose. `domain.ErrAborted` means *the command already printed its own report*, and that stops being true the moment the report went to `io.Discard`: a run that exits non-zero having written nothing to either stream cannot be told from one that hung. So `--quiet` records that it silenced the writers, `Execute` prints the error even for `ErrAborted` when it did, and a site returning that sentinel over a refusal wraps its cause (`fmt.Errorf("%w: %s", domain.ErrAborted, …)`) so there is something to print. The same rule reaches the hook runner: with no reporter installed nobody has drawn the hook's result line, so `service/hooks` names the failing command in the error rather than leaving it anonymous.

It is orthogonal to `--yes`, like the two bypass axes: `--quiet` still asks, `--yes` still reports, and a script that wants neither passes both.

## The machine contract

`--output json` is read by programs, so its shape is decided once and never follows what happened. Four rules hold for the `run` module, and a new document follows them rather than its neighbour:

- **One shape per command.** A command that acts on worktrees answers with an array of per-worktree documents even for one worktree (`run up`, `run down`, `run stop`, `run logs`); a single-subject command answers with one object (`run start`, `run job|profile add|edit|rm`). A shape that changed with the arity made every caller branch on how many worktrees it had named.
- **A worktree is `branch` + `path`**, both, always — never `worktree` or `work_dir`. `domain.WorktreeRef` is the type when nothing else rides along. A job object is keyed `name`; anything pointing at a job from another object calls it `job`.
- **`status` never claims an act that did not happen.** A stop that found nothing up is `not_running`, never `stopped`; a start that found the service already up is `already_running`, never `started`; a shared job let go of is `released`.
- **Exit codes are part of the document.** `rules.ExitCode` maps the sentinels: `2` for a command line refused before the command ran (`cmd/usage.go` wraps cobra's flag and argument errors, an unknown `--output`, and `--output json` without `--yes` on a command marked by `shared.RequireYesInJSON`, in `domain.ErrUsage`; a refusal a command makes itself — a positional that does not parse, `--all` with a name — goes through `rules.Usage`, and belongs in the command's `Args` when it reads only the arguments), `14` for a job or profile run.toml does not declare (`ErrJobNotFound`, `ErrProfileNotFound`), checked by `target.RequireDeclared` before a flow asks anything or wakes the daemon.

A document that is a protocol elsewhere is not reused for output: `domain.JobInfo` is what the daemon speaks, so `run ps` writes `domain.RunningJob`, and renaming a key there never needs a daemon restart. `run list`, `run export` and `run import` are the exception to the naming rule on purpose — they are run.toml as JSON, and keep its keys (`job`, `profile`, `env_port`).

## Showing without keeping

A hook that runs for forty seconds has to be visible while it runs — silence reads as a hang — and must not survive in a scrollback nobody rereads. `render.HookView` is the shape: a bounded tail redrawn in place, erased and replaced by one `✓ <hook> (12.4s)` line, and the tail kept on screen when the hook failed.

It applies to a terminal this process may repaint. A pipe, a CI log or `--output json` gets the raw stream, unconditionally.

Both paths go through one function, `surface/cli/shared.DrawHookPhase`, and it is one function on purpose: two paths to it — the migrated commands through `CLIPresenter`, `extract` and `checkout` through a helper of their own, since gone — drifted apart once, and a hook has to read the same whichever command ran it. It owns the log rather than the view, opening `<state-dir>/hooks/<phase>-<branch>.log` and teeing the raw stream into it on **every** path: the run whose output the reader could not watch is exactly the one whose record has to survive. And it always hands the sink a real writer — the command's own — because a sink left nil falls back to `os.Stderr` in the runner, which is how a hook finds its way onto a terminal that asked for `--quiet`.

A hook's own bytes are never barred, for the same reason progress is not: `barWriter` re-marks the row after every carriage return, so a bar drawn over a redrawing progress line lands on top of its content. The rule reaches the run module too — `render.RunPrinter` bars the lines it composes and writes a job's chunks through untouched.

What the phase *keeps* is barred, and that is the whole of the distinction: `HookView` composes every line it prints — the tail included — so those go through the bar, while the cursor moves of `clear()` go to the raw stream. A bar written before one lands on the row the cursor is about to leave, survives the erase below it, and leaves every repaint one column off. `HookViewParams.Bar` is what says which writer a line takes. `DrawHookPhase` joins the run's block itself rather than leaving that to each caller: `extract` holds a presenter that may already have opened one for the port pass, and a phase that decided for itself drew an unbarred block beside a barred one.

The seam that makes it possible is worth copying for anything similar: `service/hooks` reports `domain.HookBeat` values through `flow.HookSink` — the raw output *and* the beat of each hook starting and finishing — so the surface decides what to draw and the service formats only the fallback for a caller that installed no reporter.

## Addresses are clickable

Every job address a person reads can be followed with a click, and there are two mechanisms because there are two kinds of surface.

- **Text a command prints** wraps each address in an OSC-8 link with `rules.LinkURLs`, on a terminal only (`render.IsTerminal`). A pipe, a CI log, `--output json` and machine output (`run url`, meant for `$(…)`) never receive the escape. Link **after** padding or truncating: the escape has no width, and a column measured in bytes or runes would push everything after it. `run ps` pads its ADDRESS column first, then links it.
- **A full-screen surface holds the mouse**, so the terminal never sees a plain click on a link. The run view and the dashboard open the address under a click themselves with `components.URLAt`, which reads it off the frame they last drew, so an address is clickable wherever it lands without declaring a zone. A truncated address (ending in `…`) is never followed. The run view also draws OSC-8 links (Params.Hyperlinks) for the terminal's modifier-click; the dashboard does not, because bubblezone measures the escape as text and would shift every zone on that row.

## Adding a command

1. Pick the form: an act (A) or a readout (B). If it is neither, it is a table or a machine contract.
2. Frame once. Write to the writer the frame hands you.
3. For each block you are about to add, answer the one question. If it does not change what the reader does next, it is a count.
4. Use the glyph vocabulary. If none fits, the line is probably accounting.
5. If stdout is the command's contract, annotate it with `domain.AnnotationMachineOutput`.
