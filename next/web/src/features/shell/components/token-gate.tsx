import { type ReactNode, useSyncExternalStore } from 'react'
import { onTokenChange, token } from '@/api/token'
import { StatePage } from '@/components/state-page'
import { TooltipProvider } from '@/components/ui/tooltip'

export function TokenGate({ children }: { children: ReactNode }) {
  if (useSyncExternalStore(onTokenChange, token) !== null) return <TooltipProvider>{children}</TooltipProvider>

  return (
    <main className="flex min-h-svh items-center justify-center p-4">
      <StatePage title="Open wtm from your terminal" command="wtm-next ui">
        This page needs the address wtm prints, which carries the key to its server. Run this in a repository:
      </StatePage>
    </main>
  )
}
