import { Children, isValidElement, type ReactNode, useId } from 'react'
import { ItemBox } from '@/features/worktrees/components/item-box'

// the rows a plan leaves out, under their own label: "Left as they are", or a refusal's "Can't sync"
export function LeftSection({ label = 'Left as they are', children }: { label?: string; children: ReactNode }) {
  const heading = useId()
  if (!Children.toArray(children).some(isValidElement)) return null

  return (
    <section aria-labelledby={heading} className="flex flex-col gap-2">
      <h3 id={heading} className="text-xs font-medium text-muted-foreground">
        {label}
      </h3>
      <ItemBox>{children}</ItemBox>
    </section>
  )
}
