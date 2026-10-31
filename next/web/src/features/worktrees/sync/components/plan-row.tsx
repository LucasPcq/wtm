import { ArrowDownToLine, ArrowUpFromLine, type LucideIcon } from 'lucide-react'

const glyph: Record<'forward' | 'push', { icon: LucideIcon; label: string }> = {
  forward: { icon: ArrowDownToLine, label: "Takes origin's commits" },
  push: { icon: ArrowUpFromLine, label: 'Pushed only' },
}

// a Confirm row: its rebase's number in run order, or, for what the count leaves out (a branch fast-forwarded first,
// a push alone), its glyph; the geometry of an outcome row, so the list reads the same once it runs
export function PlanRow({
  lead,
  title,
  message,
}: {
  lead: number | 'forward' | 'push'
  title: string
  message: string
}) {
  const Glyph = typeof lead === 'number' ? undefined : glyph[lead]

  return (
    <li aria-label={title} className="flex min-h-8 items-center gap-3 text-sm">
      {Glyph ? (
        <Glyph.icon className="size-4 shrink-0 text-muted-foreground" aria-label={Glyph.label} />
      ) : (
        <span className="flex size-4 shrink-0 items-center justify-center rounded-full border text-[10px] text-muted-foreground">
          {lead}
        </span>
      )}
      <span className="shrink-0 font-medium">{title}</span>
      <span className="min-w-0 flex-1 text-muted-foreground">{message}</span>
    </li>
  )
}
