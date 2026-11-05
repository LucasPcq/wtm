import { useReducer } from 'react'
import type { LogQuery } from '@/features/worktrees/running/utils/log-search'

// what the whole log is read for, past the lines held: a search, or the lines from a time
export type Whole = { search: LogQuery } | { since: string }

// why the view jumped to a line, said under the toolbar
export type Note =
  | { kind: 'error' }
  | { kind: 'time'; time: string }
  | { kind: 'older'; time: string }
  | { kind: 'after'; time: string }

export type LogView = {
  query: LogQuery
  only: boolean
  // the key of the match the view is on (lineKey), none for the last: a line keeps its key as lines come and go
  anchor: number | undefined
  // the key of a line the view jumped to that no search names (a time); the latest error is the last of the lines
  // held, as they come
  focus: number | undefined
  note: Note | undefined
  follow: boolean
  wrap: boolean
  stamps: boolean
  whole: Whole | undefined
}

type Action =
  | { type: 'search'; change: Partial<LogQuery> }
  | { type: 'step'; anchor: number }
  | { type: 'only'; only: boolean }
  | { type: 'jump'; focus: number | undefined; note: Note }
  | { type: 'follow'; follow: boolean }
  | { type: 'wrap' }
  | { type: 'stamps' }
  | { type: 'whole'; whole: Whole | undefined }
  | { type: 'reconnect' }

const initial: LogView = {
  query: { text: '', matchCase: false, regex: false },
  only: false,
  anchor: undefined,
  focus: undefined,
  note: undefined,
  follow: true,
  wrap: false,
  stamps: true,
  whole: undefined,
}

function reduce(view: LogView, action: Action): LogView {
  switch (action.type) {
    case 'search': {
      const query = { ...view.query, ...action.change }
      const cleared = { ...view, query, anchor: undefined, focus: undefined, note: undefined }
      // the whole log was searched for the query before: a new one reads the lines held again
      if (view.whole && 'search' in view.whole) return { ...cleared, whole: undefined, follow: true }
      return cleared
    }
    case 'step':
      return { ...view, anchor: action.anchor, focus: undefined, note: undefined, follow: false }
    case 'only':
      return { ...view, only: action.only }
    case 'jump':
      return { ...view, focus: action.focus, note: action.note, follow: false }
    case 'follow':
      return { ...view, follow: action.follow, ...(action.follow && { focus: undefined, note: undefined }) }
    case 'wrap':
      return { ...view, wrap: !view.wrap }
    case 'stamps':
      return { ...view, stamps: !view.stamps }
    // a stream read again numbers its lines from the start: a key of the one before names another line
    case 'reconnect':
      return { ...view, anchor: undefined, focus: undefined, note: undefined, follow: true }
    case 'whole':
      return {
        ...view,
        whole: action.whole,
        focus: undefined,
        note: undefined,
        anchor: undefined,
        follow: !action.whole,
      }
  }
}

export const useLogView = () => useReducer(reduce, initial)
export type LogDispatch = ReturnType<typeof useLogView>[1]
