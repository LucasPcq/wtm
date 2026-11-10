import type { DomainAnswered, DomainField, DomainFieldError } from '@/api/types.gen'

// A form's values are keyed by their field's path, escaped: the form library reads remember[0] or a.b as nested.
// Each value is the answer as the catalogue takes it, an array of strings, but for a text list: its raw text, one
// entry a line, split only when it is sent, so that a line being typed keeps its newline.
type Values = Record<string, string[]>

export const keyOf = (path: string) => encodeURIComponent(path).replaceAll('.', '%2E')

export function valuesOf(fields: DomainField[]): Values {
  return Object.fromEntries(fields.map((field) => [keyOf(field.path), defaultOf(field)]))
}

export function defaultOf(field: DomainField): string[] {
  switch (field.kind) {
    case 'select':
    case 'text':
      return field.value ? [field.value] : []
    case 'multi_select':
      return field.picked
    case 'text_list':
      return [field.entries.join('\n')]
    case 'confirm':
      return [field.value ? 'true' : 'false']
    case 'hooks':
      return field.entries.map(spellHook)
  }
}

// A hook is answered as the catalogue reads it back: its command alone when it runs in the root and stops on a
// failure, else the hook as JSON.
export const spellHook = (hook: { cmd: string; cwd?: string; continue_on_error?: boolean }) =>
  hook.cwd || hook.continue_on_error ? JSON.stringify(hook) : hook.cmd

export function answersOf(fields: DomainField[], values: Values, changed: (key: string) => boolean): DomainAnswered[] {
  return fields.flatMap((field) => {
    const key = keyOf(field.path)
    if (!changed(key)) return []
    const value = values[key] ?? defaultOf(field)
    return [{ path: field.path, values: field.kind === 'text_list' ? lines(value[0] ?? '') : value }]
  })
}

const lines = (text: string) =>
  text
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)

export function problemsByKey(fields: DomainField[], problems: DomainFieldError[]): Record<string, string> {
  const paths = new Set(fields.map((field) => field.path))
  const fieldOf = (path: string) => [path, path.replace(/\[\d+\]$/, '')].find((candidate) => paths.has(candidate))
  const filed: Record<string, string> = {}
  for (const problem of problems) {
    const field = fieldOf(problem.path)
    const key = field === undefined ? '' : keyOf(field)
    filed[key] = filed[key] ? `${filed[key]}\n${problem.message}` : problem.message
  }
  return filed
}
