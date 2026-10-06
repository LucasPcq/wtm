# Named URLs and addressing

```console
$ wtm run url feat/x --job shop-api
http://api.feat-x.myrepo.localhost:11080
$ wtm run url feat/x --job shop-api --raw
http://localhost:4011
$ curl "$(wtm run url feat/x --job shop-api)/health"
```

## Two ways to reach a job

- A **port URL** is the job's own port: `http://localhost:4012`. Every worktree binds its own port, so two worktrees never collide, but they share `localhost`, and with it the browser's cookie jar and every CORS origin.
- A **named URL** is served by the **run proxy**, which lives in the run daemon: `http://api.feat-x.myrepo.localhost:11080`. Each worktree gets its own hostname, so two of them stop sharing a cookie or an origin. `*.localhost` names the loopback in browsers and most HTTP clients, so nothing is added to `/etc/hosts`.

A job opts into a name with a `url` table naming the declared port that speaks HTTP:

```toml
[[job]]
name = "shop-api"
kind = "service"
cmd  = "pnpm dev"
  [job.ports]
  PORT = 4001
  [job.url]
  port = "PORT"        # the declared port the proxy forwards to
  host = "api"         # optional: the first label, the job's name when absent
```

The host is `<host>.<worktree>.<repo>.localhost`: `<worktree>` is the branch as a DNS-safe slug, `<repo>` the repository's directory name. A [shared service](shared-services.md) has one address for the whole repository, so its host carries no worktree segment. `wtm run init` offers a name to every service that declares the port it listens on; `wtm run job add|edit --url-port PORT --url-host api` set it by hand.

`wtm run url [worktree] --job <name>` prints a job's named URL (`--raw` the port URL) and nothing else, for `$(…)`; `wtm run open` hands it to the browser. `run up`, `run ps` and the run view show the same addresses.

## The proxy's port

The proxy listens on the loopback only, on port `11080` by default, set in the [global config](configuration.md#global-config):

```toml
[proxy]
port    = 11080     # 0 or absent means this default
enabled = true      # false: every surface hands out port URLs instead
```

A port it cannot bind costs the names, never the jobs. `wtm run proxy status` reports the configured and real ports and whether port 80 is redirected.

On **macOS**, `wtm run proxy install` removes the port from every named URL: it installs a per-user LaunchAgent: launchd binds port 80 on the loopback and hands the socket to wtm, which relays it to the proxy. No sudo, no system file; `wtm run proxy uninstall` removes it. On other systems the named URLs keep their `:11080`.

## Addressing: what a `.env` value holds

When a `.env` value points at another job (`VITE_API_URL`, `CORS_ORIGIN`), wtm rewrites it for each worktree through an [`[[env_port]]` link](run-toml.md#env_port). `addressing` in `run.toml` decides what that rewrite writes:

- `"names"` (the **default** when the key is absent) writes the job's whole named origin, `http://api.feat-x.myrepo.localhost`, whenever the linked job publishes a URL for that port **and** the value has the shape of a URL. A bare `PORT=4011` stays a number, and a `DATABASE_URL` stays a port: the proxy only speaks HTTP.
- `"ports"` writes port numbers everywhere.

The choice has a consequence outside wtm: named URLs answer while `wtm run` runs the job, and not when you start the app yourself. A project whose author launches dev servers by hand wants `"ports"`. On a machine where the proxy is off, ports are written whatever the mode says, and a notice says so. Under `"ports"` the run surfaces also hand out port URLs and register no name.

```bash
wtm run addressing ports --yes             # port numbers, every worktree's .env settled
wtm run addressing names --yes             # back to named origins
wtm run addressing names --yes --keep-env  # switch run.toml alone
```

The switch writes `addressing` in `run.toml`, then settles the worktrees whose `.env` spells the other mode (an interactive run asks first; `wtm run addressing` alone asks for the mode). Under `"names"` the named URL is a worktree's only working entrance: opening its port directly sends an `Origin` the API no longer knows.

### The main checkout

The main checkout is the one that exists without wtm, the one a colleague clones and a `docker compose up` reads, so **nothing moves it onto names unless you ask**: a switch brings it back to ports but never onto names, and `wtm env main` keeps whatever its `.env` spells. You choose, when you want it:

```bash
wtm env main                          # the wizard asks: keep ports, or move onto names
wtm env main --addressing names --yes # move main onto the named URLs
wtm env main --addressing ports --yes # bring main back to ports, and only main
```

- Until then, under `"names"`, main's working entrance is its **port** URL. wtm still hands out its name everywhere (`run up`, `run url`, `run open`, the run view, `wtm ui`) and adds a line saying the `.env` still spells ports; `--raw` gives the port URL. The route is registered either way, so nothing has to restart.
- Afterwards, whoever reads main's `.env` or starts its stack depends on the proxy being up. It is worth it right after switching a project to `"names"`, and after `wtm run proxy install`, which drops the `:11080` from the origins already written.
- Reconciling main's keys (`wtm env main`, `--yes`, `--check`) never changes its addressing: it is read against the mode its `.env` already spells. It writes no `[[env]]` namespace into main either, so `--addressing ports` really does give back a checkout without wtm; see [Shared services](shared-services.md#telling-the-app-env).
- `--addressing` is the main checkout's alone: a linked worktree follows `run.toml` (`wtm run addressing`), and `--addressing names` is refused while the project addresses by ports.
- `wtm create` from main is unaffected either way: a copied value carrying main's segment is rewound to the new worktree's.
