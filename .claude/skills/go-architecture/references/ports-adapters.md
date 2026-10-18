# Ports, adapters, fakes and test kits

How a use case or a service reaches the world, what implements it, and what stands in for it in a test. The Go behind it is go-style `interfaces.md` (consumer-side interfaces; accept interfaces, return concrete types).

## Ports

- **Declared by the consumer** in its `port.go` — a command (`command/extract/port.go`) or a service (`service/provision/port.go`) — with only the methods it calls. Two consumers of git declare two `Git` interfaces, both satisfied by `git.Repo`.
- **One per outside thing:** `Git`, `Meta`, `EnvFiles`, `AnswerStore`, `Locker`, `Publisher`. Its doc comment states the contract a fake keeps too (`Meta.Load` "fails with an error wrapping fs.ErrNotExist when the branch has none").
- **A single function is a function type** in `port.go` (`extract.Shield`), or a func field of a service (`provision.Service.RunHooks`, `Now`, `LeftBehind`); the wiring passes the adapter's function (`shell.Shield`, `time.Now`).
- **`port.go` declares nothing else** than interface and function types: the types a port takes and returns sit beside the code that owns them. `[archlint]` (`portfile`)
- **A service is not a port:** a use case holds `repository.Service`, `operation.Service`, `provision.Service` as concrete fields; a test builds them over fakes.
- **A port speaks the domain:** domain values in and out (`worktree.Snapshot`, `worktree.Move`), never a file format or an adapter's type.

## Adapters

- **One package per outside thing:** `git`, `store` (meta.json, the managed .env files), `config` (config.toml, run.toml, global settings: their shapes, strict decode, JSON schema), `shell` (child processes, hooks, the shield), `oplock`, `events` (the events socket), `proxyport`, and `statefile` (locked atomic writes).
- **It exports a concrete type** (`git.Repo{Dir}`, `store.Meta{StateDir}`) whose methods satisfy every consumer's port; the compiler checks the fit in `wire.go`.
- **It imports `domain/*`, the stdlib, third parties, `adapter/statefile` and `adapter/shell`** (the one way to start a child, in its own process group) — never a service, a command, `ui/*` or another adapter. `[archlint]`
- **The file format lives in the adapter:** `adapter/config` owns every key of `config.toml` and `run.toml` and maps them onto `domain/project`. A state file is written through `statefile.Update` (lock, read, decide a change, atomic rename), carries `schema` and refuses a newer one (exit 22).
- **Tested against the real thing:** real git in a temporary repository (`gittest.Build`), real files in `t.TempDir()`.

## The wiring

`cmd/wtm-next/wire.go` builds each adapter once per repository and injects it into every use case and service of `app`: the only package that knows which adapter backs which port, the only one that imports adapters, never a fake. A method that does not fit a port exactly is adapted by a small type there (`locker` wraps `oplock.Locks`). `[archlint]`

## Fakes: `adapter/<name>test`

The in-memory twin of an adapter, for its consumers' tests: `gittest.Fake`, `storetest.Fake`.

- **It keeps the adapter's contract**, not only its signatures: the same refusals and results (a write that changes nothing reports no change, a newer schema is refused, a failed add drops the branch it created).
- **One shape:** seeded by `NewFake(FakeParams{…})`; any method made to fail through `FakeParams.Fail` (by method name); `Calls()` and readers (`Meta`, `File`, `Config`, `Moves`) give back what happened; safe for concurrent use.
- **Parity:** the adapter's tests run each case against the adapter and the fake (`adapter/git/fixture_test.go`: the `port` both implement, `implementations(t)`), and `gittest.Build` builds the real repository from the same `FakeParams` as `NewFake`. A method added to the adapter is added to the fake and to the parity test.
- **Placement:** beside its adapter, importing what the adapter may plus the adapter; imported from `_test.go` files only. A name ending in `test` under `adapter/`, `service/` or `ui/` is reserved for the kit of the package beside it. `[archlint]`

## Test kits: `service/<name>test`, `ui/<name>test`

- `service/<name>test` holds a promise a service makes every consumer, as a check the consumer runs: `operationtest.Contract(t, subject)` puts a command's `Run` through a dry run (no lock, no event), a stale plan (refused before any event), an apply (events in order, each unit finished, locks released) and a correlation ID. Each command's `contract_test.go` calls it.
- `ui/<name>test` holds helpers the commands' `cli` tests share: `ui/formtest` (the wizard driver, `Golden`, the envelope decoded; `form.md`). `[archlint]`
- A kit arrives with its second user; until then a helper stays in the package's `helpers_test.go`.

## Adding a port or an adapter

1. The port in the consumer's `port.go`, each method with its contract.
2. The methods on the adapter of that outside thing (a new adapter only for a new thing), returning domain values, tested against the real thing.
3. The same on the fake, keeping the contract, with the case added to the parity test.
4. The wiring in `wire.go`; the compiler checks the fit.
5. The consumer's tests seed the fake in `fixture_test.go`.
