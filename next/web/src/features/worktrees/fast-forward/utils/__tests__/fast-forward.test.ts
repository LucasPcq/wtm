import { expect, test } from 'vitest'
import { requestOf, unitsOf } from '@/features/worktrees/fast-forward/utils/plan'
import { recapOf, toldOf } from '@/features/worktrees/fast-forward/utils/recap'
import { optionWords } from '@/features/worktrees/fast-forward/utils/words'
import { advanceOf } from '@/test/fast-forward'
import { envelope } from '@/test/fixtures'

const plan = (forced: boolean, ...names: string[]) => ({
  worktrees: names.map((name) => advanceOf(name, forced)),
  force: forced,
})

test('the request the command picks itself names nothing', () => {
  expect(requestOf({ chosen: true, going: [advanceOf('main', false)], left: [] })).toEqual([])
})

test('a named request forces only when an unsafe worktree is checked, and takes the ones left with it', () => {
  const left = [advanceOf('feat/search', true)]
  expect(requestOf({ chosen: false, going: [advanceOf('main', true)], left })).toEqual([
    { path: 'worktree', values: ['main', 'feat/search'] },
  ])
  expect(requestOf({ chosen: false, going: [advanceOf('fix/login', true)], left: [] })).toEqual([
    { path: 'worktree', values: ['fix/login'] },
    { path: 'force', values: ['true'] },
  ])
})

test('a worktree behind origin left unchecked is never named, whatever the list it comes in', () => {
  const left = [advanceOf('fix/login', true), advanceOf('feat/billing-ui', true), advanceOf('feat/billing', true)]
  expect(requestOf({ chosen: false, going: [advanceOf('main', true)], left })).toEqual([
    { path: 'worktree', values: ['main', 'feat/billing'] },
  ])
})

test('only the worktrees that move are units of the run', () => {
  expect(unitsOf(plan(false, 'main', 'fix/login', 'feat/billing')).map((unit) => unit.subject)).toEqual(['main'])
})

test('the picker words where each option stands, a diverged one and a change in amber', () => {
  expect(optionWords({ value: 'a', origin: { ahead: 0, behind: 3, state: 'behind' }, changes: 2 })).toEqual({
    attention: ['2 uncommitted changes'],
    muted: ['3 behind origin'],
  })
  expect(optionWords({ value: 'b', origin: { ahead: 1, behind: 4, state: 'diverged' } })).toEqual({
    attention: ['diverged: 1 ahead, 4 behind'],
    muted: [],
  })
  expect(optionWords({ value: 'c' }).muted).toEqual(['not on origin'])
})

test('an outcome counts what moved: up to date is no move, and one not reached is cancelled first', () => {
  const ended = {
    ...envelope('fast-forward', []),
    plan: plan(false, 'main', 'feat/billing', 'feat/billing-ui'),
    items: [
      { subject: 'main', status: 'done' as const, detail: { path: '/acme', from: 'a', to: 'b' } },
      { subject: 'feat/billing', status: 'done' as const, reason: 'unchanged' as const, detail: undefined },
      { subject: 'feat/billing-ui', status: 'skipped' as const, reason: 'interrupted' as const, detail: undefined },
    ],
  }

  const { groups, moved } = recapOf({ units: [], envelope: ended })

  expect(moved).toBe(1)
  expect(groups.find((group) => group.kind === 'done')?.rows.map((row) => [row.title, row.message])).toEqual([
    ['main', '3 commits'],
  ])
  expect(groups.find((group) => group.kind === 'failed')?.rows.map((row) => [row.title, row.message])).toEqual([
    ['feat/billing-ui', 'not reached: cancelled first'],
  ])
})

test('a toast says what one worktree took, or why it did not move', () => {
  const done = {
    ...envelope('fast-forward', []),
    plan: plan(false, 'main'),
    items: [{ subject: 'main', status: 'done' as const, detail: { path: '/acme', from: 'a', to: 'b' } }],
  }
  expect(toldOf('main', done)).toEqual({
    tone: 'success',
    title: "main is on origin's tip",
    description: '3 commits taken from origin',
  })
  expect(toldOf('main', undefined)).toEqual({
    tone: 'error',
    title: "Couldn't fast-forward main",
    description: 'It stopped before it ran',
  })
})
