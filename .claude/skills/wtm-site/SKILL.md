---
name: wtm-site
description: Change wtm's public face without drifting from its identity — the landing page (site/src/pages/index.astro and components/landing/), the privacy and terms pages, the docs theme (site/src/styles/theme.css, Starlight), the docs home, the README product page, the banner and social preview. Covers where each value comes from (facts.mjs, machine.mjs, sandbox captures), the identity rules that matter in code (tokens, both themes, section shapes, replayed terminals), the CSS traps already hit, and before/after screenshots at desktop and phone width. Use this whenever the user asks to adjust, redesign, fix or polish the site, the landing, a section, spacing, typography, colours, the hero, the footer, the legal pages, the docs look or the README pitch — even for a "small CSS tweak", a screenshot they send of something that looks off, or "ça fait bizarre".
---

# Changing the site without losing the identity

The site is the first thing someone sees of wtm. Its identity — palette, type, section shapes, voice — is written once in [`docs/dev/identity.md`](../../../docs/dev/identity.md). Read the parts your change touches before editing: this skill does not repeat them, so the two never disagree. What it adds is how to apply them in this codebase and how to prove a change looks right, because a site change is only finished once it has been seen, at two widths, next to what it replaced.

## 1. Find what you are changing

| Surface | Files | Values from |
| -- | -- | -- |
| Landing | `site/src/pages/index.astro` (section order), `site/src/components/landing/*.astro`, `site/src/styles/landing.css`, `site/src/layouts/Landing.astro` (nav, footer, meta) | `src/lib/facts.mjs` (latest tag), `src/lib/machine.mjs` (ports, `.env`, URLs, identity cards), `src/lib/term.mjs` (terminal lines) |
| Legal pages | `site/src/pages/privacy.astro`, `terms.astro` (same layout and CSS as the landing) | what wtm really sends over the network |
| Docs theme | `site/src/styles/theme.css`, `site/astro.config.mjs` | the landing's tokens on Starlight's `--sl-*` variables |
| Docs home | `site/src/docs-home.mdx` (`__BASE__` is replaced at sync) | — |
| README, banner, social preview | `README.md`, `docs/assets/src/`, `site/public/social-preview.png` | `make demos` for the visuals |

The guide's *content* and its sidebar are not this skill: use `wtm-docs`.

## 2. Rules that bite in code

- **Values are derived, never typed.** A port, URL, Compose project or version shown on the page comes from `machine.mjs` or `facts.mjs`, which are tested (`npm test`). If the page needs a new value, add it there with a test.
- **Terminals replay real output.** Every line in a `Session` was printed by wtm in the `acme` sandbox (the `wtm-sandbox` skill), written with `cmd()` / `out()` / `note()` from `term.mjs`, trimmed but never reworded. When a command's output changes, re-capture it. A line too long for its panel wraps (`wrapped` on `Session`) or the panel goes full width; the text is not edited to fit.
- **Colour only through tokens** (`var(--primary)`…), each in its single role from the identity table. A new colour is a change to `identity.md` and `internal/styles/colors.go` first. Both themes: a token changed in `:root` is changed in the light blocks too.
- **Sections vary by shape, not by colour**, and two consecutive sections never share a layout. Moving or adding a section means checking its neighbours.
- **Paired columns stay level.** Where two columns sit side by side (`.split.top`), label, title, text and panel share grid rows (subgrid), and the two texts are written to the same number of lines at desktop width. When editing copy in one column, recount the other.
- **Copy follows the voice** in `identity.md`. Titles that are two clauses use the pair (light then heavy) with each clause `nowrap`.
- **Legal pages are factual.** Changing what wtm or the site sends (update check, fonts, hosting, analytics) means updating `privacy.astro` and its "Last updated" date in the same change.

### CSS traps already hit

- **Specificity.** `.section p` (0,1,1) beats `.label` (0,1,0): labels silently lost their margin and colour. Scope component rules at the same level as the generic one (`.section .label`), and when spacing looks wrong, check the computed style before adding more margin.
- **Starlight wraps headings** in `.sl-heading-wrapper`, which carries the font size; set `--sl-text-h2` / `--sl-text-h3` / `--sl-line-height-headings` instead of styling `h2` directly. Starlight's styles are in cascade layers, so an unlayered rule in `theme.css` wins without `!important`.
- **Short pages** keep the footer at the bottom through the body's flex column (`main { flex: 1 }`); don't give the footer a height.
- **Martian Mono is wide** (≈0.69 em per character): a 13 px line of 66 characters is ~600 px. Measure before promising that output fits a half-width panel.

## 3. See it, before and after

Build first; a broken link or anchor fails it:

```bash
cd site && npm ci && npm test && npm run build
```

In a fresh worktree, install `site/node_modules` there. Never symlink another checkout's: `npm ci` or `make site` through the link empties the original under whoever else is using it.

Then serve both versions and capture the same views. `S=.claude/skills/wtm-site/scripts`:

```bash
(cd site && node_modules/.bin/astro dev --port 4391 &)   # this branch (after)
$S/serve-ref.sh origin/main 4392                # the reference (before)
$S/shoot.sh http://localhost:4392/ /tmp/before-order.png 1440 900 'section:has(.split.top)'
$S/shoot.sh http://localhost:4391/ /tmp/after-order.png  1440 900 'section:has(.split.top)'
$S/shoot.sh http://localhost:4391/ /tmp/after-phone.png  400 800 full
$S/serve-ref.sh --stop 4392
```

`shoot.sh` reduces motion so replayed terminals and the hero show their final frame, waits for the fonts, and fails when the page scrolls sideways. Section selectors: hero `section.hero`, life of a branch `section:has(.timeline)`, run up `section:has(.cards)`, run.toml `section:has(.split:not(.top))`, agents `#agents`, sync/clean `section:has(.split.top)`, try it `section:has(.try)`, a docs page `main`.

Look at every capture yourself before reporting: spacing between label, title and text; columns level; nothing cut or scrolling; the phone width stacked with a 16 px gutter. When the user sent a screenshot of a problem, capture the same view and compare. The pairs go into the PR as proof (the `open-pr` skill).

## 4. Keep the rules current

When a change settles a new design decision (a section shape, a token, a rule for copy length), write it in `docs/dev/identity.md` in the same change, and add the trap here if it cost time to find. The next change should not rediscover it.
