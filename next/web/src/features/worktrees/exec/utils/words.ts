import type { DomainError, DomainWarning, OperationStarted } from '@/api/types.gen'
import { sentence } from '@/features/worktrees/utils/sentence'

// the warnings of exec the screen words; one the core adds shows the core's message until it is worded here
type ExecWarning = 'exec.env_unnumbered' | 'exec.env_unavailable'

const warningWords: Record<ExecWarning, string> = {
  'exec.env_unnumbered': 'Without its run env: it has no number yet',
  'exec.env_unavailable': "Without its run env: its .env files couldn't be read",
}

const worded = (code: string): code is ExecWarning => Object.hasOwn(warningWords, code)

export const warningLine = (warning: DomainWarning) =>
  worded(warning.code) ? warningWords[warning.code] : sentence(warning.message)

type Surface = OperationStarted['surface']

const surfaceWords: Record<Surface, string> = {
  cli: 'the terminal',
  tui: 'the terminal',
  web: 'the web',
  api: 'the API',
}

const isSurface = (surface: string | undefined): surface is Surface =>
  surface !== undefined && Object.hasOwn(surfaceWords, surface)

const unknown = 'unknown'

// lock.held names its subject and, when its record could be read, its holder; a holder the event stream shows under
// way holds it, one it does not show (started before the page, or lost by a reconnect) may still
export function lockWords(refusal: DomainError, live: boolean): { title: string; description: string } {
  const { scope, name, command = unknown, surface } = refusal.params ?? {}
  const subject = scope === 'wt' && name ? name : 'The repository'
  const holds = live ? 'holds it' : 'may still hold it'
  if (command === unknown)
    return { title: `${subject} is locked`, description: `Another wtm ${holds}. Try again once it's done.` }

  const article = /^[aeiou]/.test(command) ? 'An' : 'A'
  const from = isSurface(surface) ? ` started from ${surfaceWords[surface]}` : ''
  return {
    title: `${subject} is locked`,
    description: `${article} ${command}${from} ${holds}. Try again once it's done.`,
  }
}

export const lockHeld = 'lock.held'

export const retryCode = 'exec.retry'

export const passedWords = (command: string, items: { subject: string }[]) =>
  items.length === 1 ? `${command} passed in ${items[0]?.subject}.` : `${command} passed in all ${items.length}.`
