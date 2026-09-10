# Shared services — one instance for the repository, one namespace per worktree

The `run` module gives each worktree its own stack. That is the point, and it is also what makes a real monorepo expensive: two worktrees of a project with four postgres containers and a keycloak mean eight postgres and two JVMs. Some of those services have no reason to be duplicated, and `scope = "shared"` is how a project says so.

The measurement that motivated it is worth keeping: on the machine that prompted the work, CPU was 93% idle while memory sat at `587M unused` with `3230M compressor`. The constraint is memory, and the only variable wtm controls is the number of instances.

## The principle

wtm cannot know every provider. It provides **surfaces**: it runs a command at the right moment and injects the right environment, giving the command access to what only wtm knows — the worktree's ordinal, its ports, its URLs. What that command does to a postgres or a keycloak is the project's business.

Any question this document leaves open is settled with that sentence. If the answer requires wtm to learn what a realm is, it is the wrong answer.

## Where a shared job runs

`jobKey(name, workDir)` is untouched. The sharing is entirely in the choice of work dir.

The real service runs in the **main checkout** — the worktree `domain.GitWorktree.IsMain` designates — registered under `<main checkout>:<name>`: a real PTY, real ports, a real output hub. It is the only directory guaranteed to live as long as the repository, and being at ordinal 0 it takes **no port offset**, so a declared `5432` is the `5432` it binds. That stability is what lets a namespace's `env` write its URL literally.

Every other worktree posts an attachment under `<its worktree>:<name>`, with status `domain.JobStatusAttached`: no PID, no PTY, no hub, no log file. It is a pointer, and it is the reference count. **The job table is the count**, so there is no second registry to keep in step, and the statestore already persists it — a claim carries `Attached` in its record so it comes back from the index as what it is rather than as a foreground service the daemon had lost.

A claim owns no stream: attaching to one from any worktree reaches the one output there is.

A claim is dropped when its service is no longer up, and `Manager.Adopt` is where that is enforced: a daemon killed without running a handler may leave a shared foreground service to be reaped at the next start-up, and a claim outliving it would be a reference count on nothing — the next worktree would be told its service is already running. The pass runs on every adoption, not only after a reap, because the same hole opens whenever the real job's record is gone and the claims' are not (a deleted main checkout, for one). See LUC-227.

Releasing a claim stops the service only once no worktree holds it. Two worktrees racing to start the same service both succeed — `run up --all` fans out, and losing that race is not a failure. A shared job whose main checkout the client could not resolve is **refused**, never run once per worktree.

## The namespace

`[job.namespace]` is the worktree's slice of the shared service, and it is **singular**. It does not name an object of the provider: four keycloak realms are one namespace, whose internal shape belongs to the create script. That is the direct consequence of the principle above, and it is what keeps a list of realms or databases out of the TOML.

Two mechanisms, not two spellings of one:

- `{worktree}` and `{ordinal}` are **wtm's own substitution into data** — `namespace.name` and `namespace.env` are never executed, and wtm fills them in before anything runs;
- `$WTM_WORKTREE`, `$WTM_ORDINAL`, `$WTM_NAMESPACE` are **environment variables**, expanded by `/bin/sh` when `create` or `remove` runs.

`$WTM_WORKTREE` in a `name` would expand to nothing: no shell ever sees a name. Making wtm expand it there would be worse — it would look like shell syntax while only three variables worked, so `$HOME` would silently fail beside it. The step therefore offers each row only what that row takes.

`attach` and `detach` run with the **worktree's whole resolved environment** — ports and URLs included. That is what makes keycloak possible at all: a realm's `redirectUris` point at the fronts of the worktree asking for it, and the script needs those URLs. Without that access the design would handle postgres and leave keycloak stranded.

`create` runs on **every** start of the shared service, not once — wtm keeps no ledger of having run it, and a ledger would be wrong the moment the data went away behind wtm's back (`docker compose down -v`). So the command must be safe to run again: carve the slice out if it is absent, do nothing if it is there. That is the whole contract, and it is stated where the command is written — the schema, the `run init` step, and the failure message.

It is retried within `domain.NamespaceCreateTimeout`: the service it talks to was started moments ago, so a first refusal means "postgres is not accepting connections yet" far more often than it means the command is wrong. The budget is what stops a genuinely wrong command retrying for ever. It cannot tell a refusal that will pass from one that never will — which is exactly why the idempotence is the command's job and not wtm's guess.

The daemon is what runs it, and a daemon that considered itself idle while doing so used to exit under its own handler: a shared service launches detached, so nothing is left `Running` to keep it alive. The idle watcher counts connections in flight beside the running jobs.

An absent `[job.namespace]` is a valid answer: shared for good, one instance and one set of data.

### Reaching the app: the `[[env]]` link

Carving a slice out is half the work. The app has to be told which slice is its own, and that is not something a port can say.

`[[env_port]]` rewrites **the port inside** a value and leaves the rest alone, which is what lets a password live in a `.env` and never in `run.toml`. It can express "the shared keycloak answers here" and nothing else. A realm name is opaque — no number, no shape, nothing to anchor a substitution on.

So a second table, `[[env]]`, writes a key's **whole** value from a template:

```toml
[[env_port]]                          # the shared instance: one address for all
file = "apps/web/.env"
key  = "KEYCLOAK_URL"
job  = "keycloak"
port = "KEYCLOAK_PORT"

[[env]]                               # the slice: one per worktree
file  = "apps/web/.env"
key   = "KEYCLOAK_REALM"
job   = "keycloak"
value = "{namespace}"
```

That is the line between the two, and it is worth stating once: **`[[env_port]]` says where the service answers, `[[env]]` says which slice of it this worktree holds.** A shared service has one address for every worktree — its published host carries no worktree segment and its port takes no offset — so the first is not per-worktree at all.

The vocabulary is closed: `{namespace}`, `{port.NAME}`, `{origin}`, `{worktree}`, `{ordinal}`. Anything else is refused when `run.toml` is read, against a stand-in worktree, so a typo is caught for every worktree at once rather than the first time one is created. `{port.NAME}` goes through `rules.ResolvedPort`, the one place that answers what a declared port becomes in a worktree — deriving it a second time here is exactly how a report once said 5432 while the file was written 5452.

A resolved link becomes a `domain.EnvOwnedEntry`, which is the mechanism that already existed for `COMPOSE_PROJECT_NAME`: wtm owns the line, plans it, reports it when it changed, and takes it out of the reconciliation's verdict. A key wtm writes in full differs from its source by construction, so calling it a conflict would have every `wtm env` offer to undo the isolation it had just set up.

A key may not be written by both tables. They are not complementary — an `[[env]]` value writes its own port when it needs one (`postgresql://app:app@localhost:{port.POSTGRES_PORT}/{namespace}`), so a key both claim is a line to delete rather than a merge order to invent. It is refused at load, naming both.

Nothing here needs the daemon: `{namespace}` is `name` with `{worktree}` substituted, known without running anything. So the links settle at the same moments the port links do — when a worktree is created, and on `wtm env` or a `sync` reconciliation.

### `run init` and the keys it cannot detect

A port is detectable: the key is named `PORT` or `*_PORT`, the value is a number, and it matches a port a job declares. Three signs agreeing. `KEYCLOAK_REALM=myapp` has none of them — a realm name is an opaque word — so **the step asks**, and every managed key is a row. Filtering the list would hide the only key the reader wanted.

Two things narrow it without wtm pretending to know what a realm is:

- **A key whose value carries a port the service binds is its address, never its slice.** That is the `[[env_port]]` table's business, and the signal is structural rather than a guess about the key's name. It works on a first init, where no link exists yet. A key an `[[env_port]]` already writes is excluded for the same reason.
- **A key whose name starts with the job's own name is pre-checked** — `KEYCLOAK_*` beside a job called `keycloak`. That is a deduction from a name the user chose, not knowledge of the service.

On the pair that motivated the design, the two rules split it exactly: `KEYCLOAK_URL` holds `8080` and stays with the port table, `KEYCLOAK_REALM` is pre-checked and becomes an `[[env]]` link. Everything else is offered, unchecked, with the value it holds today beside it.

The one proposal wtm makes for a template is `{namespace}` — the same decision as `app_{worktree}` for the name, and as the two commands it proposes nothing for. Editing a template links its row: editing is asking for it to be written.

The step **migrates rather than stacks**. Marking a key that an `[[env_port]]` link already writes takes that link off, since the two are refused together at load — a wizard that wrote both would produce a config wtm then refuses to read, which is the worst outcome a wizard can have. The pruning happens once both tables are complete: the init pipeline settles the values and then appends more port links, so the last word is taken after that append.

Re-init is symmetric like every other step (`EnvValuesAsked`, the same `(value, asked)` pair): unchecking every row withdraws every link the step offered, a run that never asked leaves run.toml standing, and a link on a file the step never showed — one `.wtm.toml` no longer configures — survives untouched. A step may only remove what it proposed.

### Knowing a namespace exists

A claim goes with a `run stop`, so it cannot be what tells `clean` there is a database to drop. The worktree's own `meta.json` carries `namespaces`: the shared services it has actually carved a slice out of, recorded after a run from the jobs that came up. It lives there because the file is removed with the worktree it describes, and because both wrong answers are bad — giving back a namespace that was never created runs a `DROP DATABASE` on nothing, and missing one leaks a database on every iteration.

A worktree created and thrown away without ever starting the stack therefore owes nothing.

### Writing the two commands

`run init` asks. After the scope step, a step lists three rows per shared service — its name, its `create`, its `remove` — and only the name carries a proposal. wtm has nothing honest to say about the other two: a recipe for postgres would guess the port variable, the user, the host and whether `psql` is even on this machine, and a pre-filled command that is accepted and then fails inside the retry budget reads as a wtm bug rather than as a line to write. It is the same decision LUC-55 already recorded for the port flag of every framework.

What wtm *does* know it shows, while the field is open — grouped by where it comes from, since one run-on line stops being readable as soon as a job declares more than one port:

```
  available
    worktree  $WTM_NAMESPACE  $WTM_WORKTREE  $WTM_ORDINAL
    ports     $CRM_DB_PORT  $CRM_ADMIN_PORT
```

The first row is the same everywhere; the second is the ports **this job** declares, under the names it declares them by. A long group wraps under its own first variable rather than repeating its label. Both an inline command and the path to a script are accepted — both are a `/bin/sh` line run in the worktree.

An empty `create` is an answer, not an omission: the service is then shared outright, data included.

## Stopping is not destroying

`run stop` and `run down` never run `detach`. A `run down` that dropped a database would make the command unusable.

The detach belongs to `clean` and `prune`, and runs **before** the claims are released — releasing one may be the last, and a namespace cannot be given back to a service that is down. The default is to detach, since `clean` is the destructive command and removing a worktree without its data would leave an orphan behind on every iteration; `--keep-data` withholds it, under `--yes` as much as anywhere.

A service already down leaves a debt rather than being relit for a `DROP DATABASE`. The debt lives in `<git-common-dir>/wtm/pending-removals.toml`, beside the repository, because the worktree's own state directory is exactly what `clean` removes. It is a **queue and not a registry** — entries are only ever added by a failure and removed by a success — and `prune` settles it once the service is up.

## `run init` and the compose granularity

wtm generates **one job per compose file**, not per service. A scope had therefore nothing to sit on: a single `docker-compose.yml` with eight services was one job.

So the scanner reports what each file declares (`domain.ComposeScan.Services`, with `Image` and `HasBuild`), the step enumerates **services**, and marking one shared **lifts it into a job of its own** (`docker compose -f <file> up -d <service>`, stopped with `stop <service>` and never `down`, which would tear the whole file apart). The file's own job then names the services that stayed, since `docker compose up` would otherwise start the lifted one a second time. A file with nothing left keeps no job.

The step sits **before** the ports step: a shared job takes no offset, so which services are shared must be settled before their ports are.

A service with a `build:` is shown with its reason and no answer to give. That is structural, not a guess about the image's name — such a service compiles this worktree's source, so sharing it would serve one worktree's build to all of them.

Where `run.toml` has an opinion it outranks detection, and a run that never put the question leaves what it declares standing (`ScopesAsked`, the same `(value, asked)` pair as `URLsAsked` and the others).

## What a shared service changes on the surfaces

- Its published host carries **no worktree segment** (`db.projet.localhost`). One instance cannot answer under two names, and keeping the segment would have two worktrees' `.env` files disagree about where a single service answers.
- Its compose volume and network names need no special handling: they are already templated `${COMPOSE_PROJECT_NAME:-default}`, and a service running in the main checkout inherits that checkout's project name — so one stable name, automatically.
- A claim reports `pid: 0` and its own mark. Printing a PID beside three worktrees would read as three processes.
- A claim is **attachable**: it owns no stream, and the daemon resolves it to the one there is — so `run logs` works from any worktree. Its persisted tail is read from the main checkout's log directory, not from its own.
- Both the real job and every claim carry the main checkout they belong to. The daemon is machine-wide, so matching a claim to its service by name alone let two repositories that both declare `db` release each other's.

## What it costs when nothing is shared

Nothing. `sharedContext` is resolved once per seam and only when `run.toml` declares a shared job — otherwise every `run` command, `run ps` included, would pay a `git worktree list` plus a full environment resolution for the main checkout.
