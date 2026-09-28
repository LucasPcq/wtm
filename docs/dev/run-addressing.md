# Named URLs — the vocabulary

Three different things were being called "the proxy": the server that makes names
resolve, the OS rule that removes a port from those names, and the port either of them
happens to use. They are separate mechanisms with separate failure modes, and confusing
them makes every message about them unreadable. This page fixes the words. Code,
constants and CLI copy use these and no synonyms.

## The seven terms

| Term | What it is | Where it lives |
| -- | -- | -- |
| **the proxy** | The reverse proxy the daemon runs. It listens on one port and routes by `Host` header to the job answering under that name. **This is what makes names exist at all.** On by default; `[proxy] enabled = false` in the global config switches it off | `internal/service/proxy` |
| **bind port** | What the proxy actually listens on: `11080`, or the next free port when that one is taken (the R7 fallback) | `rules.ProxyPort`, then the daemon's own fallback |
| **privileged redirection** | The OS rule `wtm run proxy install` installs — on macOS a launchd agent binding `:80` and relaying to the bind port. **It only removes the port suffix from a URL.** It unlocks nothing | `internal/service/proxy/redirect*.go` |
| **public port** | What a URL *announces*, as opposed to what anything binds: nothing (i.e. `:80`) when the redirection is live, the bind port otherwise | `rules.PublicPort` |
| **route host** | `<job>.<worktree>.<project>.localhost` — the name a published job answers under | `rules.RouteHost` |
| **origin** | `scheme://host[:port]` — the thing a browser sends in `Origin:` and CORS compares. The port is part of an origin; it is **not** part of a cookie's origin | `rules.JobOrigin` |
| **addressing** | `ports` or `names`: which of the two an `[[env_port]]` link writes into a `.env` value | `rules.EffectiveAddressing` |

## What follows from the distinction

**The proxy makes names work. The redirection makes them pretty.** `*.localhost`
resolves to `127.0.0.1` natively (RFC 6761), but resolving is not answering — without
the proxy listening, a name gives `connection refused`. Without the redirection, the
same name works perfectly, carrying `:11080`.

| State | What a `.env` holds | Works? |
| -- | -- | -- |
| proxy on, redirection **not** installed | `http://api-dev.feat-x.monorepo.localhost:11080` | yes |
| proxy on, redirection installed | `http://api-dev.feat-x.monorepo.localhost` | yes |
| proxy off (`enabled = false`) | `http://localhost:4011` | yes, without names |

**A port in the URL costs nothing that matters.** CORS compares whole origin strings,
so as long as both sides carry the same one — port included — it passes. And cookie
isolation, the reason named URLs exist, keys on the **host** and ignores the port
entirely: `feat-x` and `feat-y` are separate jars whether or not `:11080` is there.

**Addressing is a project setting, the proxy is a machine setting.** `run.toml` says
what the project wants; the global config says what this machine can do. When
the machine cannot honour it — proxy off, no public port — ports are written and a
notice says so. Writing a name nothing serves would produce a syntactically perfect,
dead value.

## Addressing in `run.toml`

```toml
addressing = "names"   # the default — omit it
```

Under `names`, an `[[env_port]]` link writes the job's full origin instead of its port
number, but **only when two conditions hold at once**:

1. the job it names **publishes a url**, and that url publishes the very port the link
   follows;
2. the value in the `.env` **has the shape of a URL**.

Both are load-bearing. The first excludes Postgres — `DATABASE_URL → docker-compose.POSTGRES_PORT`
has no name and never will, since the proxy only speaks HTTP. The second excludes the
binding keys: `apps/api/.env: PORT → api-dev.PORT` names a published job and must stay
a bare number. Neither condition alone is enough.

```
apps/api/.env   PORT=4001         → 4011                                         (bare number)
apps/web/.env   VITE_API_URL      → http://api-dev.feat-x.monorepo.localhost:11080
                CORS_ORIGIN       → http://web-dev.feat-x.monorepo.localhost:11080
                DATABASE_URL      → …@localhost:5442/db                           (no name)
```

`addressing = "ports"` is the escape hatch, and it is a real inverse: a value wtm wrote
as an origin is recognised as such and rendered back to `http://localhost:<base+offset>`.

`wtm run addressing` is how it is switched (`flow/run/addressing`, `Switch`). Its two
questions are steps of one session — the mode, then whether to settle the worktrees — so the
wizard can go back from the second to the first. The second reads every worktree's plan
under the mode just picked, **before** run.toml is written: `ResolveEnvPortsParams.Addressing`
overrides the file's, and the count is cached per mode so going back does not reread every
`.env`. The settle itself is `envports.Settle`, the pass `create` and `extract` run, so the
three cannot write different values.

Which worktrees it takes is the one place the main checkout is treated apart, and the rule is
`rules.BulkSettlesMain`: a pass over every worktree brings main back to ports, and never moves
it onto names. See the next section for why main is left alone; the asymmetry is what keeps the
two commands consistent with it:

| Command | Linked worktrees | Main |
| -- | -- | -- |
| `wtm env <worktree>` | aligned on the mode | aligned on the mode — naming it is the choice |
| `wtm run addressing ports` | settled | settled: ports is the state main has without wtm |
| `wtm run addressing names` | settled | left as is, and said so — `wtm env main` is its own decision |

Without the return leg, `wtm run addressing ports` after a `wtm env main` would leave main on
names under a project that says ports, and silently: the drift warning only reads a project on
names. The condition lives in the command's choice of worktrees, not in the pass — the plan and
the drift reading stay free of any main-shaped condition, as below.

## Recognising wtm's own writing

A value already carrying a route host is recognised **structurally** — the authority
matches `<job label>.<anything>.<project label>.localhost` — rather than remembered in a
state file. One primitive then answers four questions that would otherwise each need
their own mechanism:

- a second `wtm env` recomputes the same origin, so nothing is rewritten twice;
- a `.env` copied from main or a parent carries that worktree's segment, which is
  corrected to this one;
- installing the redirection after the fact changes the public port, and the next pass
  drops the `:11080`;
- switching to `addressing = "ports"` knows which values were wtm's to undo.

A state file would be more precise on paper and worse in practice: it diverges the
moment somebody edits a `.env` by hand.

## Dev servers and the `Host` header

A named URL reaches the job with a `Host` the dev server has never seen, and some
servers refuse one they do not recognise. Two of them matter here, and they do not
need the same thing:

- **Next** refuses it unless the host is listed in `allowedDevOrigins`. `wtm run up`
  reports the missing entry when it serves a job whose directory holds a
  `next.config.*` without one (`rules.NeedsDevOrigins`, `rules.DevOriginsPattern`).
- **Vite** needs nothing. Its host check allows `localhost` and **every** `.localhost`
  subdomain outright, before `server.allowedHosts` is even consulted — verified in
  Vite 8.2.2, `isHostAllowedInternal`. There is nothing for wtm to detect or report.

Anything else is a report from a user, not a guess from us: the presence of a config
file is the signal, never a framework inferred from a command line.

## The trade the mode makes

Under `names`, **the named URL becomes the only working entrance.** Opening
`localhost:5183` directly sends an `Origin` the API no longer knows, and CORS blocks it.
`wtm run url` and `wtm run open` hand out the right link; a bookmark on the raw port
stops working. That is the cost of having two worktrees stay logged in at the same time.

## The main checkout is not a special case

The spec that introduced `names` said the main checkout stays on ports. **No code enforces
that, and none should.** Nothing in the port pass tests the ordinal, the base branch, or
whether a checkout is the main one: `worktree.List` comes from `git worktree list`, which
includes it, so `wtm env main` is accepted and writes named origins like anywhere else. The
port substitution is the identity there (offset 0); the origin rewrite is not — it replaces
an authority, which has nothing to do with the offset.

What "main stays on ports" really means is that **no command provisions it**: `create` and
`extract` write the new worktree, and there is no such event for main. That is a gap in the
lifecycle, not a guard in the code — so the answer is a warning, not a refusal.

`rules.PendingOriginRewrites` counts what a `wtm env` on a worktree would still move onto a
named origin, `rules.AddressingDriftLine`/`Lines` phrase it, `worktree.EnvPortPlanFor`
computes the plan without applying it (the same one `wtm env` writes, so the two cannot
disagree), and `flow/run/addressing` is what the run flows read. They hand the lines to the
**surface**, which renders them once where they can be seen: a band in the run view, a callout
beside a stream, nothing at all on a machine run. A notice printed after the view reaches a
reader who has already followed the URL.

Two things the count is deliberately not. It is not "the .env holds ports": a value already
carrying a named origin whose public port went stale — every worktree, the moment
`proxy install` moves 11080 to 80 — is pending too, and equally broken, which is why the
wording says *out of step* rather than naming ports. And it is not "the app is broken": wtm
sees only the keys declared as `[[env_port]]` links, and cannot know whether the app makes a
cross-origin call at all. The reading names
no worktree in particular: a linked worktree whose port pass was declined is in the same
state, and main is only the one that is there by construction.

Do not add a main-shaped condition here.

**The address a surface hands out follows the `.env`, not the setting.** While the file spells
ports, the port *is* the working entrance — both sides of a cross-origin call agree on it —
and the published name is the one nothing behind it answers on. So `rules.AddressedByPort`
zeroes the public port for that worktree and `rules.JobURL` falls back to
`http://localhost:<port>`, in the board, in `run url`, in the start sequence's line and in the
dashboard's loader alike. The route is still registered with the proxy: settling the file with
`wtm env` is enough to get the name back, with nothing to restart.

That is deliberately not "hide what is broken". It is the same reading in both directions —
what the `.env` says is where the app answers — which is why `AddressedByPort` looks at the
value the file holds and not at the plan's verdict: a value already on a **stale named origin**
keeps its names and is only told they are out of step. Sending it back to ports would take away
what the file actually says.

## Related

- `internal/rules/envorigins.go` — the whole origin surgery, pure and testable
- `internal/rules/envports.go` — the port substitution it sits beside
- [architecture.md](architecture.md) — which layer may call which
