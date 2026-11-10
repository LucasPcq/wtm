import { FileKey, FolderTree, type LucideIcon, SquareTerminal } from 'lucide-react'
import type { DomainField, DomainFieldError, ProjectEnvStrategy } from '@/api/types.gen'

export type SettingPath = 'base_path' | 'base_branch' | 'env_strategy' | 'env_files' | 'on_create' | 'on_clean'
export type Setting = DomainField & { path: SettingPath }

export const settingWords: Record<SettingPath, { label: string; help: string }> = {
  base_path: {
    label: 'Folder for new worktrees',
    help: 'Relative to the main checkout. Worktrees already made stay where they are.',
  },
  base_branch: { label: 'Branch they start from', help: 'When you create one without naming where it starts.' },
  env_strategy: { label: 'Where .env files come from', help: 'How a new worktree gets its .env files.' },
  env_files: { label: 'Files a worktree gets', help: 'Each .env file copied into a new worktree.' },
  on_create: { label: 'When a worktree is created', help: 'Commands run in it, in order, once it is made.' },
  on_clean: { label: 'Before a worktree is removed', help: 'Commands run in it, in order, before it goes.' },
}

export const isSettingPath = (path: string): path is SettingPath => path in settingWords

export const isSetting = (field: DomainField): field is Setting => isSettingPath(field.path)

export type SectionId = 'worktrees' | 'env_files' | 'hooks'

export const isSectionId = (id: string): id is SectionId => id in sectionWords

export const sectionWords: Record<SectionId, { title: string; line: string; icon: LucideIcon }> = {
  worktrees: { title: 'Worktrees', line: 'Where new worktrees go and what they start from.', icon: FolderTree },
  env_files: { title: '.env files', line: 'How a new worktree gets its .env files.', icon: FileKey },
  hooks: {
    title: 'Hooks',
    line: 'Commands run in a worktree once it is created or before it is removed.',
    icon: SquareTerminal,
  },
}

export type Strategy = ProjectEnvStrategy | 'none'

export const strategyWords: Record<Strategy, { title: string; example: string; line: string; short: string }> = {
  example: {
    title: 'Copy the template',
    example: '.env.example → .env',
    line: 'From its committed template.',
    short: 'the template',
  },
  main: { title: "Copy main's", example: "main's .env", line: "With main's real values.", short: "main's" },
  parent: {
    title: "Copy the parent's",
    example: "the parent's .env",
    line: 'From the worktree it starts from.',
    short: "the parent's",
  },
  none: { title: "Don't copy them", example: 'no .env', line: 'A new worktree gets none.', short: 'none' },
}

export const hookWords: Record<'on_create' | 'on_clean', { stops: string; goesOn: string; fails: string }> = {
  on_create: {
    stops: 'stops the creation if it fails',
    goesOn: 'the creation goes on if it fails',
    fails: 'One that fails stops the creation, unless it is set to go on.',
  },
  on_clean: {
    stops: 'stops the removal if it fails',
    goesOn: 'the removal goes on if it fails',
    fails: 'One that fails stops the removal, unless it is set to go on.',
  },
}

export const rememberedWords: Record<string, { question: string; values: Record<string, string> }> = {
  env_strategy: {
    question: 'Where .env files come from',
    values: { config: 'the setting above', example: 'the template', main: "main's", parent: "the parent's" },
  },
  isolation: { question: 'Isolation', values: { isolated: 'isolated', verbatim: 'verbatim' } },
  source_update: {
    question: 'Update the source branch first',
    values: { ff: 'fast-forward it', keep: 'keep it as is' },
  },
}

const problemSuffix = (code: string) => code.slice(code.lastIndexOf('.') + 1)

// the problem in the screen's words where it has some, else the core's message; a consequence only where the screen knows it
export function problemLine(problem: DomainFieldError): { fact: string; consequence?: string } {
  const value = String(problem.params?.value ?? '')
  const words: Record<string, () => { fact: string; consequence?: string }> = {
    one_of: () => ({
      fact: `“${value}” isn't a known strategy`,
      consequence: 'Create refuses until one is chosen below.',
    }),
    base_branch_unknown: () => ({
      fact: `“${value}” is no branch of the repository`,
      consequence: 'Create refuses a worktree started from it.',
    }),
    base_path_absolute: () => ({ fact: 'not relative to the repository', consequence: 'Create refuses until it is.' }),
    required: () => ({ fact: 'none set', consequence: 'Create refuses until one is set.' }),
  }
  return (
    words[problemSuffix(problem.code)]?.() ?? {
      fact: problem.message,
    }
  )
}

// a remembered answer's problem in the screen's words where it has some, else the core's message
export function rememberedProblem(problem: DomainFieldError): string {
  const value = String(problem.params?.value ?? '')
  const words: Record<string, string> = {
    remembered_unknown: 'No question remembers an answer under this name.',
    remembered_not_allowed: `“${value}” is no answer this question can remember.`,
  }
  return words[problemSuffix(problem.code)] ?? problem.message
}

export const refusalWords = {
  notFound: { title: 'No settings yet', line: 'Set wtm up in this repository: init asks each setting once.' },
  noParse: {
    title: "The settings don't parse",
    fix: 'Fix it in your editor; this page reads it again when you come back to it.',
  },
}

// a field's problem in the dialog, under the field: the screen's words where it has some, else the core's message
export function fieldProblem(problem: DomainFieldError): string {
  const value = String(problem.params?.value ?? '')
  const words: Record<string, string> = {
    base_path_absolute: `Must be relative to the main checkout, not ${value}.`,
    base_branch_unknown: `“${value}” is no branch of the repository.`,
    required: 'Required.',
    env_file_outside: `Must be a file of the repository, not ${value}.`,
    env_file_twice: `${value} is listed twice.`,
    hook_dir_outside: `Must run in a folder of the repository, not ${value}.`,
  }
  return words[problemSuffix(problem.code)] ?? problem.message
}
