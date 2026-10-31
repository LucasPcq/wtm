import type { ComponentProps, ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

// aria-disabled, not disabled: a disabled button takes no focus nor pointer, so its reason would never show
export function ReasonButton({
  disabled,
  why,
  onClick,
  variant,
  children,
}: {
  disabled: boolean
  variant?: ComponentProps<typeof Button>['variant']
  why: string | undefined
  onClick: () => void
  children: ReactNode
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant={variant}
          aria-disabled={disabled}
          className="aria-disabled:cursor-not-allowed aria-disabled:opacity-50"
          onClick={() => {
            if (!disabled) onClick()
          }}
        >
          {children}
        </Button>
      </TooltipTrigger>
      {disabled && why && <TooltipContent>{why}</TooltipContent>}
    </Tooltip>
  )
}
