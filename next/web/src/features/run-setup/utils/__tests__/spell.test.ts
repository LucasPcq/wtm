import { expect, test } from 'vitest'
import { settled } from '@/features/run-setup/utils/setup'
import { spellCommand, spellPort, spellProfile, spellSpace } from '@/features/run-setup/utils/spell'
import { runInitPlan } from '@/test/run-init'

const { parts } = settled(runInitPlan)

test('each row of a real plan spells as the core spells it', () => {
  for (const row of parts.ports) expect(spellPort(row)).toBe(row.spelled)
  for (const row of parts.spaces) expect(spellSpace(row)).toEqual(row.spelled)
  for (const row of parts.profiles) expect(spellProfile(row)).toBe(row.spelled)
  for (const row of parts.commands) expect(spellCommand({ job: row.job, cmd: row.proposed ?? '' })).toBe(row.spelled)
})

test('the plan holds a row of each kind, so that the test above checks each spelling', () => {
  expect(parts.ports.some((row) => !row.variable)).toBe(true)
  expect(parts.spaces.some((row) => row.remove)).toBe(true)
  expect(parts.spaces.some((row) => !row.name)).toBe(true)
  expect(parts.commands.length).toBeGreaterThan(0)
  expect(parts.profiles.length).toBeGreaterThan(1)
})
