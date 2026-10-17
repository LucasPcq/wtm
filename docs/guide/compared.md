# Compared to other tools

What wtm does that the tools around it do not, and what they do that wtm does not. The facts below were checked against each project's README, documentation or landing page in October 2026; open an [issue](https://github.com/LucasPcq/wtm/issues/new/choose) when one goes stale.

## The short version

Most tools in this space cover one part of a branch's life. Worktree managers create and remove worktrees. Agents create one per session. A handful of small tools isolate one piece of the runtime: ports, a database name, Compose, or a URL. wtm takes a branch through the whole of it, from one identity per worktree: created ready to code, running its own stack at its own address, rebased with the branches it sits on, and removed once merged without losing work. It runs your scripts rather than knowing your stack, so it works with any data store, and one person's configuration is shared with the team through `wtm run export` / `import`.

## Tool by tool

| | What it does | Where wtm differs |
|---|---|---|
| `git worktree` | A directory per branch. | Everything after the checkout: provisioning, a running stack, stacks of branches, safe removal. |
| [Worktrunk](https://github.com/max-sixty/worktrunk) | A worktree lifecycle CLI built for parallel agents: switch, list, merge, remove, hooks, agent plugins. Per-worktree ports and databases are documented as recipes you write into your hooks. | The stack is built in: shifted ports, Compose project, named URL and your namespace scripts, from one identity. Stacked branches are rebased as a chain (`sync`, `reparent`). |
| [gtr](https://github.com/coderabbitai/git-worktree-runner) | A portable worktree CLI: new, remove, copy config, post-create hooks, launch an agent or editor. | gtr stops at setup. |
| [workmux](https://github.com/raine/workmux) | A worktree and a tmux window per task, agents in panes, sandbox backends. | No port, URL or Compose identity; workmux merges a branch into its parent but does not rebase a chain. wtm does not depend on tmux. |
| Claude Code `--worktree`, Codex, Cursor | A worktree per agent session, ignored files copied, a setup script. | None of them runs the worktree's stack. wtm is what the agent drives from there: `--yes`, `--output json`, the [skill](integrations.md). |
| [workz](https://github.com/rohansx/workz) | Dependency sync, then a port range, a Postgres database name in `DATABASE_URL` and a Compose project per worktree; an MCP server. | wtm runs your own scripts for any data store instead of handling one database, serves named URLs, and adds the worktree lifecycle and stacks. |
| [worktree-compose](https://github.com/mostafasudo/worktree-compose) | Docker Compose isolation per worktree: ports, containers, volumes. | Compose only, on `localhost` ports. wtm runs any job, not only containers, behind a name. |
| [portless](https://github.com/vercel-labs/portless) | Named, stable `.localhost` URLs instead of port numbers; in a linked worktree the branch becomes the subdomain. | portless is a URL proxy, not a worktree manager: no ports per worktree for other services, no Compose, database or lifecycle. |
| [Conductor](https://www.conductor.build/), [Superset](https://github.com/superset-sh/superset), [Paseo](https://github.com/getpaseo/paseo) | Desktop apps that host your agents, each in a worktree, with setup scripts; Paseo also runs per-worktree services behind `.localhost` URLs. | wtm is a CLI in your terminal, with whatever agent, editor or terminal you already use, and scriptable end to end. |

## When another tool fits better

- You only need a directory per branch and switch between them by hand: `git worktree` or a light manager such as gtr is enough.
- You want a desktop app that hosts and supervises your agents: Conductor, Superset or Paseo.
- Your only problem is remembering port numbers: portless does that alone.
