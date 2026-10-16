# Architecture — the layers and what they buy

wtm is a Cobra CLI with interactive surfaces (an inline wizard, the form of a command on the engine, the `wtm ui` dashboard) over one set of git operations. The layering exists so that a command is written once and served by every surface. On the engine, a command is a `kernel.Command` in `command/`, assembled by `registry/` and reached through `dispatch/` ([commands.md](commands.md), [dispatch.md](dispatch.md)); `create` is there, the first. Every other command is a *flow* in `flow/` — the order of its questions, its safety checks, its service calls — replayed by the wizard and the dashboard ([flow-layer.md](flow-layer.md)).

## The map

One line per package: what it owns. The import rules between them are the next section.

```
main.go                       ← the entry point: cli.Execute
internal/
  domain/                     ← types, errors, constants only (no methods, no functions); its root only shrinks,
                                new code goes into the sub-package of its subject
    statefile/                ←   the schema version of each state file, and the error of a file a newer wtm wrote
    operation/                ←   the holder file of an operation in progress, which dispatch writes and query/operations lists
  rules/                      ← pure functions (stdlib + domain only, no I/O); its root only shrinks, new code goes
                                into the sub-package of its subject
    statefile/                ←   what a state file's schema allows: read, write, or refuse (exit 22)
    branch/                   ←   where a branch stands against origin, and whether its parent can be inferred
    envcopy/                  ←   where each .env of a new worktree is copied from, and what could not be
  kernel/                     ← the skeleton of the command engine (stdlib only, no business vocabulary): Command,
                                fields, rules, errors, results, Each / Saga (commands.md)
    text/                     ←   text.Messages and text.Catalog: a kernel.Code and its params into English, the
                                kernel's messages and those each entry of the registry declares
    kerneltest/               ←   the checks every command runs on itself (DependsOn, sagas)
  dispatch/                   ← the impure half of the engine, the one way a surface reaches a command: registry,
                                the form's calls, the run (locks → Observe → defaults → Validate → Plan →
                                fingerprint → Apply), operation locks and holders, hook logs, operation events
                                (dispatch.md)
  command/                    ← the mutations on the engine, one package per command, run through dispatch only
                                (adding-a-mutation-command.md)
    create/                   ←   `wtm create`: a worktree per branch asked, from one source; its source fast-forwarded
                                  first when asked, each branch a provision unit, the answers kept once one is created
    provision/                ←   the unit that gives a branch its worktree (saga: worktree, .env, meta.json;
                                  then ordinal, ports, on_create), shared by the commands that create one, never registered
  query/
    operations/               ←   the operations in progress, read from their holder files
  registry/                   ← the composition root: every command and query with its real ports (registry.Production),
                                the only importer of command/ and query/, imported by surface/cli (dispatch.md)
  config/                     ← load & validate config.toml + run.toml from <git-common-dir>/wtm/, plus the global config (config.GlobalPath);
                                every write puts the file's JSON schema (schemas/) beside it; ReviseProject is
                                config.toml with a change applied, for store/ to write
  store/                      ← the one writer of each state file (store.md): Meta (meta.json: Load, Records,
                                Update, Purge), ProjectConfig (config.toml: Remember, ConfigDisk.Update),
                                EnvFiles (the .env files: Copy, Edit), each on disk and doubled in testutil/storetest
  flow/                       ← the flow of each command not yet on the engine, surface-independent
                                (flow-layer.md): the vocabulary (Step, Session, Prompter, Presenter, Publisher)
    publish/                  ←   the event a flow publishes after a change to a worktree's
                                  identity, read back from git and meta.json (`wtm events`)
    ordinal/                  ←   allocating a worktree's ordinal from the flow that needs it,
                                  published as `worktree.updated`; the service only reads it
    decide/                   ←   branch/env decisions shared by the create-like flows
    envports/                 ←   settling a fresh .env's host ports onto the ones the
                                  worktree binds, per its isolation (isolated / verbatim,
                                  recorded in meta.json and read by the daemon too) —
                                  shared by `create`, `extract` and `checkout`, which it
                                  never fails: a refused run.toml or an unreadable ordinal
                                  is a warning (`SettleFresh`), the run part left undone
    create/                   ←   `wtm create`: the run (create.go) + its questions (steps.go)
    checkout/                 ←   `wtm checkout`: the run (checkout.go) + its questions (steps.go)
    clean/                    ←   `wtm clean`: the run (clean.go) + its questions (steps.go)
    reparent/                 ←   `wtm reparent`: the run (reparent.go) + its questions (steps.go)
    prune/                    ←   `wtm prune`: the run (prune.go) + its questions (steps.go)
    extract/                  ←   `wtm extract`: the run (extract.go) + its questions (steps.go),
                                  create's own embedded through `create.Embed`
    env/                      ←   `wtm env`: the run (env.go) + its questions (steps.go), the
                                  pre-scan the wizard reads (scan.go), how the worktree runs
                                  today that its isolation and addressing steps keep or switch
                                  (mode.go), and the port pass and isolation switch (pass.go);
                                  its per-key resolver is its own kind, `flow.StepEnvResolve`
    relocate/                 ←   `wtm relocate`: the run (relocate.go) + its questions (steps.go);
                                  the move, the adoption and the base_path rewrite are three
                                  separate service calls (`worktree.Move`/`Adopt`/`SetBasePath`)
    teardown/                 ←   the removal clean and prune share, one worktree or a
                                  batch (`Batch`): stop, hooks, remove, drop — then
                                  release every claim, all together
    orphans/                  ←   the question clean and prune ask about the children a
                                  removal orphans: the step, its preset, its recap line
    sync/                     ←   `wtm sync`: the run (sync.go) + its questions (steps.go)
    fastforward/              ←   `wtm fast-forward`: the run + its questions
    exec/                     ←   `wtm exec`: the run + its questions
    status/                   ←   `wtm status`: a worktree's state read without asking,
                                  numbering or waking anything (no Prompter, no Presenter)
    runlogs/                  ←   the jobs a surface shows (`Board`), their live streams,
                                  and the profile start sequence (reports events, not steps)
    run/                      ←   the `run` module's flows, mirroring its command tree:
      target/                 ←     the questions they share (worktree, job, profile,
                                    and the published-url step `run open` asks)
      urls/                   ←     where every address the module hands out is computed
      seam/                   ←     the daemon as a flow uses it: board, env, log dir,
                                    port prober, and the start sequence a surface drives
      foreigndata/            ←     the stop before a job whose `touches` reach data the
                                    worktree does not own, shared by `up` and `start`
      probes/                 ←     the offer to write `probe = false` for a job bound to its
                                    base port, made after `up` and `start` alike
      owed/                   ←     paying the namespace drops a clean deferred, whenever a run
                                    finds their shared service up
      addressing/             ←     `run addressing`: switch the mode, settle the worktrees' .env
      concurrency/            ←     the question about the other worktrees' jobs (load or
                                    port clash, `--exclusive`/`--parallel`), shared by `up` and `start`
      up/ down/ start/        ←     one package per command, as everywhere else
      stop/ logs/ open/ url/
      list/                   ←     `run list`: which entry was picked and what to do to it
      job/ profile/           ←     CRUD on run.toml's declarations, one package per group
      initrun/                ←     `run init`: detect, ask (the services wizard is its own
                                    `Wizard` seam, not a flow.Session), write run.toml,
                                    compose and .env files
  service/                    ← impure orchestration only (git exec, I/O, hooks):
    worktree/                 ←   git worktree operations (create, list, remove), for the commands on flow/
    gitwt/                    ←   a git worktree's lifecycle for the engine (services.md): Add and Remove,
                                  the Do and the Undo of a saga's worktree step
    identity/                 ←   who a worktree is (services.md): one Snapshot (number, isolation, main,
                                  derived names, where a branch is out) read once; Allocate the one mutator
    runenv/                   ←   a worktree's run environment (services.md): JobEnv, ComposeProject, Env,
                                  from the run.toml and the identity the caller read
    env/                      ←   .env provisioning (create) + drift reconciliation (`wtm env`, sync.go)
    hooks/                    ←   on_create / on_clean hook execution (a /bin/sh line each): RunHooks for
                                  flow/, Execute for the engine (each line to its caller, nothing printed)
    shell/                    ←   shell integration generation (zsh, bash, fish)
    integration/              ←   third-party adapters: handing a URL to the desktop's
                                  own opener (editor/agent detection lives in detect/)
    proxy/                    ←   the run proxy: the host→job routing table and the
                                  loopback server the daemon owns (`[proxy]`)
    detect/                   ←   auto-detection (base branch, env files, package manager)
    branch/                   ←   branch candidates for the pickers (local + origin, divergence)
    github/                   ←   pull requests through the `gh` CLI
    selfupdate/               ←   how wtm was installed, and `wtm upgrade`
    process/                  ←   the run daemon: jobs on PTYs, the durable index (jobs.json),
                                  reaping orphans, the client the commands talk through, and
                                  the schema-blind event broker (`publish` / `subscribe`)
    events/                   ←   the `wtm events` bus as wtm uses it: the Publisher every flow
                                  reports through, Watch (subscribe → snapshot → ready), the
                                  registry of repositories wtm was used in (`repos.json`, through
                                  `infra/registry.go`) and WatchAll, which follows all of them
    runconfig/                ←   load + validate + write run.toml (and its schema)
    runjobs/                  ←   the daemon's jobs as a surface reads them (the dashboard too);
                                  `Current` reads the index itself when no daemon listens
    compose/                  ←   a compose file's `ports:` and absolute names, read and rewritten
    portprobe/                ←   is anything listening on a port
    shellcmd/                 ←   checks that a config command is a valid /bin/sh line
    execsvc/                  ←   runs one shell line in several worktrees at once
    memory/                   ←   the answers the wizard remembers ([wizard.remembered] in config.toml), through store/
  styles/                     ← all Lipgloss styles (only package allowed to instantiate lipgloss.Style), and what every surface draws alike: an intro, a recap, the rows of a plan (RenderPlan)
  infra/                      ← the git functions of the commands still on flow/, and the state not yet
                                  moved to a package of its own (repos.json)
    git/                      ←   the one way the engine reaches git (git.md): Repo (a directory and its
                                  remote, -C everywhere), Reader (Worktrees, MainWorktree, Branches,
                                  RefExists, AheadBehind) and Writer (AddWorktree, RemoveWorktree,
                                  DeleteBranch, Fetch, FetchRef, FastForwardRef, MergeFFOnly); no decision
    fs/                       ←   the only writer of the files wtm keeps: UpdateFile (lock on <path>.lock, read,
                                  Mutate, temp file + fsync + rename), ReplaceFile for the daemon's index,
                                  WithFileLock, GlobalDir; Lock.Held says when flock was unavailable;
                                  TryLocks (a set, all or none, never waiting), HoldFile / HeldFiles
                                  (a file held as long as its process runs), OpenLog
    proc/                     ←   starting processes (Command, GroupCommand) and settling them on an
                                  interrupt (Shield, AwaitShields, KillCancelled)
  surface/                    ← the interfaces
    cli/                      ←   the command line (zero business logic, cli.md): the root, its signals,
                                  --quiet and exit code; the commands generated from the dispatch
                                  registry (cli.Bind); the flag wiring of the commands still on flow/
      run/runctx/             ←     what every `run` command opens on: its directory, the config,
                                    run.toml, the opt-in guard and the prompt gate
      daemon/                 ←     the hidden `daemon` command and the macOS port-80 relay launchd runs
      ui/                     ←     `wtm ui`: refuses JSON and a missing TTY, then hands off to tui/dashboard
      events/                 ←     `wtm events`: the stream (text or JSON Lines) over service/events.Watch,
                                    or WatchAll outside any repository
      versioncmd/             ←     `wtm version`: the binary's version and each machine contract's (`events`)
      render/                 ←     format and print results (zero decision logic); for a command on the
                                    engine, the generic rendering of a kernel result and the v2 envelope
    tui/                      ←   Bubbletea models (zero business logic, rendering only)
      form/                   ←     the interactive form of a command on the engine: a wizard drawn from
                                    the command's schema, state and choices, its recap the plan of a dry
                                    run, reached through dispatch only (form.md)
      flowui/                 ←     runs a flow.Session as a wizard (the only translator
                                    between flow.Step and components.Step)
      dashboard/              ←     `wtm ui`: the full-screen worktree dashboard, the second
                                    surface over flow/ (its own Prompter/Presenter, mouse
                                    zones via bubblezone). It also hands the terminal to
                                    runview (`handoff.go`) for the run flows that draw
      runview/                ←     a job's raw PTY output replayed through a terminal
                                    emulator (`github.com/charmbracelet/x/vt`)
```

## Who may call whom

```mermaid
flowchart TD
  cli["surface/cli/"] --> registry["registry/"]
  cli --> dispatch["dispatch/"]
  cli --> form["surface/tui/form/"]
  cli --> output["surface/cli/render/"]
  cli --> flow["flow/"]
  cli --> tui["surface/tui/"]
  cli --> config["config/"]
  registry --> command["command/"]
  registry --> query["query/"]
  registry --> dispatch
  form --> dispatch
  form --> kernel
  output --> kernel["kernel/"]
  dispatch --> kernel
  dispatch --> infra["infra/"]
  command --> kernel
  command --> engineservices["service/gitwt, identity, runenv, hooks"]
  command --> store["store/"]
  command --> git["infra/git"]
  query --> kernel
  engineservices --> store
  engineservices --> git
  tui --> flow
  flow --> service["service/"]
  flow --> rules["rules/"]
  service --> store
  service --> infra
  service --> rules
  flow --> domain
  store --> config
  store --> infra
  rules --> domain["domain/"]
  output --> styles["styles/"]
  tui --> styles
```

Every arrow that is *missing* is the point:

| Interdiction | What it buys |
| -- | -- |
| `surface/cli/` has no business logic | A command is readable as flags in, one call out. Changing the flow never means editing flag parsing. |
| `domain/` holds types, errors and constants only | Nothing can acquire a dependency by hiding behind a method on a shared type. |
| `rules/` imports only stdlib + `domain/` | Decisions stay testable with no repo, no network, no temp dir. `rules.DecidePush` is a table test, not an integration test. |
| `kernel/` imports only the stdlib | The skeleton carries no business vocabulary: a command composes `kernel` with `domain/`, never the reverse, so a change to the domain never reaches the contract every command, `dispatch` and surface share. I/O reaches it only as functions it is handed (`Observe`, `Apply`, the `Shield`). |
| `command/` and `query/` never import `dispatch/` or `surface/`; `surface/` never imports them | A surface reaches a command through `dispatch` only, so the locks, the fingerprint and the events apply whatever the interface; a command never runs another inside its own locks. |
| `registry/` alone imports `command/` and `query/`, and builds their real ports (`testutil/`'s doubles aside); only `surface/cli` imports it | One place says what the product serves and on which disk; `dispatch` stays a generic engine over its own ports, and a test runs a command on doubles without the registry. |
| `service/` never imports cobra, bubbletea or lipgloss | The git operations are callable from a test, a flow, a daemon — anything that is not a terminal. |
| `surface/cli/render/` and `surface/tui/` hold no decision logic | Two surfaces can render the same run without disagreeing about what it means. `render` reads a command's results from `kernel`, never runs one through `dispatch`. The form asks `dispatch` what a field is (`State`, `Choices`) rather than deciding it. |
| `surface/tui/form/` reaches a command through `dispatch` only, never `flow/` or `service/` | One form serves every command of the registry without knowing its types, and asks what any other surface would. |
| `styles/` is the only package instantiating `lipgloss.Style` | A theme change is one file. |
| `flow/` imports only `service/`, `rules/`, `domain/` and the stdlib | The flow cannot grow a dependency on the surface that runs it. This is what makes a second surface possible at all — see below. |

`flow/` cannot reach `infra/` either. When a flow needs something only `infra/` has, the fix is a thin `service/` wrapper, not an exception: `worktree.FindByBranch` and `worktree.ListAll` exist for exactly that reason.

Two more edges are constrained beyond the diagram:

- `service/x` imports `service/y` only along an edge declared in `tools/archlint` (`serviceEdges`): today `detect→branch`, `events→{process,worktree}`, `process→proxy`, `runconfig→shellcmd`, `runjobs→{process,runconfig,worktree}`, `worktree→{branch,env,github,hooks,process}`. `command/x` imports `command/y` only along an edge of `commandEdges`: today `create→provision`.
- `command/` and `query/` reach `domain/`, `rules/`, `kernel/`, `store/`, `config/`, `infra/git`, `infra/fs`, `infra/proc` and the engine's services only; an interface they declare is sealed, never a port (`ports`), they build no error from English text (`enginetext`), and they never build a real port: `store.MetaDisk`, `git.Repo` and the like are built by `registry/` alone (`portbuild`). `registry/` is the only package importing them (with `testutil/`, for the doubles), and `surface/cli` the only one importing `registry/`. `kernel/` and `rules/` import no stdlib package that does I/O (`os`, `net`, `syscall`).
- The daemon — `service/process` and `service/proxy`, which it serves — is blind to git: neither imports `service/worktree`, `service/branch`, `service/github`, `service/events` or `config`, and both call only allow-listed `infra/` functions (`fs.GlobalDir`, `fs.ReplaceFile`).
- git is run by `infra/git` only (`gitexec`); the files at the root of `infra/` that still run it, for the commands on `flow/`, are tracked in `.archlint-migrating`.
- The services a command on the engine reaches (`service/gitwt`, `identity`, `runenv`, `hooks.Execute`, [services.md](services.md)) import no other service, never `kernel/`, and publish nothing: a mutator returns what it changed, and `dispatch` publishes it.
- `store/` imports `config/`, `domain/`, `infra/` and `rules/`; `service/` and `testutil/` reach it, `flow/` never does. An exported function of `service/worktree` that reaches a write of `store.Meta` is a mutator (`metawriter`).
- A file is written through `infra/fs` only (`statewrite`): `os.WriteFile`, `os.Create`, `os.Rename`, `os.Remove`, `os.RemoveAll` and a writing `os.OpenFile` stay inside `infra/`, but for the permanent exceptions listed with their reason in `tools/archlint/statewrite.go`. A state file is written by its owner in `store/` only (`stateowner`): `fs.UpdateFile` and `fs.RemoveDir` are called from nowhere else.
- A function that reaches a write sink is a mutator, inferred from the code (`writesink`): nothing in `query/` nor in a reader of a command (`Observe`, `Choices`, `Plan`…) reaches one, and a mutator of the engine's services is called from `command/` or another service only.
- In the engine, `dispatch` alone publishes, through its `Publisher` (`publish`): `command/`, `query/` and the engine's services call no `Publish`.
- Every package of `service/`, `infra/`, `store/` and of the engine (`kernel/`, `dispatch/`, `command/`, `query/`, `registry/`) says its one responsibility in its package doc (`pkgdoc`).
- The roots of `rules/` and `domain/` only shrink (`rootfreeze`): no new file or exported symbol there; new code goes into the sub-package of its subject, a presentation rule into the surface that uses it.
- A service **mutator** (`worktree.Create`, `envsvc.ApplyEnvSync`, `runconfig.Save`, … — the table in `tools/archlint/chokepoint.go`) is called only from `internal/flow/`, whatever the calling layer; a call inside the mutator's own package is its implementation.

None of this is left to review: `make lint` runs `tools/archlint`, and each rule above is one of its analyzers. The full list, and how to add one, is in [lint.md](lint.md).

## How a command designates a worktree

One rule, no exception: **the subject is positional, and a worktree that is not the subject is a flag named after its role.** `clean [branch]`, `env [worktree]`, `extract [source]`, `sync [branch...]` take their subject positionally; `extract --to`, `create --from`, `sync --base` name a second worktree. The `run` module follows the same rule with the worktree as its subject — `run up [worktree] --profile`, `run start [worktree] --job` — so the job and the profile are flags. A new command adds no third form.

Omitting the positional resolves in one of two ways, and which one is not a matter of taste: **the current directory when it is a safe default for that command, a picker otherwise.** `run` has one (you are standing in the worktree whose services you want), so a non-interactive run silently takes it — category 1 of the bypass model, no exception to write. `clean` has none (which worktree would it destroy?), so it errors or opens a picker — category 2.

Whatever answers, a resolved worktree is always **the worktree root as git spells it** (`infra.Toplevel`), never a raw `os.Getwd()`. The daemon keys a job on `name + WorkDir` by string equality *and* runs it there, resolving `run.toml`'s `cwd` against it: a subdirectory, or macOS's `/var` where git says `/private/var`, splits one worktree into two keys and mis-resolves every relative `cwd`.

## A flow calls the service itself

A TUI package may not import `service/`, and a wizard needs data only the service has: a branch's divergence, a worktree's safety check, a sync plan, the modified files of a source. On `flow/` the step declaration carries that work — `Skip`, `Build` and `Load` are the flow's own functions, which call the service — so the surface that draws the step runs nothing but the step. No command hands a TUI a closure into the service: what the dashboard and the wizard show of a step is what its `Build` or `Load` returned, computed once, on the flow's side of the boundary.

On the engine the same boundary is `dispatch`: the form asks `State` and `Choices` ([form.md](form.md)), and the only I/O of a field is its `Choices`, run by the command.

## The run module — a flow that asks nothing

`internal/flow/runlogs` is the second shape a flow takes. `create` and `clean` ask questions and need a `Prompter`; a run has none to ask — it *reports*. So the seam is made of three types instead:

- **`runlogs.Board`** — the worktree's jobs as a surface reads them: `Jobs()` (a `JobView` per declared or running job), `Refresh()`, `Attach()` for a live `Stream`, and `History()` for what a job left in its log file. A surface never speaks to `service/process`.
- **`runlogs.Stream`** — one attached job: raw chunks in (escape sequences included, an emulator needs them untouched), keystrokes and a PTY resize out.
- **`runlogs.Run(ctx, RunParams)`** — a profile's start sequence, reporting each step to a `Sink` as an `Event`/`Phase`. It returns an `Outcome`, never an error: what a partial state is worth — an exit code, a report, a JSON entry — belongs to the surface. Cancelling `ctx` ends the *reporting*, not the jobs: that is what a detach is.

Three surfaces consume it, chosen by one pure rule (`rules.DecideRunSurface`, which needs a terminal, a human format and no `-d` before it picks the view):

| Surface | Who | What it does with the seam |
| -- | -- | -- |
| `internal/surface/tui/runview` | a terminal | full screen, one VT-emulated pane per job, tmux-style focus; returns its recap for the command to frame |
| `render.RunPrinter` | `-d`, a pipe, CI | renders each `Event` as a line on stdout/stderr |
| `render.WriteRunOutcomeJSON` | `--output json` | the array of job results, with the failing job's `output` and `exit_code` |

Everything a job needs to know about *which* worktree it belongs to is resolved by the client and travels down the seam beside `WorkDir` and `LogDir`: `RunParams.Env` → `StartRequest.Env` → `process.Request.Env` → `cmd.Env`. It cannot be inherited — the daemon is global, outlives the command that forked it, and its own environment belongs to whichever worktree happened to start it. `service/worktree.EnsureOrdinal` is what gives the worktree the stable number those variables derive from, and `service/worktree.JobEnv`/`BranchEnv` assemble them; the daemon keeps the resolved map on the `ManagedJob` so the job's stop command runs in the same environment its start did.

`internal/surface/cli/run/surface.go` is the whole wiring: open the seam, build the starter, switch on the rule. The one thing left in the command is `handleConcurrentJobs` — the question `run up` asks about another worktree's jobs. It is a `flow.Prompter` question in everything but name, and `runlogs` has no Prompter; it stays put until the `--exclusive`/`--parallel` axis is reopened, which worktree isolation may remove entirely.

## Worktree ports and the `.env` — a terminal transformation, not a source

Two modules meet on the `.env` files, and the order they meet in is the whole design.

`internal/service/env` reconciles a worktree's `.env` against a **cascade of value sources** — the parent worktree, then main, then the committed template. `internal/rules/jobports.go` resolves the **host ports** a worktree binds: the base declared in `run.toml` plus that worktree's offset. A `[[env_port]]` link says a `.env` key carries one of those ports, whether alone (`DB_PORT=5432`) or buried in a URL (`DATABASE_URL=postgres://…@localhost:5432/app`).

The tempting move is to make the resolved port a fourth value source. It is wrong, and expensively so. The sources all hold *another* worktree's port — main's, or the parent's — so in `EnvModeRefresh` the key lands in `EnvKeyConflict` between two spellings of the same setting, and `--on-conflict overwrite` dutifully restores main's port, undoing the isolation on every run.

So the port is applied **after** the merge, once, in `settleEnvPorts`, and the diff is taught to compare *modulo the offset*:

| Piece | Where | What it does |
| -- | -- | -- |
| `rules.PlanEnvPorts` | pure | resolves every link against the value on disk; only a base found **exactly once** is rewritten |
| `rules.ReduceEnvPortValue` | pure | rewinds any worktree's port to the base, so `5442` and `5432` compare equal |
| `rules.DiffEnv` (`PortBases`, `PortBlock`) | pure | the single comparison site, in `classifyKey.differ` |
| `env.ApplyEnvPorts` | service | the write, after every file is reconciled |

Two consequences worth keeping:

- **The reduction is modular, not subtractive.** Under the `parent` strategy the source value comes from another worktree whose offset the reader never learns, so `ReduceEnvPortValue` looks for *a number of the shape `base + k×block`* rather than for one known value. A value with no such number, or with two, is left alone — reducing on a guess would hide a real conflict.
- **Every match is bounded by digit boundaries.** Without them base `5432` matches inside `54321` and the substitution silently corrupts the value, which is the exact failure the feature exists to prevent.

The cross-file check has to live outside `config.LoadRun`: that loader only ever sees `run.toml` and validates what `run.toml` can answer for alone. Whether a link names a configured env target needs `config.toml` too, so `rules.ValidateEnvPortTargets` is called where both are in hand — `service/worktree.ResolveEnvPorts`.

**Where the question is put, on a worktree being created.** `internal/flow/envports.Settle` runs after `worktree.Create` — it needs the files to exist — but it does not *decide* there. The decision is the worktree's **isolation**, a step of the run that provisions those files (`create.KeyIsolation`, `checkout.KeyIsolation`, `components.IsolationStep` for the wizard of `extract`), skipped whole when `rules.IsolationApplies` finds nothing in `run.toml` to isolate. `worktree.Create` records the answer in `meta.json` before any hook runs, since a hook reads the ports it decides.

**One choice, read by both halves.** Isolation is not a port-pass option; it is what the worktree *is*, and two readers act on it:

| Reader | Isolated | Verbatim |
| -- | -- | -- |
| `service/worktree.ResolveEnvPorts` — every `.env` writer (create, extract, checkout, `wtm env`, the addressing switch) | links, identity and `[[env]]` values resolved and written | resolves to nothing: the file stays as copied |
| `service/worktree.BranchEnv` — every job and hook | `WTM_PORT_OFFSET = ordinal × block`, `COMPOSE_PROJECT_NAME` derived (the main's without its branch) | offset 0, `COMPOSE_PROJECT_NAME` left to the `.env`, `WTM_ISOLATION=verbatim` |
| `service/process.runNamespace` — the daemon | carves the worktree's namespace | carves nothing (read from `WTM_ISOLATION`: the daemon never reads metadata) |

Splitting them would be incoherent: a `.env` left on its source's ports while the daemon still shifted the jobs would have a front read one port while its back bound another, and the worktree quietly talk to its source. Anything in between the two columns is incoherent by construction, which is why there is no third answer and no flag to rewrite the ports alone. The cost of verbatim is that it shares its source's ports; `flow/run/up` measures that (`rules.PortClashes`) and turns the concurrency question into stop-the-other-or-don't-start rather than letting a bind fail. `wtm env --isolation` switches an existing worktree, and the interactive run asks the same question as a step that keeps the current isolation first, rather than skipping the port pass once.

## The event bus — the daemon relays, the flows speak, the jobs report

`wtm events` and `wtm ui` hear every change to a worktree's identity, whoever made it. The pieces, from producer to consumer:

- **`flow.Publisher`**, a seam on `flow.Context`. A flow publishes right after the mutator succeeds, through `internal/flow/publish` (`Created`, `Updated`, `Relocated`, `Reparented`, `Removed`), which reads the worktree's identity (`worktree.Identity`) after the change. A removal captures the identity *before* (`publish.Capture`), since git has forgotten the worktree once it is gone. A nil publisher publishes nothing, and neither does one that is not `Listening()`: the identity is never read for a run no daemon would relay.
- **`service/events.Publisher`** implements the seam. It stamps the envelope (`v`, `ts`, `repo`) and hands the payload to `process.Publish`, which dials the daemon with a 250 ms budget, never starts it, and reports an error its caller drops: no daemon means no subscriber, and the next subscriber gets the state from its snapshot. Its `Listening()` is a plain dial, asked before every event and never cached, since a run may start the daemon halfway through.
- **The daemon** (`service/process`, `eventhub.go`) is a broker that never decodes the payload: an envelope `{repo, payload}` in, the same out to every subscriber whose filter holds `repo`. A subscriber that falls 256 events behind is disconnected, never waited for; a subscription keeps the daemon alive; `stop()` closes every subscription. `daemonblind` forbids `service/process` to import `service/events`, so the broker cannot become schema-aware by accident.
- **Job events are the one kind the daemon writes**, since only it sees a job end. `Manager` reports each transition (`JobTransition`: started, crashed, exited, stopped) to `ManagerParams.OnTransition`, and `daemonServer.publishJob` marshals a `domain.JobEvent` into the hub. It stays blind to git: the repository and correlation id come from the client in `Request.Origin` (built by `flow.Context.Origin()` from the publisher), the branch from the job's `WTM_BRANCH`, and the origin is persisted in `jobs.json` so an adopted job can still publish its stop. A job started without an origin publishes nothing. A stop sets `ManagedJob.stopping` before it signals, so the reaper does not read the exit as a crash.
- **`service/events.Watch`** subscribes **before** it reads the snapshot (`worktree.Identities`, plus the daemon's job list mapped by `rules.WorktreeJobs`), so a change made meanwhile waits in the subscription and arrives after `ready`. On EOF it backs off and starts over with a fresh snapshot.

The ordinal is part of the identity, so the env readers never allocate it: `JobEnv`, `BranchEnv`, `ResolveEnvPorts` and the hook environment read the ordinal and answer `ErrOrdinalUnallocated` when there is none, and `internal/flow/ordinal` allocates — `Retry` around a read that asked for it, `BeforeHooks` before a hook phase that would read it — and publishes `worktree.updated`. A reader outside any flow (the dashboard's addresses) simply shows nothing for a worktree no run has numbered yet.

## Where each command runs

A new worktree-mutating command is written on the engine, and each command below moves there in turn — see [adding-a-mutation-command.md](adding-a-mutation-command.md). `create` is on the engine; every other one is still a flow. The surfaces each is wired into:

| Command | Flow lives in | Surfaces |
| -- | -- | -- |
| `create` | `internal/command/create` (the engine); `internal/flow/create` for the dashboard and `extract` until they move | CLI form, unattended, JSON v2; dashboard |
| `clean` | `internal/flow/clean` | CLI wizard, unattended, dashboard |
| `reparent` | `internal/flow/reparent` | CLI wizard, unattended, dashboard |
| `prune` | `internal/flow/prune` | CLI wizard, unattended, dashboard |
| `sync` | `internal/flow/sync` | CLI wizard, unattended, dashboard |
| `relocate` | `internal/flow/relocate` | CLI wizard, unattended |
| `checkout` | `internal/flow/checkout` | CLI wizard, unattended |
| `env` | `internal/flow/env` | CLI wizard, unattended |
| `extract` | `internal/flow/extract`, create's steps embedded | CLI wizard, unattended |
| `fast-forward` | `internal/flow/fastforward` | CLI wizard, unattended, dashboard |
| the `run` module | `internal/flow/run/<cmd>` | CLI, run view, dashboard |
