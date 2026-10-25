import { useQuery } from '@tanstack/react-query'
import { CircleAlert, CircleX } from 'lucide-react'
import { dryRunQuery } from '@/api/queries/commands'
import type { DomainAnswered, DomainFieldError } from '@/api/types.gen'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Path } from '@/features/worktrees/components/path'
import { blockerWords, reasonOf } from '@/features/worktrees/remove/utils/words'

export function RemovePlan({
  repo,
  answers,
  picked,
  reparent,
  force,
  onReparent,
  onForce,
}: {
  repo: string
  answers: DomainAnswered[]
  picked: string[]
  reparent: boolean
  force: boolean
  onReparent: (on: boolean) => void
  onForce: (on: boolean) => void
}) {
  const dry = useQuery(dryRunQuery(repo, 'clean', answers))

  if (dry.isPending) return <Skeleton className="h-40 w-full" />
  if (dry.isError) return <p className="text-sm text-destructive">{dry.error.message}</p>

  const { plan, error, warnings } = dry.data
  const removals = plan?.worktrees ?? []
  const moves = plan?.moves ?? []
  const count = error ? picked.length : removals.length

  // a reason the core refuses is on its entry, worktree[i]: the i-th worktree asked
  const refusedOf = (fields: DomainFieldError[]) => {
    const byWorktree = new Map<string, DomainFieldError[]>()
    for (const field of fields) {
      const name = picked[Number(/\[(\d+)\]$/.exec(field.path)?.[1])] ?? field.path
      byWorktree.set(name, [...(byWorktree.get(name) ?? []), field])
    }
    return [...byWorktree]
  }

  return (
    <div className="flex flex-col gap-4">
      <h3 className="font-medium">{count === 1 ? `Remove ${picked[0]}` : `Remove ${count} worktrees`}</h3>

      {error && (
        <section aria-label="Refused" className="flex flex-col gap-3 rounded-lg border border-destructive p-3.5">
          <p className="flex items-center gap-2 text-sm">
            <CircleX className="size-4 text-destructive" />
            {error.code === 'clean.unsafe' ? 'Nothing is removed: what these hold would be lost' : error.message}
          </p>
          {refusedOf(error.fields ?? []).map(([name, reasons]) => (
            <div key={name} className="flex flex-col gap-1 pl-6">
              <span className="text-sm font-medium">{name}</span>
              {reasons.map((reason) => (
                <span key={reason.code} className="flex items-center gap-2 text-sm">
                  <CircleAlert className="size-3.5 text-destructive" />
                  {reasonOf(reason)}
                </span>
              ))}
            </div>
          ))}
        </section>
      )}

      {removals.length > 0 && (
        <ul aria-label="Removed" className="flex flex-col gap-3 rounded-lg border p-3.5">
          {removals.map((removal) => (
            <li key={removal.worktree} className="flex flex-col gap-0.5">
              <span className="text-sm font-medium">{removal.worktree}</span>
              <Path path={removal.path} className="text-xs text-muted-foreground" />
              {(removal.blockers ?? []).map((blocker) => (
                <span key={blocker} className="text-xs text-destructive">
                  {blockerWords[blocker]({
                    count: String(removal.unpushed ?? 0),
                    url: removal.pull_request ?? '',
                    jobs: (removal.jobs ?? []).join(', '),
                  })}
                  , removed anyway
                </span>
              ))}
            </li>
          ))}
        </ul>
      )}

      {(moves.length > 0 || reparent) && (
        <div className="flex flex-col gap-2">
          <span className="text-sm font-medium">Children left behind</span>
          {moves.map((move) => (
            <p key={move.branch} className="text-sm text-muted-foreground">
              {move.branch} still points at {move.from}.
            </p>
          ))}
          <div className="flex items-center gap-3">
            <Switch id="reparent" checked={reparent} onCheckedChange={onReparent} />
            <Label htmlFor="reparent">{moveLabel(moves.map((move) => move.to))}</Label>
          </div>
        </div>
      )}

      {(error?.code === 'clean.unsafe' || force) && (
        <div className="flex items-center gap-2">
          <Checkbox id="force" checked={force} onCheckedChange={(on) => onForce(on === true)} />
          <Label htmlFor="force">Remove anyway: what they hold is lost</Label>
        </div>
      )}

      {warnings.map((warning) => (
        <p key={`${warning.code}-${warning.message}`} className="text-sm text-amber-600 dark:text-amber-500">
          {warning.message}
        </p>
      ))}
    </div>
  )
}

function moveLabel(targets: string[]) {
  const distinct = [...new Set(targets)]
  if (distinct.length === 1) return `Move ${targets.length === 1 ? 'it' : 'them'} onto ${distinct[0]}`
  return 'Move them onto their nearest worktree that stays'
}
