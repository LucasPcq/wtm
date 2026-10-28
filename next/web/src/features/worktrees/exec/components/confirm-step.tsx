import { TriangleAlert } from 'lucide-react'
import type { DomainError, DomainWarning, ExecPlan } from '@/api/types.gen'
import { Item, ItemTitle } from '@/components/ui/item'
import { Details } from '@/features/worktrees/components/details'
import { Facts } from '@/features/worktrees/components/facts'
import { ItemList } from '@/features/worktrees/components/item-list'
import { type Left, LeftItem } from '@/features/worktrees/components/left-item'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { LockAlert } from '@/features/worktrees/exec/components/lock-alert'
import { warningLine } from '@/features/worktrees/exec/utils/words'
import { planWide } from '@/features/worktrees/utils/outcome'

export function ConfirmStep({
  plan,
  warnings,
  left,
  held,
  live,
}: {
  plan: ExecPlan
  warnings: DomainWarning[]
  left: Left[]
  held: DomainError | undefined
  live: boolean
}) {
  const targets = plan.targets ?? []

  const where = () => {
    if (targets.length === 1) return 'In 1 worktree'
    return `In ${targets.length} worktrees · ${plan.jobs === 1 ? 'one at a time' : `${plan.jobs} at once`}`
  }

  return (
    <>
      {held && <LockAlert refusal={held} live={live} />}
      <PlanWarnings warnings={warnings.filter(planWide)} />
      <section aria-label="Command" className="flex flex-col gap-2">
        <h3 className="text-xs font-medium text-muted-foreground">Command</h3>
        <pre className="rounded-md bg-muted px-4 py-3 font-mono text-sm break-all whitespace-pre-wrap">
          {plan.command}
        </pre>
      </section>
      <section className="flex flex-col gap-2">
        <h3 className="text-xs font-medium text-muted-foreground">{where()}</h3>
        <ItemList label="Worktrees" left={leftRows(left)}>
          {targets.map((target) => (
            <Item
              key={target.worktree}
              role="listitem"
              aria-label={target.worktree}
              className="flex-col items-start gap-1 rounded-none px-4 py-3"
            >
              <ItemTitle className="min-w-0">{target.worktree}</ItemTitle>
              {warnings
                .filter((warning) => warning.params?.worktree === target.worktree)
                .map((warning) => (
                  <span key={warning.code} className="flex items-center gap-1.5 text-xs">
                    <TriangleAlert className="size-3.5 shrink-0 text-amber-500" aria-hidden="true" />
                    {warningLine(warning)}
                  </span>
                ))}
            </Item>
          ))}
        </ItemList>
      </section>
      <Details label="Details" summary="the folder of each">
        {targets.map((target) => (
          <Facts
            key={target.worktree}
            heading={target.worktree}
            facts={[{ label: 'Folder', values: [target.path], mono: true }]}
          />
        ))}
      </Details>
    </>
  )
}

// every worktree chosen has left the options: nothing is asked, and the step says what became of them
export function NothingLeft({ left }: { left: Left[] }) {
  return (
    <ItemList
      label="Worktrees"
      header={<p className="px-4 py-3 text-sm text-muted-foreground">Nothing left to run in.</p>}
      left={leftRows(left)}
    >
      {[]}
    </ItemList>
  )
}

const leftRows = (left: Left[]) =>
  left.map(({ worktree, why }) => <LeftItem key={worktree} worktree={worktree} why={why} />)
