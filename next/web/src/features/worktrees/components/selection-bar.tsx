import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'

// the actions on a selection sit after the count; the destructive one always comes last
export function SelectionBar({
  count,
  onClear,
  destructive,
}: {
  count: number
  onClear: () => void
  destructive: ReactNode
}) {
  return (
    <div role="toolbar" aria-label="Selection" className="flex h-[60px] shrink-0 items-center gap-2 border-b px-8">
      <span className="text-sm font-medium">{count} selected</span>
      <Button variant="ghost" size="sm" onClick={onClear}>
        Clear
      </Button>
      <div className="flex-1" />
      {destructive}
    </div>
  )
}
