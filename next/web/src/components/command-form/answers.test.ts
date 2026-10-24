import { expect, test } from 'vitest'
import type { DomainAnswered, DomainField } from '@/api/types.gen'
import { answersOf, keyOf, problemsByKey, problemsOf, valuesOf } from './answers'

const base = { flag: '', help: '', label: '' }
const branch: DomainField = { ...base, kind: 'text', path: 'branch', value: '' }
const from: DomainField = {
  ...base,
  kind: 'select',
  path: 'from',
  value: 'main',
  options: [],
  refreshable: false,
  remember: false,
  rememberable: true,
}
const remember: DomainField = { ...base, kind: 'text_list', path: 'remember[0]', entries: ['a'], options: [] }
const fetch: DomainField = {
  ...base,
  kind: 'confirm',
  path: 'env.fetch',
  value: false,
  no_first: false,
  remember: false,
  rememberable: false,
  yes: { label: 'Yes', value: 'true' },
  no: { label: 'No', value: 'false' },
}
const fields = [branch, from, remember, fetch]
const problem = (path: string, message: string) => ({ path, message, code: 'c', flag: '' })

test('a key never reads as nested to the form library', () => {
  expect(keyOf('remember[0]')).toBe('remember%5B0%5D')
  expect(keyOf('env.fetch')).toBe('env%2Efetch')
})

test('each field holds its default under the key of its path', () => {
  expect(valuesOf(fields)).toEqual({ branch: [], from: ['main'], 'remember%5B0%5D': ['a'], 'env%2Efetch': ['false'] })
})

test('only the fields the person changed are answered, by their path', () => {
  const values = { ...valuesOf(fields), branch: ['feat/x'] }
  expect(answersOf(fields, values, (key) => key === 'branch')).toEqual([{ path: 'branch', values: ['feat/x'] }])
})

// The textarea joined and split its entries on every keystroke: the newline of a second entry vanished as typed.
test('a text list keeps the line being typed and sends its entries', () => {
  const typing = { [keyOf('remember[0]')]: ['a\n'] }
  expect(answersOf([remember], typing, () => true)).toEqual([{ path: 'remember[0]', values: ['a'] }])
  expect(typing[keyOf('remember[0]')]).toEqual(['a\n'])

  const typed = { [keyOf('remember[0]')]: ['a\n b \n\nc'] }
  expect(answersOf([remember], typed, () => true)).toEqual([{ path: 'remember[0]', values: ['a', 'b', 'c'] }])
})

// Fields were keyed by their index in the first form: a field the core added before another took its value.
test('a value stays with its field when the questions change around it', () => {
  const values = { ...valuesOf([branch, from]), from: ['dev'] }
  const asked = [branch, remember, from]
  expect(answersOf(asked, values, (key) => key === 'from')).toEqual([{ path: 'from', values: ['dev'] }])
  expect(answersOf(asked, values, (key) => key === keyOf('remember[0]'))).toEqual([
    { path: 'remember[0]', values: ['a'] },
  ])
})

test('a problem goes under its field, one of no field under the form', () => {
  expect(
    problemsByKey(fields, [problem('branch', 'empty'), problem('branch', 'taken'), problem('other', 'odd')]),
  ).toEqual({
    branch: 'empty\ntaken',
    '': 'odd',
  })
})

test('the validator asks the core with the changed answers and files what it says', async () => {
  const asked: DomainAnswered[][] = []
  const ask = async (answers: DomainAnswered[]) => {
    asked.push(answers)
    return { problems: answers.length ? [problem('branch', 'exists already')] : [] }
  }
  const values = { ...valuesOf(fields), branch: ['main'] }

  expect(await problemsOf({ fields, values, changed: (key) => key === 'branch', ask })).toEqual({
    fields: { branch: 'exists already' },
  })
  expect(await problemsOf({ fields, values, changed: () => false, ask })).toBeUndefined()
  expect(asked).toEqual([[{ path: 'branch', values: ['main'] }], []])
})
