# The web front

`wtm-next ui` serves a page that drives wtm from the browser: the web front, a React app in `next/web/`, built by Vite into `next/internal/web/dist/` and embedded in the binary (`go:embed`, `next/internal/web/front.go`). It speaks to the core through the server's API only (`next/README.md`, "The web server").

How front code is written — its layers, components, state, tests, the server and the stream, and the lint — is the `web-front` skill (`.claude/skills/web-front/`). Read it before changing anything under `next/web/`. Its design lives in the Pencil file `wtm-ui.pen` (the `pencil` MCP).

## Building it

`make ui` is the only writer of `next/internal/web/dist/`: `npm ci`, `npm test`, `npm run build` in `next/web`. The build generates the types from `next/internal/api/api.v2.schema.json` (`npm run types`) and the route tree (`tsr generate`), type-checks (`tsc -b`), builds, then runs the lint (`npm run lint`, Biome). A type error leaves `dist/` as it was; a lint error fails the build. `make lint-ui` runs the types and the lint alone.

The build is committed, so that `go install` gives a binary with its interface, under these guards:

- no source maps (`build.sourcemap: false`);
- `next/internal/web/dist/** linguist-generated=true -diff` in `.gitattributes`: a review reads the sources;
- the CI job `ui` runs `make ui` from the lockfile with the Node of `next/web/.nvmrc` and fails when `dist/` differs from the commit (`git diff --exit-code`, no untracked file);
- goreleaser refuses to release a dirty tree; the release does not ship `wtm-next` yet, and the one that does runs the `ui` gate before it;
- a binary built `-tags noui` (`front_noui.go`) embeds no front and serves a page saying so, its API unchanged; `make test-next` and the CI run the server's tests in both builds.

A change to `next/web/` is committed with the `dist/` that `make ui` writes from it.

## Dev mode

`make ui-dev` serves the front from Vite with hot reload and proxies `/api` to the server of `wtm-next ui` (`WTM_API=http://127.0.0.1:<port>` when it does not listen on 4777). The proxy rewrites `Host` and `Origin` to the server's own, which its guard requires. Open Vite's address with the fragment `wtm-next ui --no-open` prints: `http://localhost:5173/#repo=<id>&token=<token>`. In a sandbox, stop the server with `POST /api/stop` and its token, or let `--idle` expire it.
