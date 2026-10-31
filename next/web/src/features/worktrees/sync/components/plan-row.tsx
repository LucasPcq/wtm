import { ArrowDownToLine } from 'lucide-react'

// a Confirm row: its step number in run order, or for a branch fast-forwarded first, which the count leaves out, its
// glyph; the geometry of an outcome row, so the list reads the same once it runs
export function PlanRow({ step, title, message }: { step: number | undefined; title: string; message: string }) {
  return (
    <li aria-label={title} className="flex min-h-8 items-center gap-3 text-sm">
      {step === undefined ? (
        <ArrowDownToLine className="size-4 shrink-0 text-muted-foreground" aria-label="Fast-forwarded first" />
      ) : (
        <span className="flex size-4 shrink-0 items-center justify-center rounded-full border text-[10px] text-muted-foreground">
          {step}
        </span>
      )}
      <span className="shrink-0 font-medium">{title}</span>
      <span className="min-w-0 flex-1 text-muted-foreground">{message}</span>
    </li>
  )
}
