#!/usr/bin/env bash
# Builds the throwaway project every demo tape records against: a repository
# "acme" with a web app and an API, an isolated HOME, and wtm on PATH.
set -euo pipefail

root=${WTM_DEMO_ROOT:-/tmp/wtm-demo}
bin=${WTM_DEMO_BIN:?WTM_DEMO_BIN must point at the wtm binary to record}

# A previous recording may have left its jobs up: stop them before the files go.
if [[ -x "$root/bin/wtm" && -d "$root/acme" ]]; then
  (cd "$root/acme" && HOME="$root/home" "$root/bin/wtm" run down --all --yes >/dev/null 2>&1 || true)
  HOME="$root/home" "$root/bin/wtm" run daemon stop --yes >/dev/null 2>&1 || true
fi
rm -rf "$root"
mkdir -p "$root/home" "$root/bin" "$root/acme"
cp "$bin" "$root/bin/wtm"
export HOME="$root/home" PATH="$root/bin:$PATH"

cd "$root/acme"
git init -q -b main
git config user.email demo@acme.dev
git config user.name "Acme Dev"

mkdir -p apps/web apps/api
cat > apps/web/package.json <<'JSON'
{ "name": "web", "scripts": { "dev": "python3 -m http.server $PORT" } }
JSON
cat > apps/api/package.json <<'JSON'
{ "name": "api", "scripts": { "dev": "python3 -m http.server $PORT" } }
JSON
printf 'PORT=5173\nAPI_URL=http://localhost:8787\n' > apps/web/.env.example
printf 'PORT=8787\n' > apps/api/.env.example
cp apps/web/.env.example apps/web/.env
cp apps/api/.env.example apps/api/.env
printf '.env\nnode_modules\n' > .gitignore
echo "# acme" > README.md
git add -A && git commit -qm "chore: bootstrap acme"

wtm init --yes --base-path ../acme.trees </dev/null >/dev/null 2>&1

global=$(dirname "$(find "$HOME" -name config.toml -path '*wtm*' | head -1)")/config.toml
printf '\n[proxy]\nport = 11790\n' >> "$global"

cat > .git/wtm/run.toml <<'TOML'
addressing = "ports"

[[job]]
  name = "web"
  kind = "service"
  cmd = "python3 -m http.server $PORT"
  cwd = "apps/web"
  [job.ports]
    PORT = 5173
  [job.url]
    port = "PORT"

[[job]]
  name = "api"
  kind = "service"
  cmd = "python3 -m http.server $PORT"
  cwd = "apps/api"
  [job.ports]
    PORT = 8787
  [job.url]
    port = "PORT"

[[profile]]
  name = "dev"
  jobs = ["api", "web"]
  default = true

[[env_port]]
  file = "apps/web/.env"
  key = "PORT"
  job = "web"
  port = "PORT"

[[env_port]]
  file = "apps/web/.env"
  key = "API_URL"
  job = "api"
  port = "PORT"

[[env_port]]
  file = "apps/api/.env"
  key = "PORT"
  job = "api"
  port = "PORT"
TOML
