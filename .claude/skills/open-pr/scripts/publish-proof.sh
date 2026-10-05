#!/usr/bin/env bash
# Usage: publish-proof.sh <slug> <file>...
# Commits the files on the orphan branch `pr-assets/<slug>` without touching
# the working tree or the current branch, pushes it, and prints one Markdown
# image line per file, pinned to that commit so the PR body never breaks when
# the branch moves on. PROOF_REPO=owner/repo overrides the origin.
set -euo pipefail

slug=${1:?slug, e.g. the PR branch name with / replaced by -}
shift
(($# > 0)) || { echo "no files to publish" >&2; exit 1; }
branch=pr-assets/$slug

repo=${PROOF_REPO:-$(git remote get-url origin | sed -E 's#^(https://github\.com/|git@github\.com:)##; s#\.git$##')}
[[ "$repo" =~ ^[^/:]+/[^/:]+$ ]] || { echo "cannot read owner/repo from origin; set PROOF_REPO" >&2; exit 1; }

git fetch -q origin "+refs/heads/$branch:refs/remotes/origin/$branch" 2>/dev/null || true
parent=$(git rev-parse -q --verify "refs/remotes/origin/$branch^{commit}" || true)

GIT_INDEX_FILE=$(mktemp -u)
export GIT_INDEX_FILE
trap 'rm -f "$GIT_INDEX_FILE"' EXIT
if [[ -n "$parent" ]]; then git read-tree "$parent"; else git read-tree --empty; fi

for file in "$@"; do
  blob=$(git hash-object -w "$file")
  git update-index --add --cacheinfo "100644,$blob,$(basename "$file")"
done
commit=$(git commit-tree "$(git write-tree)" ${parent:+-p "$parent"} -m "proof: $slug")
git push -q origin "$commit:refs/heads/$branch"

for file in "$@"; do
  name=$(basename "$file")
  echo "![${name%.*}](https://raw.githubusercontent.com/$repo/$commit/$name)"
done
