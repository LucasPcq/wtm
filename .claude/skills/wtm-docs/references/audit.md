# Auditing the guide

An audit answers one question: does the guide still read as one product, written by one person, for one reader? Features land one at a time; each is documented where it was convenient that day. The audit finds where that drifted and proposes how to bring it back. It reports before it edits, because half of what it finds are decisions (merge two pages, add a section) that belong to the user.

## 1. Mechanical pass

```bash
node .claude/skills/wtm-docs/scripts/check-guide.mjs
```

It flags what a script can see: a title with a colon subtitle, a page that opens on code or a heading instead of a lede, a page absent from the index (so absent from the sidebar), a hand-written table of contents, a page over 300 lines, hard-wrapped prose, and index links to missing files. Each finding is a fix, not a question.

## 2. Reading pass

Read the index (`docs/guide/README.md`), then every page's title and lede only, in sidebar order. Then read the pages themselves. Note, per page:

- **Facts.** Check every claim a reader would act on against the code, not against another page: flags and defaults (`docs/wtm_*.md`, `internal/domain/constants.go`), file formats and paths (`internal/config/`, `internal/infra/`), the order and rules a page states (task before service, what `--yes` implies), and every pipeline or command shown. A command that looks plausible can be wrong as written: `wtm run export | wtm run import -` re-imports into the same clone and needs `--yes` without a TTY; "everything is plain TOML" overlooks `meta.json`. Run the doubtful ones in the `wtm-sandbox`. A wrong fact outranks every other finding.
- **Scope.** Does the lede describe what the page actually covers? A page whose content outgrew its lede either needs a new lede or a split.
- **Overlap.** Is a concept explained in two places? Keep the fuller one as the home; the other links to it. Typical pairs: `how-run-works` and `jobs-and-profiles`, `configuration` and `state`, `integrations` and `events`.
- **Placement.** Is the page in the section a reader would look in? Is something in the guide that belongs in `--help` (one flag's behaviour), in `docs/dev/` (how it is built), or in the changelog (what changed)?
- **Order.** Within a page: common case, then variations, then edge cases. Within a section: concept before reference.
- **Examples.** Do they still match what wtm prints? Spot-check the ones on pages whose commands changed recently (`git log --since=… -- internal/surface/cli/ internal/flow/`) by running them in the `wtm-sandbox`. Repository and branch names consistent (`acme`, `feat/login`)?
- **Voice.** Against `docs/dev/identity.md` "Voice": superlatives, marketing words, "we", "simply", "just", history ("since 0.28", "previously") in a concept page.
- **Links.** Concepts mentioned without a link to their home; links to a page where an anchor would land the reader on the right section.
- **Balance.** A section with one page, or one page carrying most of a section, hints that the sections no longer match the content.

## 3. Report

Group the findings so the user can decide quickly:

1. **Errors** — facts the guide gets wrong, each with the evidence (file and line in the code, or the sandbox output). First, because a reader acting on them fails.
2. **Fixes** — mechanical findings and clear-cut issues (a missing lede, a stale example, a broken voice). Proposed as a batch.
3. **Decisions** — structural changes, each with your recommendation and its cost: "merge `state` into `configuration` (one fewer page, `configuration` grows to ~200 lines)". One line each, recommendation first.
4. **Content gaps** — something a user needs that no page covers (often visible from recent features in `CHANGELOG.md` without a `→` guide link).

Then apply what the user accepts, rerun the mechanical pass and `cd site && npm run build`, and look at the sidebar in `make site-dev`.
