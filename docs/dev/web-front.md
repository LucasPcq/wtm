# The web front

`wtm-next ui` serves a page that drives wtm from the browser: the web front, a React app in `next/web/`, built by Vite into `next/internal/web/dist/` and embedded in the binary (`go:embed`, `next/internal/web/front.go`). It speaks to the core through the server's API only (`next/README.md`, "The web server"): it reads, asks and runs commands of the catalogue, and it decides nothing the core decides.

## The stack

| Concern | Tool |
| -- | -- |
| Build and dev server | Vite, React, TypeScript 7 (`strict`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`); Node pinned in `next/web/.nvmrc` |
| Components and styles | shadcn/ui (`radix-nova`, neutral base, lucide icons) on Tailwind v4, added only through the shadcn CLI or MCP (`npx shadcn@latest add <name>`) so that `src/components/ui/` stays upstream's code |
| Routing | TanStack Router, file-based (`src/routes/`, `routeTree.gen.ts` generated): a repository's screens live under `/r/$repo/…`, `$repo` being the id the server gives it (`web.RepoID`) |
| Server state | TanStack Query: every read, mutation and stream goes through its cache (`src/api/queries/`) |
| Forms | TanStack Form over the form a command's questions return (`src/components/command-form/`) |
| Types | generated from `next/internal/api/api.v2.schema.json` by json-schema-to-typescript into `src/api/types.gen.ts` (`npm run types`, run before `dev` and `build`): the catalogue's shapes, the names of its commands (`ApiCommandName`) and the server's own (`WebRepo`, `WebVersion`, `WebCall`, `WebRefusal`…); the front types nothing by hand |
| Event stream | `eventsource-parser` (`EventSourceParserStream`) inside TanStack Query's `streamedQuery` |
| Lint, format and tests | oxlint and `prettier --check` (part of `npm run build`, a warning or an unformatted file fails it; `npm run format` writes), `tsc -b`, Vitest over `src/**/*.test.ts` (`npm test`): the non-trivial logic (the token, the event stream, the questions-backed validator), no component test yet |

## Where code goes

```
src/
  api/                  the server: fetch.ts (the token header, json), token.ts, types.gen.ts, and queries/<resource>.ts,
                        one file per resource the server answers (version, repos, commands, events), its queryOptions
  components/ui/        shadcn/ui, upstream's code: added by the CLI, never edited
  components/           what two features share: the page header, the state pages, the logo, the command form
                        (command-form/: a field per kind, the form hook, the answers and the core's problems)
  features/<feature>/   one area of the app: its components, its hooks, the queries only it reads
                        (shell/: the frame, the sidebar, the gates; worktrees/: the Worktrees screen)
  routes/               TanStack's file routes, thin: a route declares its path and renders its feature
  lib/                  pure helpers, no React, no fetch
```

- **A feature owns its area**: a screen or a frame is a folder of `features/`, everything it alone uses inside it. Code moves to `components/` (UI) or `lib/` (no UI) on the second feature that needs it, not before.
- **A route is a path and a feature.** It reads its params and renders the feature's component; no query, no markup of its own.
- **Server state is `api/`'s.** A component reads with `useQuery(xQuery)` and writes with `useMutation` over `api/queries/` (or its feature's queries), never `fetch` itself.
- **A closed set of looks is a type.** A component takes the state (`Connection`, a union) and derives its classes (`cva` variants, a `Record` over the union), never a free class string.
- **A file is named after what it declares**, kebab-case (`connection-footer.tsx` declares `ConnectionFooter`); a hook's file starts with `use-`.

## The look

The visual spec is the Pencil file `wtm-ui.pen` (the `pencil` MCP; open in the Pen app). It holds shadcn's default kit and wtm's own components built on it: `wtm/Logo`, `wtm/Sidebar` (with its `wtm/Sidebar Footer`), `wtm/Page Header`, `wtm/Empty State`, and one frame per screen and state. The code follows it: a screen is drawn in Pencil first, from these components, validated by the owner, then built; a deviation found while coding goes back into Pencil in the same change.

The identity is shadcn's defaults plus two marks of `docs/dev/identity.md`: the logo (the mark's SVG, its lanes in their light and dark values, the wordmark in Inter 700) and `signature` as `--primary` (`#A6391A` light, `#E8734A` dark, its text white in light and `#17142A` in dark, for contrast). The font is Inter. The theme follows the system's (`prefers-color-scheme`, the `.dark` class set before the first paint in `index.html`), with no toggle.

## How it talks to the server

- **The token.** `wtm-next ui` opens `http://127.0.0.1:<port>/#repo=<id>&token=<token>`. The page reads the fragment once (`src/api/token.ts`), keeps the token in `localStorage` and drops the fragment from the address. Every tab of the origin shares it: a restarted server has a new token, and the tab `wtm-next ui` then opens writes it, which the `storage` event hands to the tabs already open. Every request carries it in `X-Wtm-Token`, never in a query string. A page with no token shows how to open it. A token change reloads the page, so that a restarted server of another version serves its own code; a change of the fragment (an address pasted into the tab) reloads it too, so that its token is read and leaves the address bar.
- **The API layer** (`src/api/`) is the generated types, `fetch.ts` (`request`, fetch with the token header; `json`, its body or the server's error as an `HttpError`), and `queryOptions` per resource (`queries/`). Nothing sits between a component and Query.
- **Reads.** A read of the catalogue (`list`, `tree`, `status`) is `readQuery(repo, command, answers)`, `command` an `ApiCommandName`, its envelope cached under `['repo', id, …]`. The front never polls.
- **The event stream.** `eventsQuery` (`src/api/queries/events.ts`) is a `streamedQuery` over `GET /api/events`, read with `fetch` (`EventSource` cannot send the header) and parsed by `EventSourceParserStream`. Each line invalidates what it changed: an event of a repository `['repo', id]`, `repo.added` and `repo.removed` the list of repositories; back after a break, everything is read again, the lines in between being lost. Query reconnects it: `refetchInterval` asks again every 2 s once the stream ended or failed, and stops when the server refuses the token, until a token change reloads the page. `connectionOf` reads the query's state as live, reconnecting, connecting or disconnected, which the sidebar's footer shows with the stream's warnings (the bus lost, a repository skipped).
- **Components** hold no `useEffect`: state is derived while rendering, data and side effects go through Query, Router and Form, an outside store through `useSyncExternalStore` (the token). The only effects are in shadcn's own code (`src/components/ui/sidebar.tsx`, its keyboard shortcut; `src/hooks/use-mobile.ts`).
- **Forms.** `useCommandForm` builds a form from a command's `domain.Form` (`POST …/questions`), one value per field keyed by its path (escaped: the form library reads `remember[0]` as nested), a text list as its raw text, split when sent, and `CommandFields` draws each field with the input of its kind (select, multi-select, text, text list, confirm). Validation is the core's: each change asks the questions again with the answers changed so far, and each problem lands under its field. The front re-encodes no rule (required, allowed options, branch names).

## Building it

`make ui` is the only writer of `next/internal/web/dist/`: `npm ci`, `npm test`, `npm run build` in `next/web`. The build is committed, so that `go install` gives a binary with its interface, under these guards:

- no source maps (`build.sourcemap: false`);
- `next/internal/web/dist/** linguist-generated=true -diff` in `.gitattributes`: a review reads the sources;
- the CI job `ui` rebuilds from the lockfile with the pinned Node and fails when `dist/` differs from the commit (`git diff --exit-code`, no untracked file);
- goreleaser refuses to release a dirty tree; the release does not ship `wtm-next` yet, and the one that does runs the `ui` gate before it;
- a binary built `-tags noui` (`front_noui.go`) embeds no front and serves a page saying so, its API unchanged; `make test-next` and the CI run the server's tests in both builds.

A change to `next/web/` is committed with the `dist/` that `make ui` writes from it.

## Dev mode

`make ui-dev` serves the front from Vite with hot reload and proxies `/api` to the server of `wtm-next ui` (`WTM_API=http://127.0.0.1:<port>` when it does not listen on 4777). The proxy rewrites `Host` and `Origin` to the server's own, which its guard requires. Open Vite's address with the fragment `wtm-next ui --no-open` prints: `http://localhost:5173/#repo=<id>&token=<token>`.

## Adding a screen

1. Draw it in Pencil from the `wtm/` components; get it validated.
2. Add its feature under `src/features/<feature>/` and a thin route under `src/routes/_shell/r/$repo/`; its reads as `queryOptions` (in `src/api/queries/` when another feature reads them too), its actions as commands of the catalogue.
3. Show what the core returns: a count, a state, an anomaly or an available action arrives in the envelope, and the screen draws it.
4. `make ui`, then commit the sources with `dist/`.
