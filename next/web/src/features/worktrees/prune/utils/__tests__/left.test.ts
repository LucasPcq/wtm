import { expect, test } from 'vitest'
import { leftOut } from '@/features/worktrees/prune/utils/left'

const why = "No longer finished: it has commits past its pull request's head"

test('a worktree a plan refused is said with its why while it stays unchecked', () => {
  const dropped = [{ worktree: 'feat/a', why }]

  expect(leftOut({ dropped, asked: [], candidates: ['feat/a'], going: [] })).toEqual(dropped)
  expect(leftOut({ dropped, asked: [], candidates: ['feat/a'], going: ['feat/a'] }), 'checked again').toEqual([])
})

test('a worktree asked for that left the candidates is said once, with the why a plan gave when it gave one', () => {
  expect(leftOut({ dropped: [], asked: ['feat/a', 'feat/b'], candidates: ['feat/b'], going: ['feat/b'] })).toEqual([
    { worktree: 'feat/a', why: 'No longer finished' },
  ])
  expect(leftOut({ dropped: [{ worktree: 'feat/a', why }], asked: ['feat/a'], candidates: [], going: [] })).toEqual([
    { worktree: 'feat/a', why },
  ])
})
