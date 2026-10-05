# Adding a worktree-mutating command

A *mutation command* is one that changes worktree state: it creates, removes, moves or rewrites something, and therefore has questions to ask, safety refusals to honor, and two bypass axes to expose. Every new one goes through `internal/flow/` — the model in [flow-layer.md](flow-layer.md).

A read-only command (`list`, `tree`, `resolve`) needs none of this: parse flags, call the service, hand the result to `output/`.

## 1. Declare the vocabulary in `domain/`

Constants first, so nothing downstream invents a string:

- `domain.CmdSplit` for the command name, `domain.FlagInto` for each new flag (`internal/domain/constants.go`).
- Every user-visible label, description, recap field and skip reason. A step's prose lives in `constants.go`, not as a literal in the flow.
- A sentinel in `internal/domain/errors.go` for each required selection that has no safe default, worded so it names the flag: `ErrSplitTargetRequired`.
- If a surface will have to schedule it, an `OpKind` constant.

## 2. Put the decisions in `rules/`

Anything that is a *decision over data* — is this safe, which of these applies, what is the default here — is a pure function in `internal/rules/`, taking domain types and returning domain types. No I/O, no writing. It is then testable as a table and callable from the flow, the service and any surface.

## 3. Write the flow package

`internal/flow/split/split.go` — the run:

```go
type Request struct {
	Branch string
	Into   string
	Force  bool // the safety axis, if the command has refusals to lift
}

type Outcome struct {
	Branch  string
	Result  domain.SplitResult
	Aborted bool
}

type Presenter interface {
	flow.Presenter
	Split(Outcome) error // the typed conclusion, one per command
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

func Run(ctx context.Context, params Params) (Outcome, error) {
	f := &splitFlow{runCtx: ctx, ctx: params.Context, request: params.Request,
		prompter: params.Prompter, presenter: params.Presenter}
	return f.run()
}
```

Rules that are not negotiable:

- The package imports **only** `internal/service`, `internal/rules`, `internal/domain` and the stdlib. Never cobra, bubbletea, lipgloss, `internal/output`, `internal/tui`, `internal/config` or `internal/commands`. If you need something only `infra/` has, add a thin wrapper in `service/` — as `worktree.FindByBranch` does.
- `Request` carries **no `--yes` and no `--output`**. `--force` does belong there.
- The run's `context.Context` is the first argument, never a `Params` field. Every service call that may reach a subprocess or the daemon takes it, and a `Stage`'s work uses the one `Work` is handed (see [flow-layer.md](flow-layer.md#cancellation)).
- Errors are returned. A user abort is `presenter.Notice(flow.AbortedNotice)` followed by `Outcome{Aborted: true}, nil`.
- Long work goes through `presenter.Stage`; hook output through `presenter.HookPhase`; a line inside an ongoing phase through `presenter.Status`. The flow never prints.

`internal/flow/split/steps.go` — the questions:

```go
const (
	KeyBranch = "split.branch"
	KeyInto   = "split.into"
	KeyRecap  = "split.recap"
)

func (f *splitFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.SplitWizardErrLabel,
		Presets: flow.NewAnswers(map[string]string{
			KeyBranch: f.request.Branch,
			KeyInto:   f.request.Into,
		}),
		Steps: []flow.Step{f.branchStep(), f.intoStep(), f.recapStep()},
	}
}
```

For **each** step, decide its `Resolve` — that is the bypass taxonomy, and it is the step's own business:

| The step is… | `Resolve` |
| -- | -- |
| a decision with a safe default | returns that `Answer`. Never destructive. |
| a required selection with no safe default | returns an error naming the flag, usually the domain sentinel |
| answerable only by a human | omitted entirely — `flow.Unattended` then refuses, naming `Label` and `Flag` |

And the rest of the fields:

- `Skip func(Answers) (skip bool, reason string)` when the step can become irrelevant. The reason is user-visible; put it in `constants.go`.
- `Build` for content derived from earlier answers, `Load` when deriving it does I/O (plus `LoadingMessage`). Never do slow work in `Build`.
- `Summarize` when the raw answer value is not what the user should read back.
- `Flag` so a refusal can name the flag that would have answered the step.
- `Blockers` on the `StepContent` of a step whose dangerous option is gated by safety refusals — one entry per refusal, never folded into the prose, so a surface can have them lifted one at a time.
- The **recap is always the last step** and always unconditional. Its `Build` names every part of the plan, including the parts a flag resolved — read the value from `Answers`, which returns presets too. A flag must never make a line disappear.

If a surface may run several of these at once, declare how:

```go
func Operation() flow.Operation {
	return flow.Operation{Kind: domain.OpKindSplit, Mode: flow.ModeBlocking, TargetKey: KeyBranch}
}
```

## 4. Wire the command

`internal/commands/wt/split.go` holds flag wiring and nothing else:

```go
func runSplit(cmd *cobra.Command, args []string) error {
	into, _ := cmd.Flags().GetString(domain.FlagInto)
	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON && !yes {
		return domain.ErrSplitJSONNeedsYes
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	config, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	// The one place --yes is read: which Prompter gets installed.
	interactive := rules.IsHumanFormat(format) && !yes && term.IsTerminal(int(os.Stdin.Fd()))

	_, err = splitflow.Run(splitflow.Params{
		Context:   flowContext(config),
		Request:   splitflow.Request{Branch: branchName, Into: into, Force: force},
		Prompter:  flowPrompter(flowPrompterParams{Interactive: interactive}),
		Presenter: splitPresenter{cliPresenter: newPresenter(cmd, format)},
	})
	return err
}
```

Flag help strings are uniform:

- `--yes` / `-y` — *"Skip all prompts; resolve every decision from flags and safe defaults (…)"*
- `--force` — *"Lift safety refusals (…); still asks to confirm unless --yes"*

Register the command in its parent group and give it a `GroupID` (`domain.CmdGroup*`), or it renders under a stray "Additional Commands" heading.

## 5. Add the CLI presenter

In `internal/commands/wt/presenter.go`, next to `createPresenter` and `cleanPresenter`:

```go
type splitPresenter struct{ cliPresenter }

func (p splitPresenter) Split(outcome splitflow.Outcome) error {
	if p.format == domain.OutputJSON {
		return output.WriteSplitJSON(p.cmd.OutOrStdout(), outcome.Result)
	}
	output.Frame(p.cmd.OutOrStdout(), func() {
		output.FormatSplitResult(p.cmd.OutOrStdout(), /* … */)
	})
	return nil
}
```

`cliPresenter` already implements `Stage`, `HookPhase`, `Notice` and `Status` — embed it and add only the typed conclusion. The frame is applied **exactly once**, here; JSON and machine output are never framed.

## 6. Test it

- **Flow tests** in the flow package, with `flowtest.ScriptedPrompter` and `flowtest.Recorder`. Assert on the answers that were asked (`AskedKeys()`), on the `StepContent` a step produced (the recap prose is user-visible behavior), and on the outcome.
- **Unattended tests** with `flow.Unattended{}` directly: each required selection refuses and names its flag, each defaulted decision lands on the safe value.
- **Integration tests** at the Cobra level (`gittest.InitRepo` + `WTM_PROJECT_DIR` / `WTM_STATE_DIR`) for the two axes end to end: `--yes` without the required flag errors, `--force` alone still confirms, `--output json` requires `--yes`.

## 7. Documentation

1. `make docs` — regenerates `docs/`, never hand-edited.
2. Add the command to the `README.md` overview table, in the same group as the root `--help`.
3. Update the agent skill (`internal/commands/agents/assets/using-wtm/`, the reference file of the command's topic) if the agent-facing surface changed (a new command, a new flag, a changed JSON shape, changed failure/abort semantics).
4. Run the `build-validator` subagent. Step 6 fails the run if `internal/flow/` gained a forbidden import.

## Optional: make it work in the dashboard

Nothing in the flow changes. In `internal/tui/dashboard/actions.go`, add a `startSplit` that checks `busyReason`, calls `beginOp(splitflow.Operation())`, and launches `splitflow.Run` in a `tea.Cmd` with the dashboard's `prompter` and a presenter embedding `dashboard.presenter` plus the typed conclusion. If the flow uses a step kind the dashboard's modal cannot render, that is the only work left — and `flowui` will refuse an unknown kind rather than guess, so you will hear about it immediately.
