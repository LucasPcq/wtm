import { expect, test } from 'vitest'
import {
  clock,
  latestError,
  lineAt,
  matchingLines,
  onlyMatching,
  patternOf,
  rangesIn,
} from '@/features/worktrees/running/utils/log-search'

const lines = (...texts: string[]) => texts.map((text) => ({ text }))

test('a search is text unless asked a regular expression, its case ignored unless kept', () => {
  const text = patternOf({ text: 'a.b', matchCase: false, regex: false })
  const regex = patternOf({ text: 'a.b', matchCase: false, regex: true })
  const cased = patternOf({ text: 'Error', matchCase: true, regex: false })

  expect(text instanceof RegExp && matchingLines(lines('a.b', 'axb'), text)).toEqual([0])
  expect(regex instanceof RegExp && matchingLines(lines('a.b', 'axb'), regex)).toEqual([0, 1])
  expect(cased instanceof RegExp && matchingLines(lines('Error', 'error'), cased)).toEqual([0])
  expect(patternOf({ text: '(', matchCase: false, regex: true })).toBe('invalid')
  expect(patternOf({ text: '', matchCase: false, regex: true })).toBeUndefined()
})

test('each stretch a search matches is marked, an empty match none', () => {
  expect(rangesIn('GET /invoices invoice', /invoice/gi)).toEqual([
    [5, 12],
    [14, 21],
  ])
  expect(rangesIn('abc', /x*/g)).toEqual([])
})

test('the latest error is the last line that says error, fail, panic or exception', () => {
  expect(latestError(lines('Error: no module', 'ready', 'stripe webhook failed', 'GET / 200'))).toBe(2)
  expect(latestError(lines('ready'))).toBeUndefined()
})

test('go to time finds the first line at or after it, says older before the first, none after the last', () => {
  const at = (hours: number, minutes: number) => new Date(2026, 10, 3, hours, minutes).toISOString()
  const held = [{ at: at(14, 0) }, { at: at(14, 5) }, {}, { at: at(14, 10) }]

  expect(lineAt(held, '14:03')).toBe(1)
  expect(lineAt(held, '14:10:00')).toBe(3)
  expect(lineAt(held, '13:00')).toBe('older')
  expect(lineAt(held, '15:00')).toBeUndefined()
  expect(clock(at(9, 4))).toBe('09:04:00')
})

test('only the lines matching say how many are hidden between them and printed since the last', () => {
  expect(onlyMatching([212, 309, 468], 500)).toEqual([
    { hidden: 212 },
    { line: 0 },
    { hidden: 96 },
    { line: 1 },
    { hidden: 158 },
    { line: 2 },
    { since: 31 },
  ])
  expect(onlyMatching([0, 1], 2)).toEqual([{ line: 0 }, { line: 1 }])
})

// The review of #270: a lookahead compiled here but reached the server, whose RE2 reads none, as a failure.
test('a regular expression the server could not read is no regex here either', () => {
  expect(patternOf({ text: 'GET (?=/health)', matchCase: false, regex: true })).toBe('invalid')
  expect(patternOf({ text: '(a)\\1', matchCase: false, regex: true })).toBe('invalid')
  expect(patternOf({ text: '(?=x)', matchCase: false, regex: false })).toBeInstanceOf(RegExp)
  expect(
    patternOf({ text: 'C:\\\\1', matchCase: false, regex: true }),
    'an escaped backslash, then a digit',
  ).toBeInstanceOf(RegExp)
  expect(patternOf({ text: 'a\\(?=b', matchCase: false, regex: true }), 'an escaped parenthesis').toBeInstanceOf(RegExp)
})

// The review of #270: a log printing past midnight took every time on its first line's day.
test('a time past midnight is found on the day of the last line', () => {
  const held = [
    { at: new Date(2026, 10, 3, 23, 59, 58).toISOString(), text: 'late' },
    { at: new Date(2026, 10, 4, 0, 0, 2).toISOString(), text: 'early' },
  ]

  expect(lineAt(held, '00:00:01')).toBe(1)
  expect(lineAt(held, '23:59:59')).toBe(1)
  expect(lineAt(held, '23:59:58')).toBe(0)
  expect(lineAt(held, '23:59:00'), 'before the first line, on its day').toBe('older')
  expect(lineAt(held, '00:00:05'), 'after the last line, on its day').toBeUndefined()
})
