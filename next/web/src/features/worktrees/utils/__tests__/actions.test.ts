import { expect, test } from 'vitest'
import { withPullRequest } from '@/features/worktrees/utils/actions'
import { offers } from '@/test/fixtures'

test("the live read's actions stay, but the pull request's comes from GitHub's read", () => {
  const live = offers('feat/a', { go: 'missing', open_pr: 'missing', sync: 'missing' })
  const asked = offers('feat/a')

  const shown = withPullRequest(live, asked)

  expect(shown.find((offer) => offer.action === 'go')?.refused).toBe('missing')
  expect(shown.find((offer) => offer.action === 'open_pr')?.follow_up?.url).toBe('https://github.com/acme/acme/pull/1')
  expect(withPullRequest(live, undefined)).toEqual(live)
})
