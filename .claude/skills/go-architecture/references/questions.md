# Questions: a field, the schema, the shared fragment

A command's request is also data: a schema (`domain.Form`) that the flags, `--help`, the wizard, the command line it spells and any other surface read without knowing the command.

## What `questions.go` holds

Pure (no port, no context), in three sections:

- **The request:** `Request`, the command's fields with snake_case JSON tags, and a path constant per field (`pathFrom = "from"`). `Request.Answer(path, values) (Request, error)` writes a field's answer, leaving the request unchanged when the value is refused; `Request.Values(path) []string` reads it back spelled the same way, nil when the field is left to its default. Generic code cannot read a struct's fields: these two are how every surface reads and writes the request.
- **The form:** `Questions`, built from the domain vocabulary (`domain.Answer[T]`, `domain.Choice[T]`, `domain.Confirm`) by `NewQuestions(facts, req)`, and `Questions.Form() domain.Form`: the fields in the order the wizard asks them, with the request's problems. Labels, help, flag names and option labels live here. `Form()` works on a zero `Questions`, since the flags and `--help` exist before any repository is read.
- **Resolve:** `Resolve(facts, req) (Request, error)` answers every field as `NewQuestions` does and refuses the request with all its problems at once (`domain.Invalid`).

## The field kinds

`domain.Field` is sealed (`domain/form.go`); each kind embeds `domain.FieldSpec{Path, Flag, Label, Help, Origin, Skip}`, `Help` being both the wizard's description and the flag's usage: CLI copy, which another surface replaces with its own words. A select whose options are every value it accepts, the same in every repository, says so with `Closed`, and the schema publishes its values as an enum of the command's request.

| Kind | Asks |
| -- | -- |
| `SelectField` | one option; may be `Pinned`, `Refreshable` (r reads the options again), `Rememberable` |
| `MultiSelectField` | any options; `Empty` is the problem of picking none |
| `TextField` | one line |
| `TextListField` | a list of entries, with the values the repository knows as `Options` |
| `ConfirmField` | yes or no, valued `true`/`false`; may be `Rememberable` |

An option is a `domain.Option{Value, Label, Badge, Short}`. A new kind is a type in `domain/form.go`, its JSON `kind` in `fieldKind`, and a step in `ui/form`; go-sumtype then lists every switch that must handle it.

## Adding a field

1. The request field, its JSON tag, its path constant.
2. Its case in `Request.Answer` (`domain.One`, `domain.Yes` spell it) and `Request.Values`.
3. Its question in `Questions`, settled in `NewQuestions`: given, else remembered (unless `--ask`), else configured, else default, with that `Origin`.
4. Its field in `Form()`, where the wizard asks it.
5. Its problems in the validation: a `domain.FieldError{Path, Code, Template, Params}` naming the field or an entry (`branches[1]`), coded `<name>.<problem>` or with a shared field code (`domain.CodeOneOf`).
6. If the plan depends on it, its use in `rules.go` and in `Plan.Fingerprinted()`.

Nothing in `cli/`: the engine registers its flag, usage and step from the schema. A field two steps answer (extract's target, picked as a worktree then typed as a branch) appears once per step. A question that does not apply is skipped with its reason (`Skip`, a `domain.Skip` code the schema publishes as an enum, which the wizard words in `ui/form/skip.go` and each other surface its own way), never left out: the recap still shows it, and a flag or a remembered answer never makes a recap line disappear. Only a select or a confirmation that opts in is rememberable, never a destructive option; `--remember <field>` (`domain.PathRemember`) names the answers to write.

## The shared fragment: a question two commands ask

It is one fragment owned by the service that consumes its answers, never rewritten by a command. The model is `service/provision/questions.go`, the questions of a new worktree (env strategy, isolation, source fast-forward):

| Part | How a command uses it |
| -- | -- |
| `provision.Given`: the fragment's request fields, with `Answer`/`Values` for its paths (`remember` and `ask` among them) | embedded in the command's `Request`, whose `Answer` and `Values` hand it every path they do not know (`default:`) |
| `provision.Questions`, built by `provision.NewQuestions(QuestionsParams{…})` | embedded in the command's `Questions` |
| `Questions.Fields(…)` | spliced into the command's `Form()` where the wizard asks them |
| `provision.Answers`: unexported fields, returned only by `Questions.Answers()` | the only input `provision.Service.Run` provisions from; it refuses the zero value |

The input type enforces the fragment: no command provisions a worktree without asking these questions, and a question added here reaches every command that embeds it — flag, help, step, remembered answer — with no change in the commands but their goldens. To add one: its path, its `Given` field with `Answer`/`Values`, its question in `NewQuestions`, its field in `Fields`, its value in `Answers`, its use in `Run`.

## Tests

`questions_test.go`, table-driven over facts from `fixture_test.go`: each field's answer and origin, the problems, `Resolve`; and the round trip — folding `Form().Answers()` into a zero request (`domain.Fold`, the engine's own fold) gives back the request, and `Answer` takes back what `Values` spells (`command/create/questions_test.go`). The `--help` golden shows each `Help`.
