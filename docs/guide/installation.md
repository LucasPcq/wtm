# Installation

wtm is one static binary. It needs `git`; [`gh`](https://cli.github.com) is optional and unlocks the GitHub features (`checkout` a PR, PR status, `prune` of merged branches). Which systems it runs on, and what each one supports, is in [Platform support](platform-support.md).

## The latest release

| Method | Command | Platforms |
| --- | --- | --- |
| Homebrew | `brew install LucasPcq/tap/wtm` | macOS, Linux |
| Go | `go install github.com/LucasPcq/wtm@latest` | any Go target wtm builds on |
| Release binary | download from the [releases](https://github.com/LucasPcq/wtm/releases) | macOS, Linux (amd64, arm64) |

A release binary goes onto your `PATH` by hand:

```bash
tar -xzf wtm_*_linux_amd64.tar.gz     # or _linux_arm64, _darwin_amd64, _darwin_arm64
sudo mv wtm /usr/local/bin/
```

Each release publishes a `checksums.txt` beside its archives: `sha256sum --ignore-missing -c checksums.txt` (`shasum -a 256` on macOS) checks a download against it.

Then add the shell integration, which is what lets `wtm go` change your directory:

```bash
echo 'eval "$(wtm shell-init)"' >> ~/.zshrc    # ~/.bashrc for bash
```

For fish, add `wtm shell-init | source` to `config.fish`.

## A specific version

To stay on a version your team has validated, or to go back to one after a regression. Versions are written with or without the `v` (`0.28.0` or `v0.28.0`); the list is on the [releases](https://github.com/LucasPcq/wtm/releases) page.

| Method | How |
| --- | --- |
| Go | `go install github.com/LucasPcq/wtm@v0.28.0` |
| Release binary | download that release's archive: `https://github.com/LucasPcq/wtm/releases/download/v0.28.0/wtm_0.28.0_<os>_<arch>.tar.gz` |
| Already installed as a release binary | `wtm upgrade --version 0.28.0`, which also moves **down** to an older release |
| Homebrew | the tap ships the latest release only. `brew pin wtm` keeps the version you have; for another one, uninstall it (`brew uninstall wtm`) and use a release binary |

For example, on Linux amd64:

```bash
V=0.28.0
curl -fsSLO "https://github.com/LucasPcq/wtm/releases/download/v$V/wtm_${V}_linux_amd64.tar.gz"
tar -xzf "wtm_${V}_linux_amd64.tar.gz" && sudo mv wtm /usr/local/bin/
wtm version
```

Going back across a minor version, read that release's **Breaking** section in the [changelog](../../CHANGELOG.md) and its migration notes ([0.29](migrating-to-0.29.md), [0.28](migrating-to-0.28.md)) first: a command or a `--output json` shape your scripts rely on may differ.

## Updating

`wtm upgrade` updates wtm the way it was installed: a release binary is replaced in place once its SHA256 matches the release's checksums, a Homebrew or `go install` binary is handed to that tool. `wtm upgrade --check` only reports whether a newer release exists. See [`wtm upgrade`](../wtm_upgrade.md).

wtm also checks for a new release at most once a day and tells you when one is out. `WTM_NO_UPDATE_CHECK=1` turns that off, which is what you want on a pinned version.
