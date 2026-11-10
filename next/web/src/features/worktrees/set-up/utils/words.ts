import type { DomainOrigin, InitRequest } from '@/api/types.gen'

export type Setting = InitRequest[number]['path']
export type Section = 'worktrees' | 'env_files' | 'hooks'
export type HookList = Extract<Setting, 'on_create' | 'on_clean'>

export const sectionTitle: Record<Section, string> = { worktrees: 'Worktrees', env_files: '.env files', hooks: 'Hooks' }

export const isSection = (id: string): id is Section => Object.hasOwn(sectionTitle, id)

export const settingLabel: Record<Setting, string> = {
  base_path: 'Folder for new worktrees',
  base_branch: 'Branch they start from',
  env_strategy: 'Where .env files come from',
  env_files: 'Files a worktree gets',
  on_create: 'When a worktree is created',
  on_clean: 'Before a worktree is removed',
}

// the line under a value says where it came from: the core gives the origin, each screen words it
export const folderHelp = (name: string): Record<DomainOrigin, string> => ({
  default: `The default: beside ${name}, outside its folder.`,
  config: 'As this repository is set up now.',
  request: 'Relative to the repository. Outside it keeps the main checkout clean.',
  remembered: 'Remembered.',
  detected: 'Found in the repository.',
  pull_request: '',
})

export const branchHelp: Record<DomainOrigin, string> = {
  // detected is origin's default branch, else the branch the main worktree is on
  detected: 'Found in the repository. Changing it later only affects new worktrees.',
  config: 'As this repository is set up now. Changing it only affects new worktrees.',
  default: 'The default. Changing it later only affects new worktrees.',
  request: 'Changing it later only affects new worktrees.',
  remembered: 'Remembered.',
  pull_request: '',
}

export type Strategy = Exclude<Extract<InitRequest[number], { path: 'env_strategy' }>['values'][number], ''>

export const strategyCard: Record<Strategy, { title: string; example?: string; line: string }> = {
  example: { title: 'Copy the template', example: '.env.example → .env', line: 'From its committed template.' },
  main: { title: "Copy main's", example: "main's .env", line: "With main's real values." },
  parent: { title: "Copy the parent's", example: "the parent's .env", line: 'From the worktree it starts from.' },
  none: { title: "Don't copy them", line: 'Each worktree starts without .env files.' },
}

export const isStrategy = (value: string): value is Strategy => Object.hasOwn(strategyCard, value)

export const fileFound = (template: string | undefined) => (template ? 'template beside it' : 'found as it is')

export const hookWhere = (cwd: string | undefined) => (cwd ? `in ${cwd}` : "in the worktree's root")

export const foundLine = (files: string[]) => {
  if (files.length === 0) return undefined
  const listed = files.length === 1 ? files[0] : `${files.slice(0, -1).join(', ')} and ${files.at(-1)}`
  return `Found in ${listed}.`
}

export const hooksRun = 'A hook that fails stops the creation, or the removal, unless it is set to go on.'

// what a screen reader hears while a hook is dragged: its command and its place, never the row's inner id
const named = (cmd: string | undefined) => cmd || 'This command'
export const dragWords = {
  instructions:
    'To move a command, press space or enter, move it with the arrow keys, then press space or enter to drop it.',
  picked: (cmd: string | undefined, at: number, of: number) => `Picked up ${named(cmd)}, ${at} of ${of}.`,
  over: (cmd: string | undefined, at: number, of: number) => `${named(cmd)} moved to ${at} of ${of}.`,
  dropped: (cmd: string | undefined, at: number, of: number) => `${named(cmd)} dropped at ${at} of ${of}.`,
  back: (cmd: string | undefined, at: number, of: number) => `${named(cmd)} put back at ${at} of ${of}.`,
}
