# Contributing to wtm

Thanks for taking the time. Bug reports, recipes for a stack wtm does not cover yet, documentation fixes and code are all welcome.

## Before you start

- **A bug:** open an [issue](https://github.com/LucasPcq/wtm/issues/new/choose). The template asks for `wtm version --output json` and the commands that reproduce it; that is usually enough to act on.
- **A question** (setting up a stack, whether wtm fits a workflow): ask in [Discussions](https://github.com/LucasPcq/wtm/discussions).
- **A change larger than a fix:** open an issue first and describe the problem it solves. wtm keeps a small command surface and strict conventions, so agreeing on the shape before the code saves you a rewrite.
- **A recipe** (a `run.toml` and its scripts for a database, an identity provider, a framework): it goes in [`docs/guide/recipes.md`](docs/guide/recipes.md), no Go needed.

## Setup

You need Go (the version in [`go.mod`](go.mod)) and `git`. [`gh`](https://cli.github.com) is needed for the GitHub features, [`vhs`](https://github.com/charmbracelet/vhs) only to re-record the README GIFs.

```bash
git clone https://github.com/LucasPcq/wtm && cd wtm
make build            # bin/wtm
./bin/wtm --help
```

Try a change against a throwaway repository, never against a repository you care about: `create`, `clean`, `prune` and `run up` touch worktrees, branches and containers for real.

## The gates

```bash
make lint    # gofmt, go vet, architecture rules (tools/archlint), dead code, staticcheck
make test    # go test ./... -race -count=1
make docs    # regenerate docs/wtm_*.md when a command or a flag changed
```

CI runs the same gates on every pull request. When `make lint` fails and the reason is unclear, [`docs/dev/lint.md`](docs/dev/lint.md) explains each rule.

## How the code is organised

wtm follows a layered architecture with rules checked by `make lint`: `commands/` wires flags, `flow/` holds each command's flow, `service/` does the I/O, `domain/` holds types and constants, `rules/` pure functions, `output/` and `tui/` only render. Start with:

- [`CLAUDE.md`](CLAUDE.md): the coding standards (parameter structs, constants, early returns, comments, the bypass flags `--yes` and `--force`).
- [`docs/dev/`](docs/dev/README.md): the package map, the flow layer, how a command prints, and the recipe for adding a worktree-mutating command.

## Docs

- `docs/wtm_*.md` is generated from the Cobra tree: never edit it by hand, run `make docs`.
- `docs/guide/` (user guide) and `docs/dev/` (developer docs) are hand-written. A behaviour change updates the page that describes it in the same pull request.
- A user-facing change adds a line to `CHANGELOG.md` under `## [Unreleased]`, following [`docs/dev/changelog.md`](docs/dev/changelog.md).
- Markdown is never hard-wrapped: one paragraph is one line.

## Commits and pull requests

- Commit messages are in English, in the [Conventional Commits](https://www.conventionalcommits.org) style the history uses: `fix(create): …`, `feat(run): …`, `docs(guide): …`.
- Open the pull request against `main`, or against the parent branch when it is stacked on another one. The template asks for what changes, a proof (a terminal capture before and after, or a GIF for an interactive flow), how to verify it, and the risk.
- Keep one pull request to one change: a refactor and a feature are easier to review apart.

## Code of conduct

Everyone taking part in wtm's issues, discussions and pull requests follows the [Code of Conduct](CODE_OF_CONDUCT.md). Report a problem privately to contact@wtm.sh.
