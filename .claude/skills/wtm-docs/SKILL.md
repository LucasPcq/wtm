---
name: wtm-docs
description: Write and maintain wtm's user documentation so it stays one coherent guide as features land — decide where a fact belongs (--help, docs/guide/, the using-wtm agent skill, docs/dev/, CHANGELOG, a migration page), give a guide page its standard shape (noun-phrase title, lede, real examples, no hand-written TOC), keep docs/guide/README.md (which builds the site's sidebar) in its sections, write in the project's factual voice, and audit the whole guide for drift. Use this whenever a change adds, renames or alters a command, flag, config key, output or behaviour and the docs must follow; whenever the user asks to document something, write or rework a guide page, add a recipe or a troubleshooting entry, write a changelog entry or migration notes, reorganise the guide or its sidebar, or "faire un tour" / review / audit / clean up the docs — even when they only say "update the docs" or "is this documented?".
---

# Maintaining wtm's docs

The guide is read by someone who has just installed wtm or just hit a wall. They arrive from a search or the sidebar, read one page, and leave. Every rule below serves that reader: they must find the page from its title, know from its first paragraph whether it answers them, and trust every example on it. A guide that grows one feature at a time drifts unless each addition is placed, not appended — that is the job of this skill.

The voice is the project's, defined once in [`docs/dev/identity.md`](../../../docs/dev/identity.md) ("Voice"): factual engineer, short concrete sentences, every promise shown by real output, no superlatives. Read that section before writing prose; it is not repeated here so the two can never disagree.

## 1. Place the fact before writing it

Most drift comes from a fact written in the wrong place, then duplicated elsewhere when someone looks for it. Decide where it lives first:

| The fact is… | It lives in | How |
| -- | -- | -- |
| what one flag or command does | the Cobra `Short`/`Long`/flag help | `make docs` regenerates `docs/wtm_*.md`; never hand-edit those |
| a concept spanning several commands, a setup, a model the user needs | `docs/guide/<page>.md` | this skill, §2–4 |
| something an agent driving wtm must know (command surface, `--output json`, exit codes, abort semantics) | `internal/surface/cli/agents/assets/using-wtm/references/<topic>.md` | `SKILL.md` there only when every task needs it |
| a complete setup to copy | a section of `docs/guide/recipes.md` | one `##` per recipe, its final `run.toml` and the commands |
| a symptom and its fix | a section of `docs/guide/troubleshooting.md` | **Symptom.** / cause / the command that fixes it |
| what changed in a release | `CHANGELOG.md` | rules in [`docs/dev/changelog.md`](../../../docs/dev/changelog.md) |
| what a user must do to upgrade, beyond one line | `docs/guide/migrating-to-<x.y>.md` | linked from the Breaking bullet |
| how wtm is built inside | `docs/dev/` | not the guide |
| the landing, README pitch, docs theme | — | the `wtm-site` skill |

A fact has one home. Other pages link to it (`[isolation](isolation.md#changing-your-mind)`) instead of restating it; a restated fact is the one that goes stale.

## 2. A new page or a section?

Add a **section** to an existing page by default. Create a **page** only when the topic is a concept a reader would search for by name and it does not fit the scope stated in an existing page's lede. Signs a page should split instead: it passes ~300 lines, or its lede needs "and" to cover two audiences (e.g. configuring vs. scripting).

Each page belongs to one section of the index. The sections are fixed by [`docs/guide/README.md`](../../../docs/guide/README.md):

- **Start here** — install, first run, platforms, comparisons
- **Worktrees** — `config.toml`, provisioning, hooks, state on disk
- **Dev stacks with `wtm run`** — everything opt-in behind `run.toml`
- **Scripts and agents** — `--yes`, `--output json`, events, integrations
- **Recipes and help** — recipes, troubleshooting
- **Upgrading** — migration pages, newest first (collapsed in the sidebar)

Adding or renaming a section is a product decision: propose it to the user, don't do it in passing.

## 3. The shape of a guide page

Every page has the same skeleton, so a reader who knows one page knows them all:

```markdown
# Shared services

Isolation duplicates everything: … A **shared service** runs once for the whole repository instead, and gives each worktree its own **namespace** in it.

<the most common case, with a real example>

## Declaring one
## Where it runs
…
```

- **Title**: a noun phrase naming the concept, no colon subtitle (`Isolation`, not `Isolation: isolated or verbatim`). Backticks only when the title *is* a command or file (`` `run.toml` reference ``).
- **Lede**: one to three sentences, before any code block or heading, saying what the page covers and when a reader needs it. It is what the reader uses to decide whether to stay, and what search and `llms.txt` show.
- **Common case first**, then variations, then edge cases and what wtm does not do. Headings are noun phrases or short statements (`## Changing your mind`, `## What clean and prune do with the data`).
- **Examples are real.** Output comes from running wtm (the `wtm-sandbox` skill) and is trimmed, never typed from memory. New examples use the sandbox repository `acme`, its branches (`feat/login`, `fix/header`) and the names wtm really derives from them.
- **Shell blocks**: commands alone go in ` ```bash `; commands with their output go in ` ```console `, each command behind `$ `. The site colours a `console` command exactly like a `bash` one and leaves output plain (`site/src/lib/console-highlight.mjs`).
- **No hand-written table of contents**: the site shows "On this page", GitHub shows its outline.
- **State, not history**: describe how wtm works now. "Since 0.28", "we changed", "previously" belong in the changelog or a migration page, never in a concept page.
- **Links** are relative to the Markdown file (`[run.toml](run-toml.md#job)`), so GitHub and the site both resolve them; `sync-docs.mjs` rewrites them for the site. Never write site-only Markdown.
- One paragraph is one line (no hard wrap). English.

## 4. Register it in the index

`docs/guide/README.md` is both the guide's index and the site's sidebar (`site/src/lib/guide.mjs` reads its `##` sections). A page missing from it is invisible on the site. Add one line in its section:

```markdown
- **[Shared services](shared-services.md)**: one postgres for the repository, a namespace per worktree.
```

The link text is the sidebar label: two to four words, the page's title or a shorter form of it. The text after the colon lists what the page covers, in the reader's words. Order within a section follows a reader's path (concept before reference).

When a page is renamed or removed, `grep` for its file name across `docs/`, `README.md`, `site/src/` (the landing and `docs-home.mdx` link into the guide) and the `using-wtm` references.

## 5. Verify

```bash
cd site && npm test && npm run build    # sync + build; internal links and anchors are validated
```

Go tests also read the docs: `internal/surface/cli/agents/events_doc_test.go` requires every final `wtm events` exit code to be named in `docs/guide/events.md`, `docs/guide/recipes.md` and the `using-wtm` skill. A fact a test pins to several pages stays on each of them; run `go test ./internal/surface/cli/agents/` (or `make test`) after editing those pages.

A broken anchor fails the build: renaming a heading breaks every `#anchor` pointing at it, so `grep` the old slug first. Then look at the page in `make site-dev` (sidebar placement, lede, how code and tables render). When the change also touched the landing or the theme, take screenshots with the `wtm-site` skill.

## Auditing the guide

When asked to review, tidy or "faire un tour" of the docs, read [`references/audit.md`](references/audit.md) and follow its checklist: shape and structure, but first whether what the guide says is true of the code. It produces a findings list first; fix only what the user agrees to, since an audit often surfaces product questions (a section to add, a page to merge) that are theirs to decide.
