import type { CreateAction, CreateRequest, DomainField, DomainSkip } from '@/api/types.gen'
import type { FieldWords, OptionWords } from '@/components/command-form/command-fields'
import type { OutcomeKind } from '@/features/worktrees/utils/outcome'

// The core words its questions for the command line; the web says them its own way. Each record is exhaustive over
// what the schema publishes, so a question, an option, a reason or a phase the core adds fails the build until it
// has words.

// remember and ask say how the other questions are answered, not what
export type Question = Exclude<CreateRequest[number]['path'], 'remember' | 'ask'>
type ValuesOf<P extends Question> = Exclude<Extract<CreateRequest[number], { path: P }>['values'][number], ''>
type Options<P extends Question> = Record<ValuesOf<P>, OptionWords>

export const envFrom: Options<'env_from'> = {
  config: { title: 'Project default', description: 'The strategy config.toml sets for this project.' },
  example: { title: 'Example', description: 'Copied from .env.example.' },
  main: { title: 'Main', description: "Copied from the main checkout's .env." },
  parent: { title: 'Parent', description: "Copied from the source worktree's .env." },
}

export const isolation: Options<'isolation'> = {
  isolated: { title: 'Isolated', description: 'Its own ports and namespaces: it runs beside the others.' },
  verbatim: { title: 'Verbatim', description: 'The .env as copied: it runs in turn with the others.' },
}

// the answer as the Confirm step's one line says it, beside the others
export const envFromShort: Record<ValuesOf<'env_from'>, string> = {
  config: '.env from the project default',
  example: '.env from .env.example',
  main: ".env from the main checkout's",
  parent: ".env from the source worktree's",
}

export const isolationShort: Record<ValuesOf<'isolation'>, string> = {
  isolated: 'isolated',
  verbatim: '.env kept verbatim',
}

const fastForward: Options<'fast_forward'> = {
  true: { title: 'Update it first', description: 'Fast-forward the source branch to origin, then branch from it.' },
  false: { title: 'Keep it as it is', description: 'Branch from where the source stands now.' },
}

const ifNotExists: Options<'if_not_exists'> = {
  true: { title: 'Succeed', description: 'Leave the worktree as it is.' },
  false: { title: 'Refuse', description: 'Stop on a worktree that exists.' },
}

export const createWords: Record<Question, FieldWords> = {
  branches: { label: 'Branches', help: 'One per line; each gets its own worktree.' },
  from: { label: 'Source branch', help: 'Where a new branch starts, and the parent it syncs onto.' },
  fast_forward: {
    label: 'Source update',
    help: 'Whether to bring the source branch up to origin before branching from it.',
    options: fastForward,
  },
  env_from: {
    label: 'Env files',
    help: "Where the new worktree's .env files are copied from.",
    options: envFrom,
  },
  isolation: {
    label: 'Isolation',
    help: 'How it runs next to the other worktrees.',
    options: isolation,
    config: 'from run.toml',
  },
  if_not_exists: {
    label: 'If it exists',
    help: 'What to do when the worktree is there already.',
    options: ifNotExists,
  },
}

export type Asked = DomainField & { path: Question }

export const isAsked = (field: DomainField): field is Asked => field.path in createWords

// null for a reason that means nothing in a browser
export const skipWords: Record<DomainSkip, string | null> = {
  flag_only: null,
  all: null,
  exists: 'the worktree exists, so nothing is created',
  not_behind: 'the source branch is up to date with origin',
  held: 'the source holds the changes being extracted',
  run_unread: "run.toml couldn't be read",
  nothing_isolated: 'run.toml declares nothing to isolate',
  no_conflict: 'nothing conflicts',
  no_orphan: 'no child worktree is left behind',
  safe: 'every worktree is safe to remove',
  no_candidate: 'no worktree is done with',
}

export const actionWords: Record<CreateAction, string> = {
  create: 'New branch',
  check_out: 'Existing branch, as it is',
  resume: 'From origin, tracking it',
  unchanged: 'Already open, left as it is',
}

export const outcomeWords: Record<OutcomeKind, string> = {
  failed: "Couldn't create",
  check: 'To check',
  done: 'Ready',
}
