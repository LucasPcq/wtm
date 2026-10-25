import { execFileSync } from 'node:child_process'
import { mkdtempSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { expect, test } from 'vitest'

// Each fixture plants one violation of the rules of .oxlintrc.json and says on its first line which rule must refuse
// it (`// expect: wtm(layers)`, `none`): a rule that stops catching what it promises fails here.
const fixtures = path.join(import.meta.dirname, 'fixtures')

function filesOf(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const at = path.join(dir, name)
    return statSync(at).isDirectory() ? filesOf(at) : [at]
  })
}

test('each planted violation is refused by its rule, and the clean file by none', () => {
  // the config the build lints with, its plugin found from anywhere, the fixtures no longer ignored
  const config = JSON.parse(readFileSync(path.join(import.meta.dirname, '..', '.oxlintrc.json'), 'utf8'))
  config.jsPlugins = [path.join(import.meta.dirname, 'wtm.js')]
  config.ignorePatterns = config.ignorePatterns.filter((pattern: string) => !pattern.startsWith('lint/'))
  const file = path.join(mkdtempSync(path.join(tmpdir(), 'oxlint-')), 'oxlintrc.json')
  writeFileSync(file, JSON.stringify(config))

  let out: string
  try {
    out = execFileSync('npx', ['oxlint', '-c', file, '-f', 'json', fixtures], { encoding: 'utf8' })
  } catch (failed) {
    out = (failed as { stdout: string }).stdout
  }
  const found = new Map<string, Set<string>>()
  for (const diagnostic of JSON.parse(out).diagnostics as { filename: string; code: string }[]) {
    const at = path.resolve(diagnostic.filename)
    found.set(at, (found.get(at) ?? new Set()).add(diagnostic.code))
  }

  for (const fixture of filesOf(fixtures)) {
    const expected = /\/\/ expect: (.+)/.exec(readFileSync(fixture, 'utf8'))?.[1]
    const got = [...(found.get(fixture) ?? [])]
    expect(got, path.relative(fixtures, fixture)).toEqual(expected === 'none' ? [] : [expected])
  }
})
