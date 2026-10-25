import { expect, test } from 'vitest'
import { sentence } from '@/features/worktrees/utils/sentence'

test('a message of the core reads as a sentence, a name it opens on kept as spelled', () => {
  expect(sentence('its pull request is merged')).toBe('Its pull request is merged')
  expect(sentence('feat/billing has 1 commit it lacks')).toBe('feat/billing has 1 commit it lacks')
  expect(sentence('.env is missing')).toBe('.env is missing')
  expect(sentence("couldn't fetch origin")).toBe("Couldn't fetch origin")
})
