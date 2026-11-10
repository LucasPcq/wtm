import { expect, test } from 'vitest'
import { settled } from '@/features/run-setup/utils/setup'
import { summaryOf } from '@/features/run-setup/utils/summary'
import { parts } from '@/features/run-setup/utils/words'
import { runInitForm, runInitPlan } from '@/test/run-init'

test('each part folds into one line read from the form and the plan', () => {
  const setup = { form: runInitForm, plan: settled(runInitPlan) }

  expect(parts.map((part) => summaryOf(setup, part))).toEqual([
    '9 jobs: 3 from compose, 6 from scripts',
    'postgres and mailpit shared; redis per worktree',
    '4 jobs get a port per worktree',
    'admin, api and web by name',
    '8 keys in 5 files follow the worktree',
    'all (default), admin, api, web, worker',
  ])
})
