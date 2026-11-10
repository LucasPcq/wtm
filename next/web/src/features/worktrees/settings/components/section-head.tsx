import type { LucideIcon } from 'lucide-react'

export function SectionHead({ icon: Icon, title, line }: { icon: LucideIcon; title: string; line: string }) {
  return (
    <div className="flex items-start gap-3">
      <span className="flex size-6 shrink-0 items-center justify-center rounded-md bg-muted">
        <Icon className="size-3.5" aria-hidden="true" />
      </span>
      <div className="flex flex-col gap-1">
        <h2 className="text-base font-semibold">{title}</h2>
        <p className="text-sm text-muted-foreground">{line}</p>
      </div>
    </div>
  )
}
