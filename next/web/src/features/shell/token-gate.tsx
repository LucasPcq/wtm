import { useSyncExternalStore, type ReactNode } from 'react'
import { onTokenChange, token } from '@/api/token'
import { StatePage } from '@/components/state-page'

// TokenGate shows the page only with a token of the server, and else how to open it.
export function TokenGate(props: { children: ReactNode }) {
  if (useSyncExternalStore(onTokenChange, token) !== null) return props.children

  return (
    <main className="flex min-h-svh items-center justify-center p-4">
      <StatePage title="Open wtm from your terminal" command="wtm-next ui">
        This page needs the address wtm prints, which carries the key to its server. Run this in a repository:
      </StatePage>
    </main>
  )
}
