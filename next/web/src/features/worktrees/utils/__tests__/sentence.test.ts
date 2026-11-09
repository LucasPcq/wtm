import { expect, test } from 'vitest'
import { sentence } from '@/features/worktrees/utils/sentence'

test('a message of the core reads as a sentence, a name it opens on kept as spelled', () => {
  expect(sentence('its pull request is merged')).toBe('Its pull request is merged')
  expect(sentence('feat/billing has 1 commit it lacks')).toBe('feat/billing has 1 commit it lacks')
  expect(sentence('.env is missing')).toBe('.env is missing')
  expect(sentence("couldn't fetch origin")).toBe("Couldn't fetch origin")
  expect(sentence("wtm's daemon is not running: which jobs are up is unknown")).toBe(
    "wtm's daemon is not running: which jobs are up is unknown",
  )
  expect(sentence('wtm-next daemon restart brings them back')).toBe('wtm-next daemon restart brings them back')
  expect(sentence('wtm has no config here')).toBe('wtm has no config here')
  expect(sentence('wtmx is not ours')).toBe('Wtmx is not ours')
})
