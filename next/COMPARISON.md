# `wtm create`: engine version vs `next/`

Both versions run the same command on the same state files. Driven side by side in two identical sandboxes (`docs/proof/`), their stdout, stderr, exit codes, `meta.json`, `.env` files and `config.toml` are byte-identical over 15 command-line scenarios. In the terminal, the only differences are three glyphs and the first Ctrl+C message, both decided on purpose (`docs/proof/README.md`).

## Lines of code

Lines of Go per package (non-test / test), counted with `wc -l` on whole files.

**`next/`**: everything `wtm-next` reaches, `create` and `extract`. Nothing under `next/` imports `internal/`.

| Package | Code | Tests |
| -- | --: | --: |
| `cmd/wtm-next` (composition root, project resolution) | 259 | 134 |
| `ui/cli` (root, signals, exit codes, a command's run plumbing) | 338 | 105 |
| `ui/output` | 1,361 | 530 |
| `ui/form` (wizard engine and its 5 fields) | 1,743 | 742 |
| `service/operation` (the operation every use case goes through: locks, fingerprint, events) | 229 | 0 |
| `service/provision` | 935 | 1,166 |
| `command/create`, `command/create/cli` | 2,017 | 2,843 |
| `command/extract`, `command/extract/cli` | 2,468 | 1,970 |
| `adapter/*` (git, shell, statefile, store, config, oplock, events, proxyport) | 3,587 | 4,823 |
| `domain`, `domain/*` (5 packages) | 3,747 | 2,578 |
| **Total** | **16,684** | **14,891** |
| **Total without `extract`'s own packages** | **14,216** | **12,921** |
| test doubles `adapter/gittest`, `adapter/storetest` | 1,319 | 252 |

Since the first count (10,418), every exported name gained a doc comment (revive gates it: about 1,400 comment lines on `create`'s path, against 42 before), and the shared packages gained about 1,100 lines that only `extract` calls (git's changes, conflicts and move; the multi-select and the text field).

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

The import closure of the engine's create path holds 34 packages and 62,375 lines; the closure of `wtm-next` holds 23 packages and 16,684 lines, 21 packages and 14,216 lines without `extract`.

## Following `create feat/x --from main` from the flag to `git worktree add`

Both columns include what the run passes through on the way: signals, operation locks and the fingerprint.

| | Files opened | Packages |
| -- | --: | --: |
| `next/` | 19: `cmd/wtm-next/{main,project,wire}.go`; `ui/cli/{root,signal,run}.go`; `command/create/cli/command.go`; `command/create/{usecase,questions,rules}.go`; `service/operation/operation.go`; `service/provision/{observe,provision}.go`; `adapter/oplock/{oplock,holder}.go`; `adapter/statefile/lock.go`; `adapter/git/{worktree,repo}.go`; `adapter/shell/command.go` | 10 |
| engine | ≈ 35: `main.go`; `surface/cli/{root,create,bind,run,request,engine,signal}.go`; `registry/registry.go`; `dispatch/{registry,handle,call,run,locks,fingerprint}.go`; `kernel/{command,spec,field,form,each,unit}.go`; `command/create/{create,request,fields,facts,plan,apply}.go`; `command/provision/{provision,saga}.go`; `service/gitwt/gitwt.go`; `infra/git/{write,repo}.go`; `infra/fs/locks.go`; `infra/proc/command.go` | 12 |

## What a second command cost: `extract`, measured

`extract` (move a worktree's uncommitted changes into a new worktree) is the second slice over `next/`. Lines of Go today, code / tests; the shared pieces are counted at their current size.

| | Code | Tests |
| -- | --: | --: |
| **`extract`'s own**: `command/extract/{usecase,rules,questions,port}.go` | 1,271 | 1,129 |
| **`extract`'s own**: `command/extract/cli/{command,commandline,flag,form,steps,wizard,recap,output}.go` | 1,197 | 841 |
| **`extract`'s own**: its use case wired in `cmd/wtm-next/{main,project,wire}.go` | ≈ 25 | |
| **added to the shared pieces for it**: git's changes, exact conflicts and move (`adapter/git/changes.go`, `domain/worktree/change.go`; `0513f6b4`, fixed in `622fbbce`) | 714 | 370 |
| **added to the shared pieces for it**: the multi-select and the text field (`ui/form/{multiselect,text}.go`; `85337aa3`, `147693a7`) | 362 | 112 |
| **moved out of `create` to be shared by both**: the wizard engine (`ui/form/{wizard,wizardview,step,run,recap}.go`; `85337aa3`) | 756 | 316 |
| **moved out of `create` to be shared by both**: `worktree.Place`, the question vocabulary, the snapshot and repository facts (`03bae958`, `c9765148`, `44277de1`) | 392 | 240 |
| **moved out of `create` to be shared by both**: `provision.FastForwardEach` and `provision.Observe` (`03bae958`, `44277de1`) | 157 | 167 |
| **moved out of `create` to be shared by both**: `uicli.Run` and the command line a dry run prints (`ui/cli/run.go`, `ui/output/commandline.go`; `8eedc9ab`, `1d5dbbea`) | 179 | 26 |
| **reused as they were**: `service/operation`, the rest of `service/provision`, the other adapters and the rest of git's, `domain/env`, `dotenv`, `project`, the rest of `domain` and `ui/*` | 9,380 | |

What the engine would add for a second command (`clean`), for comparison:

| `next/` | engine |
| -- | -- |
| `command/clean/{usecase,rules,questions,port}.go` + tests: `usecase.go` tells it (`Run`, `Plan`, `Apply`, `Questions`) through `service/operation` | `command/clean/{clean,request,facts,fields,plan,apply,codes,events}.go` + tests |
| its constructor in `cmd/wtm-next/main.go`, its adapters in `wire.go` | one line in `registry.Production` |
| `command/clean/cli/command.go`: flags, help and the call, over `uicli.Run` | `surface/cli/clean.go`: its `Doc`, a positional and its `Detail` (plan rows) |
| `command/clean/cli/output.go`: its plan and item rows, and the envelope's `command` | nothing: generic rendering and envelope |
| `command/clean/cli/{steps,wizard}.go`: its steps over the shared wizard engine | nothing: the generic form |
| `adapter/store`: removing a `meta.json` | `store`: removing a `meta.json` |

## Conclusion

1. At full parity `next/` is 14.2k lines for `create` (16.7k with `extract`), against 11.5k for create's slice of the engine plus about 9–10k it reaches in shared packages: about two thirds of the code, and a 21-package closure instead of 34 packages and 62k lines.
2. A reader follows `create` through 19 files in 10 packages instead of about 35 in 12. No registry, schema, generic request or saga engine stands between the flag and the git call.
3. The engine's real gain is per command: the command line, the JSON envelope and the form come for free. In `next/` each command writes its own (1.2k lines for `extract`'s command line, wizard steps included), over the run plumbing, the wizard engine and the output primitives already shared.
4. The engine's generic layers (kernel, dispatch, binding, form: about 5.7k lines) cost more than create's own logic (1.5k). They pay off only after several more commands, and they carried bugs that were fixed in `next/`, such as the apply line dropping `--ff=false` and the clean follow-up spelled with a flag clean lacks.
5. The second command, measured: `extract` cost 2.5k lines of its own (1.3k use case, 1.2k command line) and 1.1k added to the shared pieces (git's changes, conflicts and move; two form fields), reused 9.4k lines as they were, and moved 1.5k out of `create` to be shared by both.
6. Recommendation: build the next commands as slices over `next/`'s domain and adapters, each through the operation `service/operation` already writes once. Extract any other shared helper only when a second command needs it, as `extract` did, not ahead of time.
