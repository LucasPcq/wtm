# Platform support

wtm is built for macOS and Linux, on amd64 and arm64. Windows runs it through WSL2, as a Linux.

| | macOS | Linux | WSL2 | Windows (native) |
| --- | --- | --- | --- | --- |
| Release binaries | ✓ | ✓ | ✓ (the Linux one) | ✗ |
| Homebrew | ✓ | ✓ | ✓ | ✗ |
| Worktrees: `create`, `clean`, `sync`, `prune`, `exec`, `ui`… | ✓ | ✓ | ✓ | ✗ |
| Shell integration (`wtm go`): zsh, bash, fish | ✓ | ✓ | ✓ | ✗ |
| `wtm run`: jobs, ports, compose isolation, shared services | ✓ | ✓ | ✓ | ✗ |
| Named URLs on the proxy port (`:11080`) | ✓ | ✓ | ✓ | ✗ |
| Named URLs on port 80 (`wtm run proxy install`) | ✓ | ✗ | ✗ | ✗ |
| `wtm upgrade` | ✓ | ✓ | ✓ | ✗ |
| Tested | daily use | CI (`make test` on every pull request) | not yet | ✗ |

## Linux

Everything but one feature: `wtm run proxy install`, which serves named URLs on port 80 so they drop their `:11080`, relies on a launchd agent and exists on macOS only. On Linux the named URLs keep their port, and `wtm run url --raw` prints the plain `http://localhost:<port>` address. See [Named URLs](addressing.md).

The global config and the run daemon's files live under `~/.config/wtm/` (see [State](state.md)).

## WSL2

WSL2 runs the Linux binary, with the Linux row above. Keep the repository on the Linux filesystem (`~/…`, not `/mnt/c/…`): git is much slower across the boundary, and every worktree would pay it. wtm never talks to Docker itself: your jobs call the `docker` CLI, so whichever engine that CLI reaches inside the distribution is the one `wtm run` uses.

WSL2 is not tested on every release yet: a report of what works or breaks there is welcome in an [issue](https://github.com/LucasPcq/wtm/issues/new/choose).

## Windows

Not supported natively. `wtm run` jobs, hooks and `wtm exec` commands are `/bin/sh` lines, and the run daemon talks over a Unix socket and manages process groups: none of that has a direct Windows equivalent. Use WSL2.
