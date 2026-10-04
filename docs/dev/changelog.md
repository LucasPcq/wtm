# Writing the changelog

`CHANGELOG.md` is written in **English**, in the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) shape, and follows [semver](https://semver.org). Each release section is also published verbatim as the GitHub release notes (`make release-notes`, run by the release workflow), so it is the first thing a new user reads about a version: write it for them, not for the reviewer of the PR.

## Template

Copy this under `## [Unreleased]` and keep only the sections that have entries, in this order.

```markdown
## [0.30.0] - 2026-11-02

One sentence on what this release is about, for someone deciding whether to upgrade.

### Highlights

- **`wtm events`**: stream every worktree change as JSON Lines, for editors, terminal plugins and agents. → [Event stream](docs/guide/events.md)

### Breaking

- **`wtm create --output json`** answers with an envelope: read `.results[0].path` instead of `.path`. → [Migrating to 0.29](docs/guide/migrating-to-0.29.md)

### Added

- **`wtm exec`** runs one command in several worktrees, in parallel, each with its own environment.

### Changed

- **`wtm ui`** picks up worktrees created elsewhere immediately instead of on its 20 s refresh.

### Fixed

- **`wtm relocate --to`** rewrites `base_path` even when no worktree has to move.

### Removed

- **`wtm pr`**: use `wtm checkout`.
```

## Rules

- **Curate, don't inventory.** A reader skims a release in thirty seconds to decide whether to upgrade: list what they would notice — a new command or flag, a behaviour that changed under them, a bug they may have hit. Wizard wording, alignment, message tweaks and small consistency fixes are left out, or folded into one closing bullet per area (`**`wtm env`**: clearer report, warnings on stderr, stricter flag checks.`). A release with more than ~15 bullets is an inventory: cut.
- **Short.** Aim for 20 words a bullet; the guide link carries the rest.
- **One bullet, one line, one change.** Bold the command, flag or file it is about, then say what the user gets, in the present tense. No "now", no "we", no internal names (packages, tickets, PR numbers).
- **Effect, not mechanism.** "`clean` refuses a locked worktree unless `--force`", not how the lock is detected. The detail belongs in the guide: end the bullet with `→ [Page](docs/guide/…)` when there is one.
- **Breaking is always its own section**, and every entry says what to do instead. A change that needs more than one line of instructions gets a `docs/guide/migrating-to-<version>.md` page, linked from the bullet.
- **Highlights** is optional: one to three bullets for a release with a headline feature. A bullet listed there is not repeated under Added.
- **Internal-only changes are left out** (refactors, lint rules, tests) unless they change behaviour a user can see.
- **Section titles are fixed**: `Highlights`, `Breaking`, `Added`, `Changed`, `Fixed`, `Removed`. The release heading is `## [x.y.z] - YYYY-MM-DD`, with no title after it: the summary sentence carries the theme.
- Link references at the bottom of the file (`[0.29.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.29.0`) keep the headings clickable; add one per release.
