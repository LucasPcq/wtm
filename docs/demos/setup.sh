#!/usr/bin/env bash
# Builds the throwaway project every demo tape records against: a repository
# "acme" with a web app and an API, an isolated HOME, and wtm on PATH.
set -euo pipefail

root=${WTM_DEMO_ROOT:-/tmp/wtm-demo}
bin=${WTM_DEMO_BIN:?WTM_DEMO_BIN must point at the wtm binary to record}
web_port=${WTM_DEMO_WEB_PORT:-5173}
api_port=${WTM_DEMO_API_PORT:-8787}
proxy_port=${WTM_DEMO_PROXY_PORT:-11790}

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
mkdir -p scripts
cat > scripts/dev.py <<'PY'
#!/usr/bin/env python3
# A stand-in dev server: it listens on $PORT and talks like one.
import http.server, itertools, os, sys, threading, time

name, port = sys.argv[1], int(os.environ["PORT"])
lines = {
    "web": ["hmr update /src/App.tsx", "GET / 200 in 12ms", "hmr update /src/Login.tsx", "GET /assets/app.js 200 in 4ms"],
    "api": ["GET /health 200 2ms", "POST /session 201 18ms", "GET /users/me 200 6ms", "GET /health 200 1ms"],
}[name]

class Quiet(http.server.SimpleHTTPRequestHandler):
    def log_message(self, *args):
        pass

server = http.server.ThreadingHTTPServer(("", port), Quiet)
threading.Thread(target=server.serve_forever, daemon=True).start()
print(f"  {name} v1.4.2  ready in 312 ms", flush=True)
print(f"  ➜  Local:   http://localhost:{port}/", flush=True)
print("", flush=True)
for line in itertools.cycle(lines):
    time.sleep(1.3)
    print(time.strftime("%H:%M:%S ") + line, flush=True)
PY
cat > apps/web/package.json <<'JSON'
{ "name": "web", "scripts": { "dev": "python3 ../../scripts/dev.py web" } }
JSON
cat > apps/api/package.json <<'JSON'
{ "name": "api", "scripts": { "dev": "python3 ../../scripts/dev.py api" } }
JSON
printf 'PORT=%s\nAPI_URL=http://localhost:%s\n' "$web_port" "$api_port" > apps/web/.env.example
printf 'PORT=%s\n' "$api_port" > apps/api/.env.example
cp apps/web/.env.example apps/web/.env
cp apps/api/.env.example apps/api/.env
printf '.env\nnode_modules\n' > .gitignore
echo "# acme" > README.md
git add -A && git commit -qm "chore: bootstrap acme"

wtm init --yes --base-path ../acme.trees </dev/null >/dev/null 2>&1

global=$(dirname "$(find "$HOME" -name config.toml -path '*wtm*' | head -1)")/config.toml
printf '\n[proxy]\nport = %s\n' "$proxy_port" >> "$global"

cat > .git/wtm/run.toml <<TOML
[[job]]
  name = "web"
  kind = "service"
  cmd = "python3 ../../scripts/dev.py web"
  cwd = "apps/web"
  [job.ports]
    PORT = $web_port
  [job.url]
    port = "PORT"

[[job]]
  name = "api"
  kind = "service"
  cmd = "python3 ../../scripts/dev.py api"
  cwd = "apps/api"
  [job.ports]
    PORT = $api_port
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
