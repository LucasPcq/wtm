import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Separator } from '@/components/ui/separator'

// the actions on a selection sit after the count; the destructive one always comes last, apart
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
      <Checkbox checked aria-label="Clear the selection" onCheckedChange={onClear} />
      <span className="text-sm font-medium">{count} selected</span>
      <Button variant="ghost" size="sm" onClick={onClear}>
        Clear
      </Button>
      <div className="flex-1" />
      <Separator orientation="vertical" className="mx-1 !h-5" />
      {destructive}
    </div>
  )
}
