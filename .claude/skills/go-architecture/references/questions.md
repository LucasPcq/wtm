# Questions: a field, the schema, the shared fragment

A command's request is also data: a schema (`domain.Form`) that the flags, `--help`, the wizard, the command line it spells and any other surface read without knowing the command. The core gives data and decisions; each surface gives its words (below, "Words").

## What `questions.go` holds

Pure (no port, no context), in three sections:

- **The request:** `Request`, the command's fields with snake_case JSON tags, and a path constant per field, exported since each surface words a field by its path (`PathFrom = "from"`). `Request.Answer(path, values) (Request, error)` writes a field's answer, leaving the request unchanged when the value is refused; `Request.Values(path) []string` reads it back spelled the same way, nil when the field is left to its default. Generic code cannot read a struct's fields: these two are how every surface reads and writes the request.
- **The form:** `Questions`, built from the domain vocabulary (`domain.Answer[T]`, `domain.Choice[T]`, `domain.Confirm`) by `NewQuestions(facts, req)`, and `Questions.Form() domain.Form`: the fields in the order the wizard asks them, with the request's problems. No word lives here: no label, help, flag name nor option label. `Form()` works on a zero `Questions`, since the flags and `--help` exist before any repository is read.
- **Resolve:** `Resolve(facts, req) (Request, error)` answers every field as `NewQuestions` does and refuses the request with all its problems at once (`domain.Invalid`).

## The field kinds

`domain.Field` is sealed (`domain/form.go`); each kind embeds `domain.FieldSpec{Path, Origin, Skip}`. A select whose options are every value it accepts, the same in every repository, says so with `Closed`, and the schema publishes its values as an enum of the command's request.

| Kind | Asks |
| -- | -- |
| `SelectField` | one option; may be `Pinned`, `Refreshable` (r reads the options again), `Rememberable`; `Refused` lists the values it leaves out of its options, each as the field error it would raise if given (reparent's parents that would close a cycle), so a surface says why before the request does |
| `MultiSelectField` | any options |
| `TextField` | one line |
| `TextListField` | a list of entries, with the values the repository knows as `Options` |
| `ConfirmField` | yes or no, valued `true`/`false`, no first when `NoFirst`; may be `Rememberable` |

An option is a `domain.Option`: its `Value` and what the repository knows of it, facts a surface words its own way: `Tag` (a `domain.OptionTag` the schema publishes as an enum: local, checked out, only on origin, current, main), `Base` (the configured base branch), `Detached`, `Parent` (the branch a worktree is rebased onto), `Origin` (where the branch stands against origin's, a `worktree.Origin`), `Changes` (a count), `Change` (the `worktree.Change` it is). A closed select's options are its values alone (`domain.OptionsOf`). A new kind is a type in `domain/form.go`, its JSON `kind` in `fieldKind`, and a step in `ui/form`; go-sumtype then lists every switch that must handle it.

## Adding a field

1. The request field, its JSON tag, its path constant.
2. Its case in `Request.Answer` (`domain.One`, `domain.Yes` spell it) and `Request.Values`.
3. Its question in `Questions`, settled in `NewQuestions`: given, else remembered (unless `--ask`), else configured, else default, with that `Origin`.
4. Its field in `Form()`, where the wizard asks it.
5. Its problems in the validation: a `domain.FieldError{Path, Code, Template, Params}` naming the field or an entry (`branches[1]`), coded `<name>.<problem>` or with a shared field code (`domain.CodeOneOf`).
6. If the plan depends on it, its use in `rules.go` and in `Plan.Fingerprinted()`.
7. Its words in `cli/words.go` (below); the web's in its screen's `words.ts` when the web renders the command.

Nothing else in `cli/`: the engine registers its flag, usage and step from the schema and its words. A field two steps answer (extract's target, picked as a worktree then typed as a branch) appears once per step. A question that does not apply is skipped with its reason (`Skip`, a `domain.Skip` code the schema publishes as an enum, which the wizard words in `ui/form/skip.go` and each other surface its own way), never left out: the recap still shows it, and a flag or a remembered answer never makes a recap line disappear. Only a select or a confirmation that opts in is rememberable, never a destructive option; `--remember <field>` (`domain.PathRemember`) names the answers to write.

## Words

The core gives data and decisions, each surface gives its words. A field's flag (`<branches>`, positional; `--from`; `-- <command>`, the words after `--` as one value, spelled last: exec's command), its label, its help (the wizard's description and the flag's usage), a multi-select's problem of none picked, a closed select's or a confirmation's option labels and how the trail names each (`Short`): the command line's, all of them, in the command's `cli/words.go`, a `form.Worder[Q]`: `func(q Q, field domain.Field) form.Words`, switching on the field's path (and on its kind where one path has two fields, extract's target). Words built from data read it from the typed questions (`q.Behind`, `q.OnConflict.Conflicts`, `q.Source.Given`), never from a field the core words. What the CLI says of an option's facts (its badge, `main (default)`, `(detached)`, a rename's arrow) is the engine's, in `ui/form/words.go`, and so are the words of the questions of a new worktree (`form.NewWorktree`), which create and extract share. The flag is a word too: only the command line reads it, and the web names a field by its path. Each `cli` test calls `formtest.Worded[Q](t, words)`, which fails while a field of the schema lacks its flag, label or help, a multi-select its problem of none, or a closed select's option its label: a question cannot ship without its words. The web's words are exhaustive Records over the paths and values the schema publishes (`next/web/src/features/worktrees/create/utils/words.ts`, `web-front` skill), so a new question fails its build too. A skip is a code (`domain.Skip`), worded by `ui/form/skip.go` and a command's `Skipped`. A problem (`code` and `message`) still carries the core's message.

## The shared fragment: a question two commands ask

It is one fragment owned by the service that consumes its answers, never rewritten by a command. The model is `service/provision/questions.go`, the questions of a new worktree (env strategy, isolation, source fast-forward):

| Part | How a command uses it |
| -- | -- |
| `provision.Given`: the fragment's request fields, with `Answer`/`Values` for its paths (`remember` and `ask` among them) | embedded in the command's `Request`, whose `Answer` and `Values` hand it every path they do not know (`default:`) |
| `provision.Questions`, built by `provision.NewQuestions(QuestionsParams{…})` | embedded in the command's `Questions` |
| `Questions.Fields()` | spliced into the command's `Form()` where the wizard asks them; their command-line words are `form.NewWorktree{EnvHelp, Configured, Behind, Many}.Words(path)`, which the command's `words` asks first |
| `provision.Answers`: unexported fields, returned only by `Questions.Answers()` | the only input `provision.Service.Run` provisions from; it refuses the zero value |

The input type enforces the fragment: no command provisions a worktree without asking these questions, and a question added here reaches every command that embeds it — flag, help, step, remembered answer — with no change in the commands but their goldens. To add one: its path, its `Given` field with `Answer`/`Values`, its question in `NewQuestions`, its field in `Fields`, its value in `Answers`, its use in `Run`.

## Tests

`questions_test.go`, table-driven over facts from `fixture_test.go`: each field's answer and origin, the problems, `Resolve`; and the round trip — folding `Form().Answers()` into a zero request (`domain.Fold`, the engine's own fold) gives back the request, and `Answer` takes back what `Values` spells (`command/create/questions_test.go`). The `--help` golden shows each field's help, from its words; `words_test.go` in each `cli` runs `formtest.Worded`.
