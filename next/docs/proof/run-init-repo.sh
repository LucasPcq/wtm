#!/usr/bin/env bash
# Usage: run-init-repo.sh <root> <wtm binary>
# Builds the monorepo run init is proven on, under <root>: a repository "acme"
# (pnpm workspaces, a turbo root dev script, apps api, web, admin and worker,
# packages/db with migrate and seed, a docker-compose.yml with postgres, redis and
# mailpit, five .env files, apps/web's a .env.local with no template), its own
# HOME, and `wtm init --yes` done with the binary given, which config.toml copies
# the committed templates' .env files from. No run.toml: run init writes it.
set -euo pipefail

root=${1:?root}
bin=${2:?wtm binary}
mkdir -p "$root/home" "$root/bin" "$root/acme"
cp "$bin" "$root/bin/wtm"
export HOME=$root/home PATH=$root/bin:$PATH
cd "$root/acme"
git init -q -b main
git config user.email dev@acme.dev
git config user.name "Acme Dev"

mkdir -p apps/api apps/web apps/admin apps/worker packages/db
cat >package.json <<'JSON'
{
  "name": "acme",
  "private": true,
  "packageManager": "pnpm@9.12.0",
  "scripts": { "dev": "turbo run dev", "build": "turbo run build" },
  "devDependencies": { "turbo": "^2.1.0" }
}
JSON
printf 'packages:\n  - "apps/*"\n  - "packages/*"\n' >pnpm-workspace.yaml
printf '{ "tasks": { "dev": { "cache": false, "persistent": true } } }\n' >turbo.json

cat >apps/api/package.json <<'JSON'
{ "name": "@acme/api", "scripts": { "dev": "tsx watch src/server.ts", "build": "tsc" } }
JSON
cat >apps/web/package.json <<'JSON'
{ "name": "@acme/web", "scripts": { "dev": "next dev", "build": "next build" } }
JSON
cat >apps/admin/package.json <<'JSON'
{ "name": "@acme/admin", "scripts": { "dev": "next dev -p 3001", "build": "next build" } }
JSON
cat >apps/worker/package.json <<'JSON'
{ "name": "@acme/worker", "scripts": { "dev": "tsx watch src/worker.ts" } }
JSON
cat >packages/db/package.json <<'JSON'
{ "name": "@acme/db", "scripts": { "migrate": "prisma migrate deploy", "seed": "tsx seed.ts" } }
JSON

cat >docker-compose.yml <<'YAML'
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: postgres
    ports:
      - "5432:5432"
  redis:
    image: redis:7
    ports:
      - "6379:6379"
  mailpit:
    image: axllent/mailpit
    ports:
      - "8025:8025"
      - "1025:1025"
YAML

printf 'PORT=4000\nDATABASE_URL=postgres://postgres:postgres@localhost:5432/acme\nREDIS_URL=redis://localhost:6379\n' >apps/api/.env.example
printf 'PORT=3000\nNEXT_PUBLIC_API_URL=http://localhost:4000\n' >apps/web/.env.local
printf 'NEXT_PUBLIC_API_URL=http://localhost:4000\n' >apps/admin/.env.example
printf 'REDIS_URL=redis://localhost:6379\nSMTP_PORT=1025\n' >apps/worker/.env.example
printf 'DATABASE_URL=postgres://postgres:postgres@localhost:5432/acme\n' >packages/db/.env.example
for template in apps/api/.env.example apps/admin/.env.example apps/worker/.env.example packages/db/.env.example; do
  cp "$template" "${template%.example}"
done
printf '.env\n.env.local\nnode_modules\n' >.gitignore
echo "# acme" >README.md
git add -A
git -c core.hooksPath=/dev/null commit -qm "chore: bootstrap acme"

wtm init --yes --base-path ../acme.trees </dev/null >/dev/null 2>&1
