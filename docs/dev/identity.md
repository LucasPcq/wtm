# Identity

How wtm looks and speaks outside the terminal: the landing page, the documentation site, the README, the social preview, launch posts. Read it before designing or changing any of them. The terminal itself follows [output.md](output.md); this page takes its colours from the same source so the two read as one product.

The reference prototype is the **"Native · one ground, shapes not colours"** artboard of the [identity canvas](https://claude.ai/artifact/H9Ww2NtvZwGbDpk4j3BfyM) (private to the maintainer).

## Positioning

- **The sentence:** *Every branch, running.* It is the headline, the repository description and the opening of every post.
- **The reader:** a developer who runs several coding agents in parallel. The first screen is written for them; the team argument comes lower on the page.
- **The story is the life of a branch:** created ready to code (`create`), running with its own stack (`run up`), kept in order (`sync`), gone once merged (`clean`, `prune`). `run` is the peak of that arc, told through the agent that starts the whole app in its worktree and checks its own work.
- **The team argument:** configured once, run by everyone. One person writes `run.toml` and its scripts, the team imports them. It answers the "there is config" objection.
- **What wtm is not:** a provider catalogue. It knows nothing about Postgres or Keycloak; it gives each worktree an identity and runs the user's scripts. Say so plainly: explicit, no magic, any stack.

### Voice

Factual engineer. Short concrete sentences, every promise shown by real output. No superlatives and none of the words every competitor uses: *blazing*, *10x*, *fleet*, *orchestrate*, *zero-config*, *for humans and agents*, *first*, *only*. Several tools cover parts of per-worktree runtime; wtm's claim is the combination, never exclusivity.

## Colour

The palette is the CLI's own: `internal/styles/colors.go` is the source, and a change there is a change here. Every colour has one role and appears only in that role.

| Token | Dark | Light | Role |
| -- | -- | -- | -- |
| `signature` | `#E8734A` | `#A6391A` | The wordmark, the second line of the headline, the primary button. Nothing else. |
| `primary` | `#9B8CFF` | `#5B4BE0` | Command labels above section titles, links, navigation state. |
| `success` | `#4ADE80` | `#158A4A` | `✓` in replayed output. |
| `warning` | `#F0C000` | `#B07800` | `⚠` and `~` in replayed output. |
| `danger` | `#FA6E76` | `#C21E28` | `✗` and error lines (the hero's collision). |
| `bg` | `#14121B` | `#FBFAFD` | The single page ground. |
| `panel` | `#1A1824` | `#FFFFFF` | Figures and cards. |
| `raised` | `#201D2C` | `#F5F3FA` | Elements inside a panel (windows, the copy button). |
| `code` | `#0F0D16` | `#F5F3FA` | Terminal panels. |
| `line` | `#2A2638` | `#E3DFEC` | Hairlines between sections, borders. |
| `strong` | `#F4F1FA` | `#17142A` | Headings. |
| `body` | `#C4BFD4` | `#4A4560` | Paragraphs. |
| `muted` | `#9D97B2` | `#635D7A` | Secondary text. |
| `faint` | `#6F6987` | `#857F9C` | Chrome only (prompts, comments in output, footer): below 4.5:1, never for text that must be read. |

**Branch lanes.** Three branches are drawn in three colours: `#E8734A` (signature), `#9B8CFF` (primary), `#5FD4C0` (mint; light `#D0582F`, `#5B4BE0`, `#159C88`). They appear only where a branch is the subject: the logo, the hero's lines and windows, the left or top edge of a worktree card. Replayed output stays neutral; `wtm tree` is not coloured per branch.

**Rules**

- One ground for the whole page. Sections are separated by a `line` hairline and by their shape, never by alternating background tints or colour bands.
- No glow, halo, radial gradient, blur or grain behind anything.
- The only saturated surface is the primary button. The closing call to action sits on the same ground as the rest.
- Colour never carries state alone: every state also has its glyph or its word.

## Type

| Face | Use | Settings |
| -- | -- | -- |
| **Archivo** (variable, `wdth` 62–125) | Headlines, body, UI | Body 16–20 px at weight 400–500 |
| **Martian Mono** | Commands, replayed output, URLs, labels | 11.5–14.5 px |

Both are OFL fonts, served by Google Fonts (the privacy page says so).

- **The headline pair:** the first line light and condensed (weight 300, `font-stretch: 70%`), the second heavy and expanded (weight 850, 110–118%), in `signature` on the hero. Section titles reuse the pair when they are two clauses ("Its own stack. Its own URL."), otherwise weight 800 at 80% width. A two-clause title never wraps mid-clause: each clause is `nowrap`.
- **Labels are commands:** the line above a section title is the command it is about (`wtm run up`, `run.toml`), in Martian Mono and `primary`. No all-caps eyebrows, no numbered labels.
- Ports and counts use `font-variant-numeric: tabular-nums`.

## Logo

A dot that splits into three lanes: one branch, three worktrees. The dot is `strong`, the lanes are the three branch colours, the wordmark `wtm` sits beside it in Archivo 800 at 112% width, in `signature`.

```xml
<svg width="30" height="24" viewBox="0 0 30 24">
  <circle cx="4" cy="12" r="3" fill="#F4F1FA"/>
  <path d="M6 12 C14 12 14 4 22 4 H27" fill="none" stroke="#E8734A" stroke-width="3" stroke-linecap="round"/>
  <path d="M6 12 H27" fill="none" stroke="#9B8CFF" stroke-width="3" stroke-linecap="round"/>
  <path d="M6 12 C14 12 14 20 22 20 H27" fill="none" stroke="#5FD4C0" stroke-width="3" stroke-linecap="round"/>
</svg>
```

On the light theme the dot takes `#17142A` and the lanes their light values. The mark alone (no wordmark) is the favicon.

## Layout

Content width 1240 px with 32 px gutters; at phone width every row stacks and wide panels scroll inside their box.

**The hero** is a conventional tool hero: navigation (mark, version, Docs, Guide, Changelog, GitHub), then the headline, one sentence that names the product ("A git worktree manager that…"), and the action row right under it: the install command with a copy button, then *Get started*. The figure sits to the right, contained in a panel.

**The hero figure: collision, then calm.** Three lanes braid into one window at `localhost:3000` that lists the three real errors (`EADDRINUSE :::3000`, the Compose container name in use, `DATABASE_URL` not found). The errors dim, the lanes split, and three windows arrive, each with its URL, who runs it (`you`, `claude`, `codex`), its branch and its port.

**Sections vary by shape, not by colour.** Consecutive sections never share a layout. The shapes in use:

- a horizontal timeline (the life of a branch);
- a tall section with a centred title and a full-width figure below (`run up` and the three worktree identity cards);
- two equal columns, text beside a terminal (`run.toml`);
- a heading alone, three short columns, then a full-width terminal (agents and `wtm events`);
- two equal columns with panels of equal height (`sync`, `clean · prune`): label, title, text and panel share a row across the columns (CSS subgrid), the two texts are kept to the same number of lines, and a line too long for the half-width panel wraps with a hanging indent, as a terminal of that width would;
- a centred closing block (try it: `brew install`, `wtm init`, `wtm create`).

A terminal is never smaller than the text beside it; when the output is long, it goes full width under the text.

## Motion

One sequence, played once on load, with a *replay* control: errors visible for about 1.9 s, errors dim, lanes cross-fade to the split (700 ms), windows enter 120 ms apart (600 ms, ease-out, no bounce). Only `opacity` and `transform` animate. With `prefers-reduced-motion`, the page shows the final frame. Nothing else on the page moves: no pulsing dot, no typing effect, no marquee.

## Showing the product

Every terminal on the page replays output wtm really printed, captured with the `wtm-sandbox` skill in a repository called `acme`, trimmed to fit. Terminal panels are plain `code` boxes with a hairline border: no traffic-light dots, no fake title bar, no typing animation. Names in figures (Compose projects, databases, ports) are the ones wtm produces, never invented.

## Anti-patterns

The look every agent-tool landing already has, and the generated-design tells that read as "AI made this": near-black with a single glowing accent; Inter, Geist, Roboto or Space Grotesk; a halo, spotlight or blur behind the hero; gradient text; grid-line or graph-paper backgrounds; glassmorphism; a badge above the headline; tiny numbered section labels; bento and identical card grids; hard offset shadows; alternating section tints or a rainbow of colour bands; a cream page with an italic serif headline; a pulsing status dot; a wall of agent logos.

## Applying it

| Surface | Where | What it takes |
| -- | -- | -- |
| Landing | `site/src/pages/index.astro`, `site/src/components/landing/`, `site/src/styles/landing.css` | Applied. Hero, life of a branch, `run up` cards (values from `machine.mjs`), `run.toml`, agents, `sync` and `clean · prune`, try it |
| Docs theme | `site/src/styles/theme.css`, `site/astro.config.mjs` | Applied. The tokens (both themes) on Starlight's `--sl-color-*`, Archivo and Martian Mono, the mark as logo, titles in the condensed heavy cut, inline code a step smaller than the text |
| README | `README.md`, `docs/assets/banner-*.svg` | Applied. The banner (text outlined from Archivo, a dark and a light file behind `<picture>`), one section shape repeated for the four stages, visuals recorded with the wtm VHS theme (`docs/demos/_env.tape`), no command table |
| Social preview | `site/public/social-preview.png` (1280×640), uploaded by hand in the repository settings | Applied. `bg`, the mark and wordmark, the headline pair, one sentence, the three lanes splitting to the right edge with their URLs. It is the `og:image` / `twitter:image` of every page of the site |
| Search | `site/src/layouts/Landing.astro`, `site/src/pages/robots.txt.ts`, `site/astro.config.mjs` | Applied. A title with "git worktree manager", a description under 160 characters, a canonical per page, Open Graph and Twitter tags, `SoftwareApplication` JSON-LD, `robots.txt` pointing at the sitemap, `theme-color`, the touch icon |
| Favicon | `site/public/favicon.svg` | Applied. The mark on `bg` |

The banner and the social preview are generated by the scripts in [`docs/assets/src/`](../assets/src/README.md), which outline their text from the two fonts.
