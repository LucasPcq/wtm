import type { LucideIcon } from 'lucide-react'

export function PlanSentence({ icon: Icon, children }: { icon: LucideIcon; children: string }) {
  return (
    <li className="flex items-center gap-3 text-sm">
      <Icon className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
      <span title={children} className="min-w-0 flex-1 truncate">
        {children}
      </span>
    </li>
  )
}
