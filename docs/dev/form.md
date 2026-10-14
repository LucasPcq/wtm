# The form — how a command on the engine is asked for

`internal/surface/tui/form` is the interactive form of every command of the registry: a terminal wizard drawn from the command's schema, state and choices, whose recap is the plan of a dry run and whose confirmation runs that plan. It reaches the command through `dispatch` only ([dispatch.md](dispatch.md)), on the contract of [commands.md](commands.md), and is the `cli.Form` of the production engine ([cli.md](cli.md)). No command reaches it yet: the registry holds the `operations` query, and `create` is the first command (step 3 of LUC-283). `flowui` and `inittui` serve the commands still on `flow/` until each is migrated; the dashboard's modal (step 9) and the `records` field of the inits (step 8) come later.

## Running it

```go
reply, err := form.Run(ctx, form.Params{
	Entry:    fillable,  // the command's dispatch.Fillable
	Request:  request,   // what the flags filled, as JSON
	Options:  options,   // --ask, --dry-run, the correlation id
	Names:    spell,     // a field's spelling on the command line: --from, <branches>
	Plan:     rows,      // the rows of a plan: the command's render.Detail.Plan
	Progress: view,      // the progress view of the command line, on stderr
})
```

The command line calls it on an interactive run of a command — stdin a terminal, human output, no `--yes` — and concludes the `Reply` it returns as a run without a form: the form draws no conclusion and no envelope.

## The loop

1. **`Observe` once**, behind a loading box: the facts and their fingerprint, kept while the user types. On `--ask`, `Forget` drops the remembered answers from the facts the form evaluates on; the facts as observed still say what they were.
2. **A step per field** of the schema, in order, then the recap. A step is built when the wizard enters it: the request is the flags' plus the answer of every step asked before, `State` is called on it (pure, no I/O), and the field is asked or settled.
3. **`Choices`** for an asked field that has them, cached under the field's path and the values of what it `DependsOn`: they are listed again only when one of those changes.
4. **The recap**: `Run` with `DryRun`, then the plan and its warnings drawn by `styles.RenderPlan` at the width of the recap's box, as the dry run of the command line draws them ([output.md](output.md#a-command-on-the-engine)), « Yes, apply » / « No, cancel ». A plan with nothing to do offers only « No, cancel ». A dry run that fails titles the step « Cannot apply this plan » and says why under « Review & confirm »: `✗` and the error's sentence, each field or blocker it names indented beneath, « No, cancel » alone.
5. **The run**: once the wizard ends, `Run` with the fingerprint of the plan shown and `Surface: dispatch.SurfaceTUI`; its progress goes to the view the command line handed over, outside the wizard, so Ctrl+C is a signal again.
6. **`412 plan.stale`** (the facts changed while the recap was on screen): the form observes again and reopens on the recap, every field answered before settled, the new plan shown under `! the facts changed since the preview`, inside the recap.

With `--dry-run`, the wizard has no recap: once the fields are answered the form returns the reply of the dry run, which the command line draws as the recap of a dry run.

## Which field is asked

| State of the field | The form |
| -- | -- |
| `skipped` | leaves it out, the reason in the trail (`⊘ Message — <reason>`) |
| `provided` by a flag | settles it, the flag in the trail (`✓ Source: main · --from`) |
| `defaulted`, origin `remembered` | settles it (`✓ Isolation: verbatim · remembered`); on `--ask` the facts forget it and it is asked |
| `defaulted`, origin `default` or `config` | asks it, the default offered; kept, the trail says where it came from (`✓ Source: main · config`) |
| `missing` | asks it |
| any, with errors | asks it, its value offered |

A field answered before a `412` is settled on the second pass. The fields before the first one asked are settled before the wizard opens: the flags alone decide them.

## A field per type

| Type | Drawn with | Checked |
| -- | -- | -- |
| `text` | `components.TextInput` | each keystroke and enter: the errors `State` finds on the field |
| `bool` | `components.Confirm`, the default preselected | — |
| `select` | `components.SelectList`; the pinned option first; an option `kernel.Disabled` refused is disabled, its reason as a badge | the list offers only what may be picked |
| `multiselect` | `components.MultiSelect` | enter: `State` on the set |
| `reorder` | `components.ReorderList`: the value's order, then the choices it lacks | — |
| `textlist` | `components.TextList`, each entry badged as its choice is | an entry is refused on the errors `State` finds on that entry (`branches[2]`), the field on its own |
| `decisions` | `components.DecisionList`: a row per key of the value, ←→ among the choices | enter: `State` on the map |

The choices of a field are its `Choices`, else the values its `Enum` accepts. A `select` in `search` mode (`kernel.ChoicesSearch`) is the branch picker: `/` filters, `r` observes again and lists the choices anew (every cached choice is dropped with the facts it came from). Badges, a choice's banner and every refusal are `kernel.Code`s rendered by the catalogue the command line hands the form (`Params.Catalog`, [commands.md](commands.md#messages)); a field is named by its `Label` in a message, where the command line spells it as a flag. What only the form says (the loading lines, the recap's title and options) is a constant in `text.go`.

## Remembering

A command that declares `kernel.Remember` ([commands.md](commands.md#fields)) gets the "Always use this answer in this repo" box on each rememberable `select`, flipped with tab. The form never shows the `remember` field itself: it writes the paths whose box is ticked into it, and the command's `Apply` keeps them. On `--ask`, the box opens ticked on the answer the repository remembers.

## Backing out

Ctrl+C, Esc on the first field asked, and « No, cancel » end the form with a `kernel.Cancelled` (`unit.interrupted`), which the command line exits with 19. Esc anywhere else goes back a step; the answers after it are asked again.

## Follow-ups

The form does not run what a command proposes next: the command line does, once it has concluded the run ([cli.md](cli.md#the-run)). `form.FollowUp` is the confirmation it shows, the command line that runs the follow-up and why; a yes runs it through the form again, its request filled in.

## What the lint holds

- `layers`: `surface/tui/form` imports `dispatch`, `kernel` (with `kernel/text`), `domain`, `styles`, the rest of `surface/tui` and bubbletea, never `flow/` or `service/`; `surface/cli` imports it.
- `engine`: like every surface, it never imports `command/` or `query/`.

## Testing

The tests follow the kernel's layout, with testify. `fixture_test.go` is a command shaped like `create` with a field of every type, a `remember` field, a dependent `Skip`, a `config` default, a rule refusing an option and choices that load, over a fake repository counting each `Observe` and `Choices`. `helpers_test.go` builds the form on the `dispatchtest` bench and drives the wizard as bubbletea would (`driver`: a key at a time, the form's own loads run and fed back, a load held in flight with `update`); `world.script` runs the whole form with each wizard it opens driven by a turn. `form_test.go` holds the end-to-end cases (apply, dry run, backing out, stale facts), `load_test.go` the I/O (none when a value nothing depends on is typed, choices listed once per value of what they depend on, slow choices, refresh); the other files test their source file. `components.DecisionList` is tested in `components` with the stdlib, as its neighbours are.
