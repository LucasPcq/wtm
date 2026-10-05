#!/usr/bin/env bash
# Prints the branch wtm records as the parent of <branch> (default: the current
# branch). Exit 3 when wtm knows the branch but records no parent (a root),
# exit 4 when wtm does not manage the branch at all.
set -euo pipefail

branch=${1:-$(git branch --show-current)}
tree=$(command wtm tree --output json)

if ! jq -e --arg b "$branch" '[.roots[] | recurse(.children[]?) | select(.branch == $b)] | length > 0' <<<"$tree" >/dev/null; then
  echo "wtm does not manage '$branch' (not in wtm tree)" >&2
  exit 4
fi

parent=$(jq -r --arg b "$branch" '[.roots[] | recurse(.children[]?) | select(any(.children[]?; .branch == $b)) | .branch] | first // empty' <<<"$tree")
if [[ -z "$parent" ]]; then
  echo "'$branch' is a root in wtm tree: no parent recorded" >&2
  exit 3
fi
echo "$parent"
