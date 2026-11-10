import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { type Part, partWords } from '@/features/run-setup/utils/words'

export function OpenPart({
  part,
  number,
  next,
  onContinue,
  children,
}: {
  part: Part
  number: number
  next: string
  onContinue: () => void
  children: ReactNode
}) {
  return (
    <section aria-label={partWords[part].title} className="flex flex-col gap-6 rounded-lg border p-6">
      <header className="flex gap-3">
        <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-foreground text-xs font-medium text-background">
          {number}
        </span>
        <div className="flex flex-col gap-1">
          <h2 className="text-lg font-semibold">{partWords[part].title}</h2>
          <p className="text-sm text-muted-foreground">{partWords[part].why}</p>
        </div>
      </header>
      {children}
      <div className="flex justify-end">
        <Button onClick={onContinue}>Continue to {next}</Button>
      </div>
    </section>
  )
}
