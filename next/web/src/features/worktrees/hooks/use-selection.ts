import { create } from 'zustand'

// a store, not the table's state: the selection bar over the table acts on what the rows check
export const useSelections = create<Record<string, string[] | undefined>>(() => ({}))

export function useSelection(repo: string) {
  const selected = useSelections((selections) => selections[repo]) ?? []
  const set = (subjects: string[]) => useSelections.setState({ [repo]: subjects })

  return {
    selected,
    set,
    toggle: (subject: string) =>
      set(selected.includes(subject) ? selected.filter((s) => s !== subject) : [...selected, subject]),
    clear: () => set([]),
  }
}
