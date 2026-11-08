// the worktree `wtm-next ui` last ran in, per repository: the address it prints carries it, the Running page's
// "this worktree"
const key = (repo: string) => `wtm.here.${repo}`

export const keepHere = (repo: string, worktree: string) => localStorage.setItem(key(repo), worktree)

export const hereIn = (repo: string): string | undefined => localStorage.getItem(key(repo)) ?? undefined
