## wtm run init

Configure the run module (services & tasks) for this repo

### Synopsis

Set up run.toml by detecting docker-compose files and package.json scripts and turning
the selected ones into jobs.

In a TTY, opens a wizard to pick which ones to include; non-interactively (or piped),
auto-generates from detection. Re-running pre-fills every step from the existing
run.toml: what stays checked is kept, what you uncheck is removed along with the
profile entries and .env links naming it. Only jobs this wizard proposed are ever
removed — one added with `wtm run job add` is never listed, so never touched.
A non-interactive run asks nothing and removes nothing.

Ports declared in the selected compose files become per-worktree ports. A literal
host port ("5432:5432") binds the same port everywhere, so wtm offers to rewrite it
as "${DB_PORT:-5432}:5432" — the default keeps `docker compose up` working on its
own. Declining leaves the file untouched and declares no port for it.

The names those files pin absolutely get the same treatment. A container_name, or
a volume's or network's explicit name, is resolved by the Docker daemon rather than
by the compose project, so COMPOSE_PROJECT_NAME never reaches it and a second
worktree collides on it. wtm offers to front them with the project — a renamed
volume starts empty, its data staying under the name it used to carry.

In a monorepo, a root script that starts several apps at once is asked which
declared jobs it runs. wtm reads the directory a script sits in, never what its
command does: the relation is declared, and it is what keeps a runner from being
reported as a service that forgot its port — and from being started alongside one
of its own children.

Dev servers get theirs from the env files sitting next to their package.json —
a PORT (or *_PORT) entry in .env.local, .env, or a committed .env.example. A
service nothing was found for is offered anyway: declaring its port is what keeps
a second worktree from binding the same one.

wtm injects the variable, it never edits the command. When a command never
mentions the port it is given, the wizard offers it for editing on the spot
(`pnpm dev --port ${PORT}`) rather than reporting it once it is too late.

The mode those names are written in is asked too, because it is the one choice
with a consequence outside wtm: named urls are served by the run proxy, so they
answer while `wtm run` runs the job and not when you start it yourself. A project
whose author launches their own dev servers wants ports.

Every service that declares the port it listens on is then offered a name of its
own — <job>.<worktree>.<repo>.localhost, served by the proxy — so two worktrees
stop sharing a cookie jar. A port a job only dials (DB_PORT, REDIS_PORT) is never
offered: a name nothing answers under is worse than no name at all.

`wtm run` is experimental — the workflow is still stabilizing and commands may change.

```
wtm run init [flags]
```

### Options

```
  -h, --help              help for init
      --link-env          Link the .env keys holding a declared port, so each worktree gets its own
      --non-interactive   Auto-generate from detection; never prompt
      --patch-compose     Rewrite the selected compose files' literal host ports and absolute names to read a variable
      --write-port-keys   Write each declared port into the job's .env and its template, so an app launched by hand reads the worktree's port
  -y, --yes               Auto-generate from detection; never prompt
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

