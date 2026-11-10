import type { EnvKeyStatus, EnvStatus, EnvshowCheck, EnvshowFollows, EnvshowKey, EnvshowOrigin } from '@/api/types.gen'

// the core publishes what a key follows and where a file comes from; the screen says each its own way, so a value it
// adds fails the build until it has words
export const followsWords: Record<EnvshowFollows, (key: EnvshowKey) => string> = {
  port: ({ job }) => `${job}'s port`,
  address: ({ job }) => `${job}'s address`,
  value: ({ job }) => (job ? `${job}'s value` : 'a value of run.toml'),
  owned: () => "wtm's own",
  nothing: () => 'follows nothing',
  copied: () => 'copied as it is',
}

export const originWords: Record<EnvshowOrigin, (parent: string | undefined) => string> = {
  template: () => 'copied from its template',
  main: () => "copied from main's",
  parent: (parent) => `copied from ${parent ?? 'the parent'}'s`,
  fallback: (parent) => `copied from main's: ${parent ?? 'the parent'} has none`,
  own: () => "the main checkout's own",
  none: () => 'not copied: its source has none',
}

export const statusWords: Record<EnvKeyStatus, string | undefined> = {
  in_step: undefined,
  add: 'not here yet: Reconcile adds it',
  missing: 'no source gives it a value',
  conflict: 'differs from its source',
  orphan: 'in no source',
}

const leftAloneWords: Record<EnvStatus, string> = {
  missing_key: 'the key is in no file',
  base_not_found: 'wtm finds no port of its job in it',
  ambiguous: "its job's port appears more than once",
  foreign_host: 'it points at another host, which no job here serves',
  secure_scheme: 'https: the run proxy serves plain HTTP',
  rewrite: 'wtm rewrites it',
  unchanged: 'in step',
}

export const checkWords: Record<EnvshowCheck['kind'], (check: EnvshowCheck) => { fact: string; why: string }> = {
  unlinked: ({ job, port, file }) => ({
    fact: job ? `follows nothing: looks like ${job}'s ${port}` : 'follows nothing: holds an address of this machine',
    why: `In ${file}. Until it is linked, every worktree reaches the same one.`,
  }),
  left_alone: ({ status, file }) => ({
    fact: `left alone: ${status ? leftAloneWords[status] : 'wtm writes nothing in it'}`,
    why: `In ${file}. Set it by hand to its job's port, or link it again.`,
  }),
  out_of_step: ({ here, writes, file }) => ({
    fact:
      here !== undefined && writes !== undefined ? `out of step: ${here} here, wtm writes ${writes}` : 'out of step',
    why: `In ${file}. Reconcile writes what wtm writes.`,
  }),
}
