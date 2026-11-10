import type { ProjectFoundHook, ProjectHook } from '@/api/types.gen'
import { spellHook } from '@/components/command-form/answers'

// a hook's answer as the form holds it, read back: JSON for one that runs in a folder or goes on, else the command
export function hookOf(value: string): ProjectHook {
  if (!value.trimStart().startsWith('{')) return { cmd: value }
  try {
    const read: unknown = JSON.parse(value)
    if (read && typeof read === 'object' && 'cmd' in read && typeof read.cmd === 'string') return read as ProjectHook
  } catch {}
  return { cmd: value }
}

export const changed = (values: string[], at: number, change: Partial<ProjectHook>) =>
  values.map((value, i) => {
    if (i !== at) return value
    const { cwd, continue_on_error: goOn, ...hook } = { ...hookOf(value), ...change }
    return spellHook({ ...hook, ...(cwd && { cwd }), ...(goOn && { continue_on_error: true }) })
  })

export const moved = <T>(values: T[], from: number, to: number) => {
  const next = [...values]
  const [value] = next.splice(from, 1)
  if (value !== undefined) next.splice(to, 0, value)
  return next
}

// the files the repository suggests its hooks from, each once, in the order found
export const foundIn = (found: ProjectFoundHook[]) => [...new Set(found.map((hook) => hook.file))]
