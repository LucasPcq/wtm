import { useQuery } from '@tanstack/react-query'
import { ExternalLink, TriangleAlert } from 'lucide-react'
import { dryRunQuery } from '@/api/queries/commands'
import type { DomainAnswered } from '@/api/types.gen'
import { Item, ItemTitle } from '@/components/ui/item'
import { Skeleton } from '@/components/ui/skeleton'
import { Details } from '@/features/worktrees/components/details'
import { Facts } from '@/features/worktrees/components/facts'
import { ItemList } from '@/features/worktrees/components/item-list'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { actionTag, isRowWarning, rowWarningLine } from '@/features/worktrees/create/utils/checkout-words'
import { envFromShort, isolationShort } from '@/features/worktrees/create/utils/words'
import { phaseLabel } from '@/features/worktrees/utils/phases'

export function CheckoutConfirm({ repo, answers }: { repo: string; answers: DomainAnswered[] }) {
  const dry = useQuery(dryRunQuery(repo, 'checkout', answers))

  if (dry.isPending) return <Skeleton className="h-40 w-full" />
  if (dry.isError) return <p className="text-sm text-destructive">{dry.error.message}</p>
  if (dry.data.error) return <p className="text-sm text-destructive">{dry.data.error.message}</p>

  const { plan, warnings } = dry.data
  if (!plan) return null

  const { pull_request: pr } = plan
  const branch = pr.branch
  // the parent strategy with no worktree of the parent copies main's .env, as the plan's Alert says
  const fellBack = warnings.some((warning) => warning.code === 'checkout.env_parent_fallback')
  const forwarded = plan.fast_forward?.includes(branch) ?? false
  const line = [
    `From ${plan.from}`,
    plan.isolation && isolationShort[plan.isolation],
    envFromShort[fellBack ? 'main' : plan.env_from],
    forwarded && 'fast-forwarded first',
  ].filter(Boolean)
  const phases = plan.phases ?? []
  const hooks = plan.on_create ?? []
  const hooked = hooks.length > 0 && phases.includes('on_create')
  const summary = [
    `${phases.length} step${phases.length === 1 ? '' : 's'}`,
    hooked && `on_create hook${hooks.length === 1 ? '' : 's'}`,
  ]
    .filter(Boolean)
    .join(', ')

  return (
    <div className="flex flex-col gap-6">
      <PlanWarnings warnings={warnings.filter((warning) => !isRowWarning(warning))} />
      <ItemList>
        <Item role="listitem" aria-label={branch} className="items-center gap-3 rounded-none px-4 py-3">
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <ItemTitle className="min-w-0">{branch}</ItemTitle>
            {warnings.filter(isRowWarning).map((warning) => (
              <p key={warning.code} className="flex items-center gap-1.5 text-xs">
                <TriangleAlert className="size-3.5 shrink-0 text-amber-500" aria-hidden="true" />
                {rowWarningLine(warning)}
              </p>
            ))}
            <a
              href={pr.url}
              target="_blank"
              rel="noreferrer"
              className="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground"
            >
              <span className="truncate">
                #{pr.number} {pr.title}
              </span>
              <ExternalLink className="size-3 shrink-0" aria-hidden="true" />
            </a>
            <p className="text-xs text-muted-foreground">{line.join(' · ')}</p>
          </div>
          <span className="shrink-0 text-xs text-muted-foreground">{actionTag[plan.action]}</span>
        </Item>
      </ItemList>
      <Details label="Details" summary={summary}>
        <Facts
          facts={[
            { label: 'Folder', values: [plan.path], mono: true },
            {
              label: 'Steps',
              values: [
                phases.map((phase, i) => (i === 0 ? phaseLabel[phase] : phaseLabel[phase].toLowerCase())).join(', '),
              ],
            },
            ...(hooked ? [{ label: 'on_create', values: hooks, mono: true }] : []),
          ]}
        />
      </Details>
    </div>
  )
}
