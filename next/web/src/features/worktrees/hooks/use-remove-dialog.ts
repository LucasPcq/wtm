import { create } from 'zustand'

// a store, not the dialog's state: the detail's menu opens the dialog the page header holds
export const useRemoveDialog = create<{ open: boolean; preset: string[] }>(() => ({ open: false, preset: [] }))

export const openRemove = (preset: string[]) => useRemoveDialog.setState({ open: true, preset })
