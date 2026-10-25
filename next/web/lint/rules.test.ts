import { spawnSync } from 'node:child_process'
import { copyFileSync, cpSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { expect, onTestFinished, test } from 'vitest'

// A GritQL plugin that breaks at runtime lets everything through without failing: only this test notices. Each case
// is a file of src/ (a path outside it starts with ../) and the plugin that must refuse it, none for a clean one.
// Biome's built-in rules are its own to test.
type Rule = 'layers' | 'unknown-folder' | 'props' | 'test-doubles' | 'spy-module' | 'none'

const cases: { path: string; source: string; rule: Rule }[] = [
  { path: 'features/a/to-b.ts', source: "export { y } from '@/features/b/y'", rule: 'layers' },
  { path: 'features/a/own.ts', source: "export { y } from '@/features/a/utils/y'", rule: 'none' },
  { path: 'features/a/dotted.ts', source: "export { y } from '@/features/a/../b/y'", rule: 'layers' },
  { path: 'api/up.ts', source: "export { cn } from '@/lib/utils'", rule: 'layers' },
  { path: 'lib/react.ts', source: "export { useState } from 'react'", rule: 'layers' },
  { path: 'routes/x.tsx', source: "export { Centered } from '@/components/centered'", rule: 'layers' },
  {
    path: 'routes/y.tsx',
    source: "import type { QueryClient } from '@tanstack/react-query'\nexport type Q = QueryClient",
    rule: 'none',
  },
  { path: 'components/into.ts', source: "export { x } from '@/features/a/x'", rule: 'layers' },
  { path: 'features/a/kit.ts', source: "export { server } from '@/test/server'", rule: 'layers' },
  { path: 'features/a/msw.ts', source: "export { http } from 'msw'", rule: 'layers' },
  {
    path: 'features/a/stream.ts',
    source: "export { EventSourceParserStream } from 'eventsource-parser/stream'",
    rule: 'layers',
  },
  { path: 'stuff/x.ts', source: 'export const x = 1', rule: 'unknown-folder' },
  { path: 'loose.ts', source: 'export const x = 1', rule: 'unknown-folder' },
  { path: 'features/a/components/not-tsx.ts', source: 'export const x = 1', rule: 'unknown-folder' },
  { path: 'features/a/hooks/__tests__/use-x.test.ts', source: 'export const x = 1', rule: 'unknown-folder' },
  { path: 'features/a/b/odd/x.ts', source: 'export const x = 1', rule: 'unknown-folder' },
  { path: 'features/a/b/hooks/use-y.ts', source: 'export const y = 1', rule: 'none' },
  { path: '../vite.config.ts', source: 'export default {}', rule: 'none' },
  {
    path: 'features/a/components/one.tsx',
    source: 'export function One(props: { t: string }) {\n  return <b>{props.t}</b>\n}',
    rule: 'props',
  },
  {
    path: 'features/a/components/two.tsx',
    source: 'export default function Two(props: { t: string }) {\n  return <b>{props.t}</b>\n}',
    rule: 'props',
  },
  {
    path: 'features/a/components/three.tsx',
    source: 'export const Three = function (props: { t: string }) {\n  return <b>{props.t}</b>\n}',
    rule: 'props',
  },
  {
    path: 'features/a/components/four.tsx',
    source: "import { memo } from 'react'\nexport const Four = memo((props: { t: string }) => <b>{props.t}</b>)",
    rule: 'props',
  },
  {
    path: 'features/a/components/five.tsx',
    source: 'export function Five({ t }: { t: string }) {\n  return <b>{t}</b>\n}',
    rule: 'none',
  },
  {
    path: 'features/a/components/__tests__/alias.test.tsx',
    source: "import { vi as v } from 'vitest'\nexport const x = v",
    rule: 'test-doubles',
  },
  {
    path: 'features/a/components/__tests__/vitest.test.tsx',
    source: "import { vitest } from 'vitest'\nvitest.mock('@/api/fetch')",
    rule: 'test-doubles',
  },
  {
    path: 'features/a/components/__tests__/hook.test.tsx',
    source: "import * as rtl from '@testing-library/react'\nexport const x = rtl.renderHook",
    rule: 'test-doubles',
  },
  {
    path: 'features/a/components/__tests__/fetch.test.tsx',
    source: "import { vi } from 'vitest'\nvi.spyOn(globalThis, 'fetch')",
    rule: 'test-doubles',
  },
  {
    path: 'features/a/components/__tests__/module.test.tsx',
    source: "import { vi } from 'vitest'\nimport * as api from '@/api/fetch'\nvi.spyOn(api, 'json')",
    rule: 'spy-module',
  },
  {
    path: 'features/a/components/__tests__/storage.test.tsx',
    source: "import { vi } from 'vitest'\nvi.spyOn(localStorage, 'setItem')",
    rule: 'none',
  },
]

const web = path.join(import.meta.dirname, '..')

type Diagnostic = {
  category: string
  message?: unknown
  description?: unknown
  location: { path?: string | { file: string } }
}

// the build's own biome.json, plugins and ignore file, unchanged, at the root of the cases; the root sits in a folder
// named src, as a checkout can
function project() {
  const top = mkdtempSync(path.join(tmpdir(), 'wtm-lint-'))
  onTestFinished(() => rmSync(top, { recursive: true, force: true }))
  const root = path.join(top, 'src', 'web')

  mkdirSync(root, { recursive: true })
  for (const file of ['biome.json', '.gitignore']) copyFileSync(path.join(web, file), path.join(root, file))
  cpSync(path.join(web, 'lint'), path.join(root, 'lint'), { recursive: true, filter: (file) => !file.endsWith('.ts') })
  for (const { path: at, source } of cases) {
    const file = path.join(root, 'src', at)
    mkdirSync(path.dirname(file), { recursive: true })
    writeFileSync(file, `${source}\n`)
  }
  return root
}

// it runs Biome over the cases: a cold start on a busy runner takes seconds
test('each case is refused by exactly its plugin, and a clean one by none', { timeout: 30_000 }, () => {
  const root = project()

  const run = spawnSync(path.join(web, 'node_modules/.bin/biome'), ['lint', '--reporter=json', '.'], {
    cwd: root,
    encoding: 'utf8',
  })
  const found = new Map<string, Set<string>>()
  for (const { category, message, description, location } of JSON.parse(run.stdout).diagnostics as Diagnostic[]) {
    if (category !== 'plugin') continue
    const at = path.normalize(typeof location.path === 'string' ? location.path : (location.path?.file ?? ''))
    const rule = /web-front skill, ([a-z-]+)/.exec(String(message ?? description))?.[1] ?? '?'
    found.set(at, (found.get(at) ?? new Set()).add(rule))
  }

  for (const { path: at, rule } of cases) {
    const refused = [...(found.get(path.normalize(path.join('src', at))) ?? [])]
    expect(refused, at).toEqual(rule === 'none' ? [] : [rule])
  }
})
