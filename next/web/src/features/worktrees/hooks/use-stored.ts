import { useSyncExternalStore } from 'react'

// A choice a viewer makes for themselves (the columns, the sections) is kept in this browser only: storage may be
// refused (a private window), and the page then keeps it in memory, for itself, until it closes.
const listeners = new Set<() => void>()
const refused = new Map<string, string>()

function subscribe(listener: () => void) {
  listeners.add(listener)
  addEventListener('storage', listener)
  return () => {
    listeners.delete(listener)
    removeEventListener('storage', listener)
  }
}

function read(key: string): string | null {
  const kept = refused.get(key)
  if (kept !== undefined) return kept
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function parse(raw: string | null): unknown {
  try {
    return raw === null ? undefined : JSON.parse(raw)
  } catch {
    return undefined
  }
}

// useStored is the value stored under key, undefined when none is or it does not read, and how to store another.
export function useStored(key: string): [unknown, (value: unknown) => void] {
  const raw = useSyncExternalStore(subscribe, () => read(key))
  const store = (value: unknown) => {
    const raw = JSON.stringify(value)
    try {
      localStorage.setItem(key, raw)
      refused.delete(key)
    } catch {
      refused.set(key, raw)
    }
    listeners.forEach((listener) => listener())
  }
  return [parse(raw), store]
}
