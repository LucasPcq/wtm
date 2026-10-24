import type { DomainAnswered, DomainField, DomainFieldError } from '@/api/types.gen'

// A form's values are keyed by their field's path, escaped: the form library reads remember[0] or a.b as nested.
// Each value is the answer as the catalogue takes it, an array of strings, but for a text list: its raw text, one
// entry a line, split only when it is sent, so that a line being typed keeps its newline.
export type Values = Record<string, string[]>

export const keyOf = (path: string) => encodeURIComponent(path).replaceAll('.', '%2E')

// valuesOf is what each field holds as the form stands, the core's defaults included.
export function valuesOf(fields: DomainField[]): Values {
  return Object.fromEntries(fields.map((field) => [keyOf(field.path), valueOf(field)]))
}

export function valueOf(field: DomainField): string[] {
  switch (field.kind) {
    case 'select':
    case 'text':
      return field.value ? [field.value] : []
    case 'multi_select':
      return field.picked
    case 'text_list':
      return [field.entries.join('\n')]
    case 'confirm':
      return [field.value ? field.yes.value : field.no.value]
  }
}

// answersOf is the request a form's values make: only the fields the person changed, so that what they left to
// its default (a remembered answer included) stays the core's to decide.
export function answersOf(fields: DomainField[], values: Values, changed: (key: string) => boolean): DomainAnswered[] {
  return fields.flatMap((field) => {
    const key = keyOf(field.path)
    if (!changed(key)) return []
    const value = values[key] ?? valueOf(field)
    return [{ path: field.path, values: field.kind === 'text_list' ? lines(value[0] ?? '') : value }]
  })
}

const lines = (text: string) =>
  text
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)

// problemsByKey files each problem of the core under the key of its field, one of a field's value (branches[1])
// under that field, and a problem no field holds under ''.
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

// problemsOf asks the core the questions again with the answers the person changed, and returns its problems
// filed for the form: under each field's key, and under form those of no field. The front checks nothing itself.
export async function problemsOf(params: {
  fields: DomainField[]
  values: Values
  changed: (key: string) => boolean
  ask: (answers: DomainAnswered[]) => Promise<{ problems: DomainFieldError[] }>
}): Promise<{ form: string | undefined; fields: Record<string, string> } | undefined> {
  const asked = await params.ask(answersOf(params.fields, params.values, params.changed))
  const { '': form, ...fields } = problemsByKey(params.fields, asked.problems)
  return form || Object.keys(fields).length ? { form, fields } : undefined
}
