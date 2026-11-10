import type { DomainSection } from '@/api/types.gen'

export const parts = ['runs', 'shared', 'ports', 'addresses', 'env', 'profiles'] as const
export type Part = (typeof parts)[number]

export const isPart = (id: string): id is Part => (parts as readonly string[]).includes(id)

export const partWords: Record<Part, { title: string; why: string; decides: string; preview: string }> = {
  runs: {
    title: 'What runs',
    why: 'Everything wtm can start: a service keeps running, a task runs once.',
    decides: 'the jobs, from your scripts and docker-compose.yml',
    preview: 'Runs',
  },
  shared: {
    title: 'Shared or per worktree',
    why: 'Shared runs one copy for every worktree; per worktree runs one each.',
    decides: 'one copy, or one per worktree, for each job',
    preview: 'Data',
  },
  ports: {
    title: 'Ports',
    why: 'Each job gets a port per worktree, through the variable it reads.',
    decides: 'a port per worktree, for each per-worktree job',
    preview: 'Ports',
  },
  addresses: {
    title: 'Addresses',
    why: 'A job answering under its name is reached the same way in every worktree.',
    decides: 'a name per job, on its port',
    preview: 'Addresses',
  },
  env: {
    title: '.env files',
    why: 'Keys that point at a port or an address follow each worktree.',
    decides: 'keys that follow ports, addresses and data',
    preview: '.env written',
  },
  profiles: {
    title: 'Profiles',
    why: 'wtm run up starts a profile: the default one when you name none.',
    decides: 'which jobs wtm run up starts',
    preview: 'wtm run up',
  },
}

// the files each part lands in, as the overview lists them
export const partFiles: Record<Part, string[]> = {
  runs: ['run.toml'],
  shared: ['run.toml', 'docker-compose.yml'],
  ports: ['run.toml', 'docker-compose.yml'],
  addresses: ['run.toml'],
  env: ['run.toml', 'config.toml'],
  profiles: ['run.toml'],
}

const skipped: Record<string, string> = {
  no_compose: 'no docker-compose.yml, nothing to share.',
  no_runner: 'no root script starts the apps.',
  no_profile: 'no job to group.',
  nothing_to_ask: 'nothing to ask.',
}

// a part the core skips says why; a reason this page does not know is still said plainly
export const skippedWhy = (section: DomainSection) => `Skipped: ${skipped[section.reason ?? ''] ?? 'nothing to ask.'}`

export const kindWords: Record<string, string> = { port: 'port', address: 'address', data: 'data', none: '—' }

export const writtenWhere: Record<string, string> = {
  'run.toml': 'in .git/wtm, kept out of git',
  'config.toml': 'in .git/wtm, kept out of git',
}
