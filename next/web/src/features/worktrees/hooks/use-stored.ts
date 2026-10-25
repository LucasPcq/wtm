import { useSyncExternalStore } from 'react'

// A choice a viewer makes for themselves (the columns, the sections) is kept in this browser only: storage may be
// refused (a private window), and the page then keeps the default.
const listeners = new Set<() => void>()

function subscribe(listener: () => void) {
  listeners.add(listener)
  addEventListener('storage', listener)
  return () => {
    listeners.delete(listener)
    removeEventListener('storage', listener)
  }
}

function read(key: string): string | null {
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
    try {
      localStorage.setItem(key, JSON.stringify(value))
    } catch {
      // kept for this page only: nothing else to do
    }
    listeners.forEach((listener) => listener())
  }
  return [parse(raw), store]
}
