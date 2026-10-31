#!/usr/bin/env bash
# Usage: checkout.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same checkout scenarios in two identical sandboxes (the docs/demos "acme"
# project with a bare origin and a second clone that pushes to it), one per binary:
# legacy `wtm checkout` against `wtm-next checkout`. A fake gh, signed in unless the
# sandbox holds gh.signed-out, answers both from the sandbox's prs.json (gh pr list,
# gh pr view). After each scenario it records the exit code, the output and the state
# (branches with their upstream, worktrees, every meta.json), then diffs the states.
# Then, in a fresh sandbox per side, it drives the wizard in tmux and captures its
# screens under <out dir>/tty/<side>. Outputs differ by design (next's v2 contract);
# the states may differ only where next/docs/proof/README.md says why.
set -euo pipefail

# absolute <binary>: the binary's absolute path, since each scenario runs from inside its sandbox.
absolute() {
  [[ -f $1 && -x $1 ]] || { echo "$0: $1 is not an executable" >&2; return 1; }
  echo "$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
}
legacy=$(absolute "${1:?wtm binary}")
next=$(absolute "${2:?wtm-next binary}")
out=${3:?out dir}
mkdir -p "$out" && out=$(cd "$out" && pwd)
repo=$(git rev-parse --show-toplevel)

# fake_gh <dir>: writes in dir a gh that answers gh pr list and gh pr view from <dir>/../prs.json, signed out while <dir>/../gh.signed-out exists.
fake_gh() {
  cat >"$1/gh" <<'SH'
#!/usr/bin/env bash
root=$(cd "$(dirname "$0")/.." && pwd)
if [[ $1 == auth ]]; then [[ -f $root/gh.signed-out ]] && { echo "You are not logged into any GitHub hosts." >&2; exit 1; }; exit 0; fi
exec python3 - "$root/prs.json" "$@" <<'PY'
import json, sys
prs, args = json.load(open(sys.argv[1])), sys.argv[2:]
if args[:2] == ["pr", "list"]:
    prs = [p for p in prs if p["state"] == "OPEN"]
    if "--author" in args: prs = [p for p in prs if p["author"]["login"] == "me"]
    if "--search" in args: prs = [p for p in prs if p.get("review")]
    print(json.dumps(prs))
    sys.exit()
if args[:2] == ["pr", "view"]:
    for p in prs:
        if p["number"] == int(args[2]):
            print(json.dumps(p))
            sys.exit()
    print("GraphQL: Could not resolve to a PullRequest with the number of %s. (repository.pullRequest)" % args[2], file=sys.stderr)
    sys.exit(1)
print("gh: unexpected call", args, file=sys.stderr)
sys.exit(1)
PY
SH
  chmod +x "$1/gh"
}

# commit <dir> <name>: one commit of a file of that name in the checkout at dir.
commit() {
  echo "$2" >"$1/$2.txt"
  git -C "$1" add "$2.txt"
  git -C "$1" -c user.email=dev@acme.dev -c user.name=Dev commit -qm "$2"
}

# pr <number> <title> <author> <head> <base> <fork> <draft> [state]: a pull request as gh prints it.
pr() {
  printf '{"number":%s,"title":"%s","author":{"login":"%s"},"headRefName":"%s","headRefOid":"","baseRefName":"%s","url":"https://github.com/acme/acme/pull/%s","isCrossRepository":%s,"isDraft":%s,"state":"%s"}' \
    "$1" "$2" "$3" "$4" "$5" "$1" "$6" "$7" "${8:-OPEN}"
}

# prepare <root>: the acme project with origin, the branches of the pull requests and prs.json, as the legacy wtm leaves it.
prepare() {
  local root=$1
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=24400 WTM_DEMO_API_PORT=24401 WTM_DEMO_PROXY_PORT=24402 \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home PATH=$root/bin:$PATH
  fake_gh "$root/bin"
  cd "$root/acme"
  git init -q --bare "$root/origin.git"
  git remote add origin "$root/origin.git"
  git push -q -u origin main
  git push -q origin main:release/1.0
  git clone -q "$root/origin.git" "$root/other"
  local branch
  for branch in remote local behind diverged held gone draft on-release merged own auto-deleted; do
    git -C "$root/other" checkout -q -b "feat/$branch" main
    commit "$root/other" "$branch"
    git -C "$root/other" push -q origin "feat/$branch"
  done
  git fetch -q origin
  for branch in local behind diverged held own; do
    git branch -q --track "feat/$branch" "origin/feat/$branch"
  done
  git -C "$root/other" checkout -q feat/behind && commit "$root/other" behind-2 && git -C "$root/other" push -q origin feat/behind
  git -C "$root/other" checkout -q feat/diverged && commit "$root/other" origin-only && git -C "$root/other" push -q origin feat/diverged
  git fetch -q origin
  git checkout -q feat/diverged && commit . local-only && git checkout -q main
  git worktree add -q "$root/held" feat/held
  git push -q origin --delete feat/gone
  git fetch -q --prune origin
  # merged and deleted on origin since the last fetch, which kept its remote-tracking branch: no prune since
  git -C "$root/other" push -q origin --delete feat/auto-deleted
  {
    echo "["
    pr 1 "Add the remote feature" alice feat/remote main false false && echo ","
    pr 2 "Local branch in sync" me feat/local main false false && echo ","
    pr 3 "Local branch behind origin" bob feat/behind main false false && echo ","
    pr 4 "Local branch diverged from origin" bob feat/diverged main false false && echo ","
    pr 5 "From a fork, on main" stranger main main true false && echo ","
    pr 6 "Branch held by a worktree" alice feat/held main false false && echo ","
    pr 7 "Branch deleted on origin" alice feat/gone main false false && echo ","
    pr 8 "Stacked draft on feat/local" me feat/draft feat/local false true && echo ","
    pr 9 "Merging into a branch only origin has" alice feat/on-release release/1.0 false false && echo ","
    pr 10 "Merged already" alice feat/merged main false false MERGED && echo ","
    pr 11 "Its own parent, asked" me feat/own main false false && echo ","
    pr 12 "Merged, its branch deleted since the last fetch" alice feat/auto-deleted main false false MERGED
    echo "]"
  } >"$root/prs.json"
}

scenario() {
  local name=$1
  shift
  local code=0
  "$bin" "$@" </dev/null >"$dir/$name.stdout" 2>"$dir/$name.stderr" || code=$?
  echo "$code" >"$dir/$name.exit"
  state >"$dir/$name.state"
}

state() {
  echo "## branches"
  git branch --format='%(refname:short) %(upstream:short)'
  echo "## worktrees"
  git worktree list --porcelain | awk '/^worktree /{print $2} /^branch /{print "  " $2}'
  for meta in $(find .git/wtm/worktrees -name meta.json 2>/dev/null | sort); do
    echo "## $meta"
    grep -v created_at "$meta"
  done
}

# normalise <files…>: the sandbox, times, operation ids, commits, fingerprints and durations as placeholders, and next's name as legacy's.
normalise() {
  local root=$1
  shift
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e 's#"created_at": "[^"]*"#"created_at": "<T>"#g' \
    -e 's#"op": "[0-9a-f]{16}"#"op": "<OP>"#g' \
    -e 's#[0-9a-f]{40}#<SHA>#g' \
    -e 's#[0-9a-f]{24}#<FP>#g' \
    -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' \
    -e 's#wtm-next checkout#wtm checkout#g' \
    -e 's#wtmx#wtm#g' \
    "$@"
}

run_side() {
  local side=$1
  bin=$2
  local root
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  dir=$out/$side
  rm -rf "$dir" && mkdir -p "$dir"
  (
    prepare "$root"
    state >"$dir/00-before.state"
    scenario 01-origin-only checkout 1 --yes
    scenario 02-local-json checkout 2 --yes --output json
    scenario 03-behind-kept checkout 3 --yes
    scenario 04-diverged-ff checkout 4 --yes --ff
    scenario 05-fork checkout 5 --yes
    scenario 06-held checkout 6 --yes
    scenario 07-deleted-on-origin checkout 7 --yes
    scenario 08-not-found checkout 99 --yes
    scenario 09-no-number checkout --yes
    scenario 10-not-a-number checkout abc
    scenario 11-own-parent checkout 11 --yes --from feat/own
    scenario 12-base-only-on-origin checkout 9 --yes
    scenario 13-merged checkout 10 --yes
    scenario 14-fork-json checkout 5 --yes --output json
    scenario 15-mine-and-review checkout --mine --review --yes
    touch "$root/gh.signed-out"
    scenario 16-signed-out checkout 8 --yes
    rm "$root/gh.signed-out"
    # next only: legacy has no dry run
    scenario 17-dry-run checkout 8 --dry-run
    scenario 18-stacked-draft checkout 8 --yes
    scenario 19-deleted-since-fetched checkout 12 --yes
  )
  normalise "$root" "$dir"/*
}

# tty_side <side> <binary>: the wizard of checkout --from main in tmux, in a sandbox of its own, origin's feat/local
# moved on since the last fetch: the picker, the env strategy, then next's fast-forward asked on where origin stands now,
# kept, where legacy, reading origin as last fetched since no branch list refreshed it, asks none (B3); the recap, the
# outcome.
tty_side() {
  local side=$1 bin=$2 root
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  local tty=$out/tty/$side session=wtmcheckout-$side-$(basename "$root")
  rm -rf "$tty" && mkdir -p "$tty"
  (
    prepare "$root"
    git -C "$root/other" checkout -q feat/local && commit "$root/other" local-2 && git -C "$root/other" push -q origin feat/local
    cp "$bin" "$root/bin/wtmx"
    trap 'tmux kill-session -t "$session" 2>/dev/null' EXIT
    screen() { tmux capture-pane -p -t "$session" | cat -s; }
    settle() {
      local now last="" tries=0
      while ((tries < 100)); do
        now=$(screen)
        [[ $now == *"$1"* && $now == "$last" ]] && return
        last=$now tries=$((tries + 1))
        sleep 0.3
      done
      echo "checkout.sh: the $side screen never settled on \"$1\"" >&2
      screen >&2
      return 1
    }
    capture() {
      settle "$2"
      screen >"$tty/$1.txt"
    }
    # highlight <text>: moves down the list until the row under the cursor (▌) holds text.
    highlight() {
      local tries=0
      until screen | grep '▌' | grep -q -- "$1"; do
        ((tries++ < 20)) || { echo "checkout.sh: no row of $1 on the $side screen" >&2; return 1; }
        tmux send-keys -t "$session" Down
        sleep 0.3
      done
    }
    tmux new-session -d -s "$session" -x 100 -y 40 -c "$root/acme" "zsh -f"
    tmux send-keys -t "$session" "export HOME=$root/home PATH=$root/bin:\$PATH PS1='$ ' && clear" Enter
    settle '$'
    tmux send-keys -t "$session" "wtmx checkout --from main; echo EXIT=\$?" Enter
    capture 1-pull-requests '#9'
    highlight 'Local branch in sync'
    tmux send-keys -t "$session" Enter
    capture 2-env 'Env strategy'
    tmux send-keys -t "$session" Enter
    settle 'Isolation'
    tmux send-keys -t "$session" Enter
    if [[ $side == next ]]; then
      capture 3-branch-update 'Keep it as-is'
      highlight 'Keep it as-is'
      tmux send-keys -t "$session" Enter
    fi
    capture 4-recap 'No, cancel'
    tmux send-keys -t "$session" Enter
    capture 5-outcome 'EXIT=0'
  )
  normalise "$root" "$tty"/*
}

mkdir -p "$out"
roots=()
trap 'rm -rf ${roots[@]+"${roots[@]}"}' EXIT
run_side legacy "$legacy"
run_side next "$next"
tty_side legacy "$legacy"
tty_side next "$next"
{
  for state in "$out"/legacy/*.state; do
    name=$(basename "$state" .state)
    echo "=== $name: exit legacy $(cat "$out/legacy/$name.exit" 2>/dev/null), next $(cat "$out/next/$name.exit" 2>/dev/null)"
    diff -u --label "legacy/$name" --label "next/$name" "$state" "$out/next/$name.state" || true
  done
} >"$out/states.diff"
echo "$out/states.diff"
