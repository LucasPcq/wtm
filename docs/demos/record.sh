#!/usr/bin/env bash
# Records every tape against a fresh demo project: `make demos`.
set -euo pipefail
cd "$(dirname "$0")/../.."
bin=$(mktemp -d)/wtm
version=$(sed -n 's/^## v\([^ ]*\).*/\1/p' CHANGELOG.md | head -1)
go build -ldflags "-X github.com/LucasPcq/wtm/internal/domain.Version=${version:-dev}" -o "$bin" .
for tape in docs/demos/${1:-*}.tape; do
  [[ $(basename "$tape") == _* ]] && continue
  WTM_DEMO_BIN=$bin docs/demos/setup.sh
  vhs "$tape"
done
WTM_DEMO_BIN=$bin docs/demos/setup.sh
