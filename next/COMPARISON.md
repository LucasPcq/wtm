# `wtm create`: engine version vs `next/`

Both versions run the same command on the same state files. Driven side by side in two identical sandboxes (`docs/proof/`), their stdout, stderr, exit codes, `meta.json`, `.env` files and `config.toml` are byte-identical over 15 command-line scenarios. In the terminal, the only differences are three glyphs and the first Ctrl+C message, both decided on purpose (`docs/proof/README.md`).

## Lines of code

Lines of Go per package (non-test / test), counted with `wc -l` on whole files.

**`next/`**: everything `wtm-next create` reaches. Nothing under `next/` imports `internal/`.

| Package | Code | Tests |
| -- | --: | --: |
| `cmd/wtm-next` (wiring, project resolution) | 179 | 114 |
| `surface/cli` | 500 | 637 |
| `surface/output` | 1,260 | 698 |
| `surface/form` (wizard and its 3 components) | 1,308 | 388 |
| `app` (locks, fingerprint, events) | 252 | 439 |
| `app/create` | 1,096 | 1,179 |
| `app/provision` | 608 | 924 |
| `adapter/*` (git, shell, statefile, store, config, oplock, events, proxyport) | 2,468 | 4,208 |
| `wtm/*` (the domain: 12 packages) | 2,929 | 2,345 |
| **Total** | **10,600** | **10,932** |
| test doubles `adapter/gittest`, `adapter/storetest` | 965 | 215 |

**Engine**: create's own packages, plus the machinery and services it runs on, counted as whole packages.

| Package | Code | Tests |
| -- | --: | --: |
| `command/create` | 1,112 | 776 |
| `command/provision` | 428 | 511 |
| `kernel`, `kernel/text` | 1,886 | 1,888 |
| `dispatch` | 1,215 | 1,317 |
| `surface/cli`: the generic binding and `create.go` (15 files) | 1,547 | 1,497 |
| `surface/tui/form` | 1,070 | 858 |
| `store`, `infra/git`, `infra/fs`, `infra/proc` | 1,838 | 3,092 |
| `service/gitwt`, `identity`, `runenv`, `hooks` | 1,115 | 1,606 |
| `rules/branch`, `envcopy`, `runenv`, `statefile`, `worktree` | 456 | 333 |
| `config` | 798 | 1,537 |
| **Subtotal** | **11,465** | **13,415** |
| shared packages it reaches only in part: `rules` (21,389), `domain` (8,613), `surface/tui/components` (7,082), `surface/cli/render` (5,102), `service/process` (4,694), `styles` (734)… | ≈ 9,000–10,000 reached of ≈ 52,000 | |
| test doubles `testutil/gittest`, `testutil/storetest` | 1,152 | |

The import closure of the engine's create path holds 34 packages and 62,375 lines; the closure of `next/` holds 27 packages and 10,600 lines.

## Following `create feat/x --from main` from the flag to `git worktree add`

Both columns include what the run passes through on the way: signals, operation locks and the fingerprint.

| | Files opened | Packages |
| -- | --: | --: |
| `next/` | 26: `cmd/wtm-next/{main,project,app}.go`; `surface/cli/{root,create,signal}.go`; `app/{create,app,fingerprint}.go`; `app/create/{create,facts,resolve,questions,decision,locks,subjects,plan,apply}.go`; `app/provision/{provision,add}.go`; `adapter/oplock/{oplock,holder}.go`; `adapter/statefile/lock.go`; `adapter/git/{worktree,repo}.go`; `adapter/shell/command.go` | 9 |
| engine | ≈ 35: `main.go`; `surface/cli/{root,create,bind,run,request,engine,signal}.go`; `registry/registry.go`; `dispatch/{registry,handle,call,run,locks,fingerprint}.go`; `kernel/{command,spec,field,form,each,unit}.go`; `command/create/{create,request,fields,facts,plan,apply}.go`; `command/provision/{provision,saga}.go`; `service/gitwt/gitwt.go`; `infra/git/{write,repo}.go`; `infra/fs/locks.go`; `infra/proc/command.go` | 12 |

## What a second command (`clean`) would add

| `next/` | engine |
| -- | -- |
| `app/clean/{clean,facts,plan,apply}.go` + tests | `command/clean/{clean,request,facts,fields,plan,apply,codes,events}.go` + tests |
| `app/clean.go`: `App.Clean` (locks, fingerprint and events, the shape of `app/create.go`) | one line in `registry.Production` |
| `surface/cli/clean.go`: flags, help and the call | `surface/cli/clean.go`: its `Doc`, a positional and its `Detail` (plan rows) |
| `surface/output/clean.go`: its plan and item rows, and the envelope's `command` | nothing: generic rendering and envelope |
| `surface/form/clean.go`: its questions over the existing components | nothing: the generic form |
| `adapter/store`: removing a `meta.json`; wiring in `cmd/wtm-next/app.go` | `store`: removing a `meta.json` |

## Conclusion

1. At full parity `next/` is 10.6k lines for the whole binary, against 11.5k for create's slice of the engine plus about 9–10k it reaches in shared packages: about half the code, and a 27-package closure instead of 34 packages and 62k lines.
2. A reader follows `create` through 26 files in 9 packages instead of about 35 in 12. No registry, schema, generic request or saga engine stands between the flag and the git call.
3. The engine's real gain is per command: the command line, the JSON envelope and the form come for free. In `next/` each command writes its own (about 300–500 lines for clean), with the components and output primitives already shared.
4. The engine's generic layers (kernel, dispatch, binding, form: about 5.7k lines) cost more than create's own logic (1.5k). They pay off only after several more commands, and they carried bugs that were fixed in `next/`, such as the apply line dropping `--ff=false` and the clean follow-up spelled with a flag clean lacks.
5. Recommendation: build the next commands as slices over `next/`'s domain and adapters. Extract a shared helper only when a second command needs it, not ahead of time.
