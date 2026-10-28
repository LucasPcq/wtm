import { ReasonButton } from '@/features/worktrees/components/reason-button'

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
    <ReasonButton disabled={disabled} why={why} onClick={onClick}>
      Next
    </ReasonButton>
  )
}
