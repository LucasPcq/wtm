import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

type Kind = 'columns' | 'sections'

type Ui = Record<Kind, Record<string, string[]>> & {
  choose: (kind: Kind, repo: string, chosen: string[] | undefined) => void
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
      choose: (kind, repo, chosen) => set((ui) => ({ [kind]: { ...ui[kind], [repo]: chosen } })),
    }),
    { name: 'wtm.ui', storage: forgiving, partialize: ({ columns, sections }) => ({ columns, sections }) },
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
