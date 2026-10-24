import { expect, test } from 'vitest'
import type { DomainAnswered, DomainField } from '../api/types.gen'
import { answersOf, problemsByKey, problemsOf, valuesOf } from './answers'

const base = { flag: '', help: '', label: '' }
const fields: DomainField[] = [
  { ...base, kind: 'text', path: 'branch', value: '' },
  { ...base, kind: 'select', path: 'from', value: 'main', options: [], refreshable: false, remember: false, rememberable: true },
  { ...base, kind: 'text_list', path: 'remember[0]', entries: ['a'], options: [] },
  { ...base, kind: 'confirm', path: 'fetch', value: false, no_first: false, remember: false, rememberable: false, yes: { label: 'Yes', value: 'true' }, no: { label: 'No', value: 'false' } },
]
const problem = (path: string, message: string) => ({ path, message, code: 'c', flag: '' })

test('each field holds its default under the key of its index', () => {
  expect(valuesOf(fields)).toEqual({ f0: [], f1: ['main'], f2: ['a'], f3: ['false'] })
})

test('only the fields the person changed are answered, by their path', () => {
  const values = { ...valuesOf(fields), f0: ['feat/x'], f2: ['a', 'b'] }
  expect(answersOf(fields, values, (key) => key === 'f0' || key === 'f2')).toEqual([
    { path: 'branch', values: ['feat/x'] },
    { path: 'remember[0]', values: ['a', 'b'] },
  ])
})

test('a problem goes under its field, one of no field under the form', () => {
  expect(problemsByKey(fields, [problem('branch', 'empty'), problem('branch', 'taken'), problem('other', 'odd')])).toEqual({
    f0: 'empty\ntaken',
    '': 'odd',
  })
})

test('the validator asks the core with the changed answers and files what it says', async () => {
  const asked: DomainAnswered[][] = []
  const ask = async (answers: DomainAnswered[]) => {
    asked.push(answers)
    return { problems: answers.length ? [problem('branch', 'exists already')] : [] }
  }
  const values = { ...valuesOf(fields), f0: ['main'] }

  expect(await problemsOf({ fields, values, changed: (key) => key === 'f0', ask })).toEqual({ fields: { f0: 'exists already' } })
  expect(await problemsOf({ fields, values, changed: () => false, ask })).toBeUndefined()
  expect(asked).toEqual([[{ path: 'branch', values: ['main'] }], []])
})
