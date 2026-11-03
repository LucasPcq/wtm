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
  | { kind: 'no-error' }

export type LogView = {
  query: LogQuery
  only: boolean
  // which match the view is on, counted from the last: a log grows at its end
  fromLast: number
  // a line the view jumped to that no search names (latest error, a time)
  focus: number | undefined
  note: Note | undefined
  follow: boolean
  wrap: boolean
  stamps: boolean
  whole: Whole | undefined
}

type Action =
  | { type: 'search'; change: Partial<LogQuery> }
  | { type: 'step'; by: 1 | -1; count: number }
  | { type: 'only'; only: boolean }
  | { type: 'jump'; focus: number | undefined; note: Note }
  | { type: 'follow'; follow: boolean }
  | { type: 'wrap' }
  | { type: 'stamps' }
  | { type: 'whole'; whole: Whole | undefined }

const initial: LogView = {
  query: { text: '', matchCase: false, regex: false },
  only: false,
  fromLast: 0,
  focus: undefined,
  note: undefined,
  follow: true,
  wrap: false,
  stamps: true,
  whole: undefined,
}

function reduce(view: LogView, action: Action): LogView {
  switch (action.type) {
    case 'search':
      return { ...view, query: { ...view.query, ...action.change }, fromLast: 0, focus: undefined, note: undefined }
    case 'step': {
      if (action.count === 0) return view
      const fromLast = (view.fromLast - action.by + action.count) % action.count
      return { ...view, fromLast, focus: undefined, note: undefined, follow: false }
    }
    case 'only':
      return { ...view, only: action.only }
    case 'jump':
      return { ...view, focus: action.focus, note: action.note, follow: action.focus === undefined && view.follow }
    case 'follow':
      return { ...view, follow: action.follow, ...(action.follow && { focus: undefined, note: undefined }) }
    case 'wrap':
      return { ...view, wrap: !view.wrap }
    case 'stamps':
      return { ...view, stamps: !view.stamps }
    case 'whole':
      return { ...view, whole: action.whole, focus: undefined, note: undefined, fromLast: 0, follow: !action.whole }
  }
}

export const useLogView = () => useReducer(reduce, initial)
export type LogDispatch = ReturnType<typeof useLogView>[1]
