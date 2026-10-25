import type { ReactNode } from 'react'
import { type SectionId, sectionTitle } from '@/features/worktrees/detail/sections'

export function Section({ id, meta = '', children }: { id: SectionId; meta?: string; children: ReactNode }) {
  return (
    <section aria-label={sectionTitle[id]} className="flex flex-col gap-2.5 border-t pt-4">
      <div className="flex items-baseline gap-2">
        <h3 className="text-sm font-semibold">{sectionTitle[id]}</h3>
        <span className="ml-auto text-xs text-muted-foreground">{meta}</span>
      </div>
      {children}
    </section>
  )
}
