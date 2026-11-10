import { ChevronDown, TriangleAlert } from 'lucide-react'
import type { EnvshowCheck } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { checkWords } from '@/features/worktrees/env-files/utils/words'

// each key the core says to check, with its fact and the one action of the moment; a key that follows nothing waits on
// Link a key, which comes with run.toml's editing
export function ToCheck({ checks, onReconcile }: { checks: EnvshowCheck[]; onReconcile: () => void }) {
  return (
    <Collapsible defaultOpen className="flex flex-col gap-3">
      <CollapsibleTrigger className="group flex w-fit items-center gap-1.5 text-sm text-muted-foreground">
        <ChevronDown className="size-4 transition-transform group-data-[state=closed]:-rotate-90" aria-hidden="true" />
        To check · {checks.length}
      </CollapsibleTrigger>
      <CollapsibleContent>
        <ul aria-label="To check" className="flex flex-col gap-3">
          {checks.map((check) => {
            const { fact, why } = checkWords[check.kind](check)
            return (
              <li key={`${check.file}:${check.key}`} className="flex items-start gap-3">
                <TriangleAlert className="mt-0.5 size-4 shrink-0 text-amber-500" aria-hidden="true" />
                <div className="flex min-w-0 flex-1 flex-col gap-1">
                  <p className="truncate text-sm">
                    <span className="font-mono">{check.key}</span> <span className="text-muted-foreground">{fact}</span>
                  </p>
                  <p className="truncate text-xs text-muted-foreground">{why}</p>
                </div>
                {check.kind === 'out_of_step' && (
                  <Button variant="outline" size="sm" onClick={onReconcile}>
                    Reconcile
                  </Button>
                )}
              </li>
            )
          })}
        </ul>
      </CollapsibleContent>
    </Collapsible>
  )
}
