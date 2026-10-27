import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

// aria-disabled, not disabled: a disabled button takes no focus nor pointer, so its reason would never show
export function NextButton({
  disabled,
  why,
  onClick,
}: {
  disabled: boolean
  why: string | undefined
  onClick: () => void
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          aria-disabled={disabled}
          className="aria-disabled:cursor-not-allowed aria-disabled:opacity-50"
          onClick={() => {
            if (!disabled) onClick()
          }}
        >
          Next
        </Button>
      </TooltipTrigger>
      {disabled && why && <TooltipContent>{why}</TooltipContent>}
    </Tooltip>
  )
}
