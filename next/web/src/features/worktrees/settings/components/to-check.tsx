import { ChevronDown, CircleX } from 'lucide-react'
import { useState } from 'react'
import type { DomainFieldError } from '@/api/types.gen'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { problemLine, type SettingPath, settingWords } from '@/features/worktrees/settings/utils/words'
import { cn } from '@/lib/utils'

export function ToCheck({ problems }: { problems: (DomainFieldError & { setting: SettingPath })[] }) {
  const [open, setOpen] = useState(true)

  if (problems.length === 0) return null

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="flex flex-col gap-2">
      <CollapsibleTrigger className="flex w-fit items-center gap-2 text-xs font-medium text-muted-foreground">
        <ChevronDown className={cn('size-3.5 transition-transform', !open && '-rotate-90')} />
        To check · {problems.length}
      </CollapsibleTrigger>
      <CollapsibleContent className="flex flex-col gap-3">
        {problems.map((problem) => {
          const { fact, consequence } = problemLine(problem)
          return (
            <div key={problem.path} className="grid grid-cols-[auto_1fr] items-center gap-x-3 gap-y-1">
              <CircleX className="size-4 text-destructive" aria-hidden="true" />
              <p className="text-sm">
                <span className="font-medium">{settingWords[problem.setting].label}</span>
                <span className="text-muted-foreground"> · {fact}</span>
              </p>
              <p className="col-start-2 text-xs text-muted-foreground">{consequence}</p>
            </div>
          )
        })}
      </CollapsibleContent>
    </Collapsible>
  )
}
