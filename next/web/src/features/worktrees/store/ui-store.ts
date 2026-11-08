import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

// a group folded or opened against its default, a job pinned to the top, a group whose ended jobs are shown
type Kind = 'columns' | 'sections' | 'folded' | 'pinned' | 'quiet'

export type Chip = 'all' | 'running' | 'crashed'
export type ShownState = 'running' | 'crashed' | 'ended'
// what the Running page shows: a chip, one worktree (`here`, a branch, `shared`) or all, and the states it lists
export type RunFilter = { chip: Chip; worktree: string | undefined; states: ShownState[] }
export const calm: RunFilter = { chip: 'all', worktree: undefined, states: ['running', 'crashed'] }

type Ui = Record<Kind, Record<string, string[]>> & {
  running: Record<string, RunFilter>
  choose: (kind: Kind, repo: string, chosen: string[] | undefined) => void
  filter: (repo: string, change: Partial<RunFilter> | undefined) => void
}

// A browser that refuses storage (a private window) keeps the choice in memory until the page closes.
const forgiving = createJSONStorage(() => ({
  getItem: (key: string) => {
    try {
      return localStorage.getItem(key)
    } catch {
      return null
    }
  },
  setItem: (key: string, value: string) => {
    try {
      localStorage.setItem(key, value)
    } catch {
      return
    }
  },
  removeItem: (key: string) => localStorage.removeItem(key),
}))

export const useUi = create<Ui>()(
  persist(
    (set) => ({
      columns: {},
      sections: {},
      folded: {},
      pinned: {},
      quiet: {},
      running: {},
      choose: (kind, repo, chosen) => set((ui) => ({ [kind]: { ...ui[kind], [repo]: chosen } })),
      filter: (repo, change) =>
        set((ui) => {
          const { [repo]: _, ...others } = ui.running
          return { running: change ? { ...others, [repo]: { ...(ui.running[repo] ?? calm), ...change } } : others }
        }),
    }),
    {
      name: 'wtm.ui',
      storage: forgiving,
      partialize: ({ columns, sections, folded, pinned, quiet, running }) => ({
        columns,
        sections,
        folded,
        pinned,
        quiet,
        running,
      }),
    },
  ),
)

// another tab's choice shows here at once: the browser tells every other tab of the origin when storage changes
addEventListener('storage', (event) => {
  if (event.key === useUi.persist.getOptions().name) void useUi.persist.rehydrate()
})

export function useChoice<T extends string>(kind: Kind, repo: string, order: readonly T[], fallback: readonly T[]) {
  const stored = useUi((ui) => ui[kind][repo])
  const choose = useUi((ui) => ui.choose)
  const shown = stored ? order.filter((id) => stored.includes(id)) : [...fallback]

  return {
    shown,
    toggle: (id: T) =>
      choose(
        kind,
        repo,
        order.filter((known) => (known === id ? !shown.includes(id) : shown.includes(known))),
      ),
    reset: () => choose(kind, repo, undefined),
  }
}

// a set of a viewer's per repository, each member toggled on its own
export function useMarks(kind: 'folded' | 'pinned' | 'quiet', repo: string) {
  const marked = useUi((ui) => ui[kind][repo]) ?? []
  const choose = useUi((ui) => ui.choose)

  return {
    has: (id: string) => marked.includes(id),
    toggle: (id: string) =>
      choose(kind, repo, marked.includes(id) ? marked.filter((one) => one !== id) : [...marked, id]),
  }
}

export function useRunFilter(repo: string) {
  const stored = useUi((ui) => ui.running[repo])
  const filter = useUi((ui) => ui.filter)

  return {
    filter: stored ?? calm,
    change: (change: Partial<RunFilter>) => filter(repo, change),
    clear: () => filter(repo, undefined),
  }
}
