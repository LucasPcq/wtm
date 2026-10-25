import { SlidersHorizontal } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { type SectionId, sectionIds, sectionTitle } from '@/features/worktrees/detail/sections'
import { ChoicePopover } from '@/features/worktrees/shared/choice-popover'

export function SectionsChooser({ shown, onToggle }: { shown: SectionId[]; onToggle: (id: SectionId) => void }) {
  const hidden = sectionIds.length - shown.length

  return (
    <ChoicePopover
      trigger={
        <Button variant="ghost" size="sm" className="text-muted-foreground">
          <SlidersHorizontal />
          Sections · {hidden === 0 ? 'all shown' : `${hidden} hidden`}
        </Button>
      }
      groups={[{ choices: sectionIds.map((id) => ({ id, label: sectionTitle[id] })) }]}
      shown={shown}
      onToggle={(id) => {
        const section = sectionIds.find((known) => known === id)
        if (section) onToggle(section)
      }}
      side="top"
      className="w-56"
    />
  )
}
