import { expect, test } from 'vitest'
import { dropOn, targetOf } from '@/features/worktrees/tree/utils/targets'
import { reparentQuestionsOf } from '@/test/reparent'

// feat/billing dragged: feat/billing-ui sits under it, main is its parent
const form = reparentQuestionsOf([{ path: 'worktree', values: ['feat/billing'] }])
const target = (branch: string, condition: 'clean' | 'deleted' = 'clean') =>
  targetOf({ branch, form, parent: 'main', condition })

test('a branch reparent offers takes the drop; its current parent neither takes nor refuses it', () => {
  expect(target('fix/login')).toEqual({ kind: 'valid' })
  expect(target('main')).toEqual({ kind: 'parent' })
})

test('a branch reparent refuses says why, worded from its code', () => {
  expect(target('feat/billing-ui')).toEqual({ kind: 'refused', why: 'feat/billing-ui sits under feat/billing' })
})

test('a branch reparent does not know is no branch any more: a deleted root says so', () => {
  expect(target('feat/gone', 'deleted')).toEqual({ kind: 'refused', why: 'feat/gone is deleted' })
})

test('until the questions land, every target waits but its current parent, known already', () => {
  expect(targetOf({ branch: 'fix/login', form: undefined, parent: 'main', condition: 'clean' })).toEqual({
    kind: 'pending',
  })
  expect(targetOf({ branch: 'main', form: undefined, parent: 'main', condition: 'clean' })).toEqual({
    kind: 'parent',
  })
})

test('a drop runs on a target that takes it or one not yet known, does nothing on its parent, says why when refused', () => {
  expect(dropOn({ kind: 'valid' })).toBe('run')
  expect(dropOn({ kind: 'pending' })).toBe('run')
  expect(dropOn({ kind: 'parent' })).toBe('none')
  expect(dropOn({ kind: 'refused', why: 'feat/billing-ui sits under feat/billing' })).toEqual({
    refused: 'feat/billing-ui sits under feat/billing',
  })
})
