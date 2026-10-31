import { TriangleAlert } from 'lucide-react'
import { Checkbox } from '@/components/ui/checkbox'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { toneIcon } from '@/features/worktrees/components/glyph'

export type Box = {
  checked: boolean
  disabled: boolean
  label: string
  onCheck?: ((on: boolean) => void) | undefined
  tooltip?: string | undefined
}

// one row of Sync's tree: its box (none for a branch it can't move), its name, an amber reason, then muted lines
export function StackRow({
  name,
  box,
  attention = [],
  lines,
}: {
  name: string
  box: Box | undefined
  attention?: string[]
  lines: string[]
}) {
  const checkbox = box && (
    <Checkbox
      aria-label={box.label}
      // Chrome sends no pointer event through a disabled button: the wrapper takes it, so the tooltip shows
      className="disabled:pointer-events-none"
      checked={box.checked}
      disabled={box.disabled}
      onCheckedChange={(on) => box.onCheck?.(on === true)}
    />
  )

  return (
    <div className="flex items-start gap-3 py-2 pr-4">
      <span className="flex h-5 w-4 shrink-0 items-center">
        {box?.tooltip ? (
          <Tooltip>
            <TooltipTrigger asChild>
              {/* the wrapper takes the pointer a disabled box refuses, so its reason shows */}
              {/* biome-ignore lint/a11y/noNoninteractiveTabindex: the reason shows to a keyboard as to a pointer */}
              <span tabIndex={0} className="flex">
                {checkbox}
              </span>
            </TooltipTrigger>
            <TooltipContent>{box.tooltip}</TooltipContent>
          </Tooltip>
        ) : (
          checkbox
        )}
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="text-sm leading-5 font-medium">{name}</span>
        {attention.map((line) => (
          <span key={line} className="flex items-center gap-1.5 text-xs">
            <TriangleAlert className={toneIcon({ tone: 'attention' })} aria-hidden="true" />
            {line}
          </span>
        ))}
        {lines.map((line) => (
          <span key={line} className="text-xs text-muted-foreground">
            {line}
          </span>
        ))}
      </div>
    </div>
  )
}
