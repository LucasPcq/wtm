# The lint: Biome and its GritQL rules

One tool, Biome (pinned exactly in `package.json`), formats, organises imports and lints `next/web`. Its config is `biome.json`; the rules no built-in can say are GritQL plugins in `lint/*.grit`.

## Running it

| Command | What it does |
| -- | -- |
| `npm run lint` | `biome check --error-on-warnings .`: format, imports, lint; any finding fails |
| `npm run format` | `biome check --write .`: applies the safe fixes (format, import order) |
| `npm test` | includes `lint/rules.test.ts`, the guard of the rules (below) |
| `make lint-ui` | the types, the route tree, `tsc -b`, then `npm run lint` |

`npm run build`, `make ui` and the CI `ui` gate run the lint too. When the RTK hook rewrites `npx biome …`, call `./node_modules/.bin/biome` directly.

Out of the lint: `src/components/ui/` (shadcn's code), `src/hooks/use-mobile.ts` (shadcn's, it holds an effect), `*.gen.ts`, `lint/fixtures/` (they violate on purpose), what `.gitignore` ignores.

## The formatter

Single quotes, no semicolons, 120 columns, spaces (Prettier's old output, unchanged). CSS needs `css.parser.tailwindDirectives` for Tailwind's `@theme`/`@apply`.

## The rules, and why

| Rule | Refuses | Why |
| -- | -- | -- |
| `style/noRestrictedImports` (paths) | `useEffect`, `useLayoutEffect`, `useInsertionEffect` from `react`, also re-exported; `renderHook` | no effects: derive while rendering, Query/Router/Form, a ref callback; a hook is tested through the page |
| `style/noRestrictedImports` (patterns) | `../` imports | a parent-relative path hides the layer it lands in, and Biome cannot resolve it: `@/` names it whole |
| `nursery/noJsRestrictedProperties` | `X.useEffect` (and the two others) on any object, including destructuring; `window`/`globalThis`/`self` `.fetch`, `.EventSource`, `.XMLHttpRequest`, `.WebSocket`; `navigator.sendBeacon`; `vi.mock`, `vi.doMock`, `vi.stubGlobal` | the member spellings of the two rules around it; a module or a global mocked replaces the app under test |
| `style/noRestrictedGlobals` | bare `fetch`, `EventSource`, `XMLHttpRequest`, `WebSocket` | the server is reached through `src/api/` only |
| `style/noNestedTernary` | `a ? b : c ? d : e` | a named function with early returns reads in one pass |
| `style/noExcessiveLinesPerFile` (300) | a file past 300 lines | split it by what it draws |
| `correctness/useHookAtTopLevel` | a hook called in a branch or a loop | React's rules of hooks |
| the `recommended` preset | a11y (a label tied to its control by `htmlFor`/`id`), suspicious and correctness rules | |
| plugin `layers` (`lint/layers.grit`) | an import outside its layer's allowlist; a path with `..` inside it (`@/a/../b`, `./../b`); `vitest`, `msw`, Testing Library or `@/test/` outside tests and `src/test/`; `eventsource-parser` outside `src/api/` | the layers of the SKILL ("Where code goes") |
| plugin `unknown-folder` | a file of `src/` in a folder that is no layer, or at the root other than `main.tsx` and `routeTree.gen.ts` | a new layer is a decision, made in this config and the skill |
| plugin `props` | a component (a capitalised function) taking `props` whole | destructured props say what it uses |
| plugin `spy-module` (tests only) | `vi.spyOn` on a namespace import of the app | it replaces a module: mock the server with msw |

The `src/api/` override turns the network rules off there. **An override replaces a rule's options instead of merging them**: the override repeats the whole `noJsRestrictedProperties` list minus the network entries. Change one, change both.

Prefer a precise entry (`{ "object": "window", "property": "fetch" }`) to a bare `{ "property": "fetch" }`: the bare one also refuses `client.fetchQuery`-like members of our own objects.

**"Few comments" is a review rule**: no tool tells a why from a restatement.

## The guard: fixtures

A GritQL plugin that fails at runtime (a node name that changed, a typo) prints an info line and lets everything through: the lint goes green on everything. So every rule has planted violations in `lint/fixtures/src/` (the same layout as `src/`), each file's first line naming the rules that must refuse it:

```ts
// expect: plugin/layers
import { Glyph } from '@/features/b/glyph'
```

`// expect: none` marks a file that must pass (the clean cases matter as much: a rule that refuses too much is broken too). Built-in rules are named by their category without `lint/` (`style/noNestedTernary`); a plugin by `plugin/<name>`, read from the `(web-front skill, <name>)` tag that ends each plugin's message. Keep that tag in every new plugin message.

`lint/rules.test.ts` writes the build's `biome.json` at the root of the fixtures (same rules and overrides, only its paths moved, deleted after), runs Biome there, and checks each fixture is refused by exactly the rules it names. It runs in `npm test`, so a broken plugin fails CI.

## Adding or changing a rule

1. Plant the violation first: a fixture whose first line names the rule, and a clean neighbour if the rule could over-reach. Run `npx vitest run lint` and see it fail.
2. Write the rule: a built-in in `biome.json` (both the root and the `src/api/` override when it is `noJsRestrictedProperties`), or a `.grit` file listed in `plugins`. A GritQL message ends with `(web-front skill, <name>)`.
3. Run the test again, then `npm run lint` on the app: the app's own code must stay green.
4. Say it in this file's table, with its why.
