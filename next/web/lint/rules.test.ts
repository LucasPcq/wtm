import { spawnSync } from 'node:child_process'
import { readdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { expect, onTestFinished, test } from 'vitest'

// Each fixture plants a violation and names on its first line the rules that must refuse it, `none` for a clean one.
// A GritQL plugin that breaks at runtime lets everything through without failing: only this test notices.
const web = path.join(import.meta.dirname, '..')
const fixtures = path.join(import.meta.dirname, 'fixtures')

type Diagnostic = {
  category: string
  message?: unknown
  description?: unknown
  location: { path?: string | { file: string } }
}

const filesOf = (dir: string): string[] =>
  readdirSync(dir).flatMap((name) => {
    const at = path.join(dir, name)
    return statSync(at).isDirectory() ? filesOf(at) : [at]
  })

// the build's config, as the root of the fixtures: the same rules and overrides, only its paths moved
function configure() {
  const config = JSON.parse(readFileSync(path.join(web, 'biome.json'), 'utf8'))
  const moved = (plugin: string) => path.relative(fixtures, path.join(web, plugin))
  const file = path.join(fixtures, 'biome.json')

  writeFileSync(
    file,
    JSON.stringify({
      ...config,
      $schema: undefined,
      vcs: undefined,
      root: true,
      files: { includes: ['**', '!biome.json', '!src/components/ui', '!**/*.gen.ts'] },
      plugins: config.plugins.map((plugin: string | { path: string }) =>
        typeof plugin === 'string' ? moved(plugin) : { ...plugin, path: moved(plugin.path) },
      ),
    }),
  )
  onTestFinished(() => rmSync(file))
}

function ruleOf({ category, message, description }: Diagnostic): string {
  if (category !== 'plugin') return category.replace(/^lint\//, '')
  return `plugin/${/web-front skill, ([a-z-]+)/.exec(String(message ?? description))?.[1] ?? '?'}`
}

// it runs Biome over the whole tree: a cold start on a busy runner takes seconds
test('each planted violation is refused by its own rules, and a clean file by none', { timeout: 30_000 }, () => {
  configure()

  const run = spawnSync(path.join(web, 'node_modules/.bin/biome'), ['lint', '--reporter=json', '.'], {
    cwd: fixtures,
    encoding: 'utf8',
  })
  const found = new Map<string, Set<string>>()
  for (const diagnostic of JSON.parse(run.stdout).diagnostics as Diagnostic[]) {
    const at = diagnostic.location.path
    const file = path.join(fixtures, typeof at === 'string' ? at : (at?.file ?? ''))
    found.set(file, (found.get(file) ?? new Set()).add(ruleOf(diagnostic)))
  }

  for (const fixture of filesOf(path.join(fixtures, 'src'))) {
    const expected = /^\/\/ expect: (.+)/.exec(readFileSync(fixture, 'utf8'))?.[1] ?? '?'
    const rules = expected === 'none' ? [] : expected.split(', ')
    expect([...(found.get(fixture) ?? [])].sort(), path.relative(fixtures, fixture)).toEqual(rules.sort())
  }
})
