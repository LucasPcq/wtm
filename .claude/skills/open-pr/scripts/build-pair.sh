#!/usr/bin/env bash
# Usage: build-pair.sh <before-ref> <out-dir>
# Builds <out-dir>/before/wtm from <before-ref> (git archive, no worktree
# registered) and <out-dir>/after/wtm from the current working tree.
set -euo pipefail

ref=${1:?before ref (e.g. the merge-base with the PR base)}
out=${2:?output directory}
root=$(git rev-parse --show-toplevel)
src=$(mktemp -d)
trap 'rm -rf "$src"' EXIT

mkdir -p "$out/before" "$out/after"
git -C "$root" archive "$ref" | tar -x -C "$src"
(cd "$src" && go build -o "$out/before/wtm" .)
(cd "$root" && go build -o "$out/after/wtm" .)
echo "$out/before/wtm"
echo "$out/after/wtm"
