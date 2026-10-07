# Security policy

## Supported versions

Security fixes land in the latest release only. `wtm upgrade` brings an install up to it.

## Reporting a vulnerability

Please do not open a public issue. Report it privately through [GitHub's private vulnerability reporting](https://github.com/LucasPcq/wtm/security/advisories/new), with the version (`wtm version --output json`), the platform and the steps to reproduce.

You get an answer within a week. Once a fix is released, the advisory is published with credit to the reporter, unless they prefer otherwise.

## What is in scope

wtm runs on a developer's machine, with that developer's rights. The parts worth a report are the ones that cross a boundary:

- **The run daemon** and its Unix socket: another local user reaching it, or a command it runs that it should not.
- **The run proxy**, which listens on the loopback, and the port 80 LaunchAgent that `wtm run proxy install` installs on macOS.
- **`wtm upgrade`**, which downloads a release and checks it against the release's `checksums.txt` before replacing the binary.
- **Secrets**: a `.env` value or a token leaking into output, logs, `--output json` or the event stream.

Out of scope: hooks (`on_create`, `on_clean`) and `run.toml` jobs run the commands their author wrote, as the user who runs wtm. A malicious repository can already run code through them, as it can through a `Makefile` or a `package.json` script.
