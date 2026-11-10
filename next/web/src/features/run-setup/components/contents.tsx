import { Map as MapIcon } from 'lucide-react'
import { PartMark, stateWords } from '@/features/run-setup/components/part-mark'
import type { Open } from '@/features/run-setup/hooks/use-run-setup'
import type { PartState } from '@/features/run-setup/utils/setup'
import { type Part, parts, partWords } from '@/features/run-setup/utils/words'
import { cn } from '@/lib/utils'

export function Contents({
  states,
  open,
  reachable,
  onOpen,
}: {
  states: Record<Part, PartState>
  open: Open
  reachable: (part: Part) => boolean
  onOpen: (to: Open) => void
}) {
  const item = 'flex w-full items-start gap-3 rounded-md px-3 py-2 text-left text-sm disabled:cursor-default'

  return (
    <nav aria-label="Contents" className="sticky top-6 flex w-50 shrink-0 flex-col gap-1 self-start">
      <button
        type="button"
        className={cn(item, open === 'overview' ? 'bg-muted' : 'hover:bg-muted/50')}
        aria-current={open === 'overview' ? 'step' : undefined}
        onClick={() => onOpen('overview')}
      >
        <MapIcon className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <span className="flex flex-col">
          <span className="font-medium">Overview</span>
          <span className="text-muted-foreground">{open === 'overview' ? 'In progress' : "What you'll set up"}</span>
        </span>
      </button>
      {parts.map((part, at) => {
        const state = states[part]
        const greyed = state === 'skipped' || !reachable(part)

        return (
          <button
            key={part}
            type="button"
            disabled={greyed}
            className={cn(item, open === part ? 'bg-muted' : 'enabled:hover:bg-muted/50', greyed && 'opacity-50')}
            aria-current={open === part ? 'step' : undefined}
            onClick={() => onOpen(part)}
          >
            <PartMark state={state} number={at + 1} />
            <span className="flex flex-col">
              <span className="font-medium">{partWords[part].title}</span>
              <span className="text-muted-foreground">{stateWords[state]}</span>
            </span>
          </button>
        )
      })}
    </nav>
  )
}
