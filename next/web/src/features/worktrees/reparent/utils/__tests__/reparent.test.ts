import { expect, test } from 'vitest'
import type { Applied } from '@/api/queries/commands'
import type { DomainFollowUp } from '@/api/types.gen'
import { byParent, syncOf } from '@/features/worktrees/reparent/utils/plan'
import { toldOf } from '@/features/worktrees/reparent/utils/words'

const move = (worktree: string, to: string): DomainFollowUp => ({
  command: 'reparent',
  answers: [
    { path: 'worktree', values: [worktree] },
    { path: 'to', values: [to] },
  ],
  code: 'clean.reparent',
  message: `move it onto ${to}`,
})

const ran = (moves: [string, string, string][]): Applied<'reparent'> => ({
  v: 2,
  ok: true,
  command: 'reparent',
  plan: undefined,
  warnings: [],
  items: moves.map(([subject, from, to]) => ({ subject, status: 'done', detail: { path: `/${subject}`, from, to } })),
  follow_ups: [
    {
      command: 'sync',
      answers: [{ path: 'worktree', values: moves.map(([subject]) => subject) }],
      code: 'reparent.sync',
      message: 'rebase onto the new parent',
    },
  ],
})

test('moves onto several parents are one run per parent, in the order they came', () => {
  expect(byParent([move('a', 'main'), move('b', 'x'), move('c', 'main')])).toEqual([
    { to: 'main', worktrees: ['a', 'c'] },
    { to: 'x', worktrees: ['b'] },
  ])
})

test('the runs one after the other are synced with one command naming them all', () => {
  const sync = syncOf([ran([['a', 'f', 'main']]), ran([['b', 'g', 'x']])])
  expect(sync?.answers).toEqual([{ path: 'worktree', values: ['a', 'b'] }])
})

test('one toast says what the runs did: one worktree, several onto one parent, onto several', () => {
  expect(toldOf([ran([['a', 'f', 'main']])]).title).toBe('a is on main')
  expect(toldOf([ran([['a', 'f', 'main']]), ran([['b', 'g', 'main']])]).title).toBe('2 worktrees moved onto main')
  expect(toldOf([ran([['a', 'f', 'main']]), ran([['b', 'g', 'x']])]).title).toBe('2 worktrees moved')
})

test('a run that failed says which worktree it could not move and why', () => {
  const failed: Applied<'reparent'> = {
    ...ran([]),
    ok: false,
    items: [
      {
        subject: 'b',
        status: 'failed',
        detail: undefined,
        error: { kind: 'internal', code: 'reparent.write_failed', message: 'x', params: { cause: 'denied' } },
      },
    ],
  }
  expect(toldOf([ran([['a', 'f', 'main']]), failed])).toEqual({
    tone: 'error',
    title: "Couldn't reparent b",
    description: "couldn't write its record",
  })
})

test('a run refused before it started says why, as the server said it', () => {
  expect(toldOf([], 'feat/billing is locked by clean, started from the terminal').description).toBe(
    'feat/billing is locked by clean, started from the terminal',
  )
})
