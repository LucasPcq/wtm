import { expect, test } from 'vitest'
import { itemWarnings } from '@/features/worktrees/utils/outcome'

const warning = { code: 'exec.other', message: 'something the screen does not word' }

// A warning the screen does not word showed its core text twice: as its line, and again behind its chevron.
test("a warning the screen words its own way keeps the core's text behind its chevron, and one it does not, not twice", () => {
  const [worded] = itemWarnings([{ subject: 'feat/a', warnings: [warning] }], () => 'Worded')
  const [unworded] = itemWarnings(
    [{ subject: 'feat/a', warnings: [warning] }],
    (w) => w.message.charAt(0).toUpperCase() + w.message.slice(1),
  )

  expect(worded?.output).toEqual({ lines: ['something the screen does not word'] })
  expect(unworded?.message).toBe('Something the screen does not word')
  expect(unworded?.output).toBeUndefined()
})
