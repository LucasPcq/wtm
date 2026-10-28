import { ArrowDownToLine, CircleX, FastForward, GitMerge, type LucideIcon, TriangleAlert } from 'lucide-react'
import type { MutationCommand } from '@/api/queries/commands'
import type { DomainFollowUp, StatusProblem, StatusProblemCode } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { openPrune } from '@/features/worktrees/hooks/use-prune-dialog'
import { openRemove } from '@/features/worktrees/hooks/use-remove-dialog'
import { isMutation } from '@/features/worktrees/hooks/use-run'
import { type RunEntry, useRunEntry } from '@/features/worktrees/hooks/use-run-entry'
import { menu, pruneAction, subjectsOf } from '@/features/worktrees/utils/actions'
import { sentence } from '@/features/worktrees/utils/sentence'
import { cn } from '@/lib/utils'

type Tone = 'problem' | 'failure' | 'suggestion'

const calloutOf: Record<StatusProblemCode, { tone: Tone; icon: LucideIcon; consequence: string | null }> = {
  'status.missing': {
    tone: 'problem',
    icon: TriangleAlert,
    consequence: 'Its branch and its record go with it; the remote branch stays.',
  },
  'status.merged': { tone: 'suggestion', icon: GitMerge, consequence: null },
  'status.parent_gone': {
    tone: 'problem',
    icon: TriangleAlert,
    consequence: 'It rebases onto its new parent on its next sync.',
  },
  'status.cycle': {
    tone: 'problem',
    icon: TriangleAlert,
    consequence: 'It rebases onto its new parent on its next sync.',
  },
  'status.behind_origin': { tone: 'suggestion', icon: FastForward, consequence: null },
  'status.behind': { tone: 'suggestion', icon: ArrowDownToLine, consequence: null },
  'status.env_missing': { tone: 'problem', icon: TriangleAlert, consequence: null },
  'status.isolation_pending': { tone: 'problem', icon: TriangleAlert, consequence: null },
  'status.job_crashed': { tone: 'failure', icon: CircleX, consequence: null },
}

const look: Record<Tone, { box: string; glyph: string }> = {
  problem: { box: 'border-amber-500/35 bg-amber-500/8', glyph: 'text-amber-500' },
  failure: { box: 'border-destructive/35 bg-destructive/8', glyph: 'text-destructive' },
  suggestion: { box: 'border-border bg-muted/40', glyph: 'text-muted-foreground' },
}

// the fix the web runs, as the menus run it: Remove and Prune open their dialog, Fast-forward and Reparent run at once
const runsHere: Record<
  MutationCommand,
  { verb: string; open: (repo: string, fix: DomainFollowUp, runEntry: RunEntry) => void } | null
> = {
  clean: {
    verb: menu.clean.label,
    open: (repo, fix) => openRemove(repo, { worktrees: subjectsOf(fix), left: [], prechecked: true }),
  },
  'fast-forward': {
    verb: menu.fast_forward.label,
    open: (_repo, fix, runEntry) => runEntry({ action: 'fast_forward', followUp: fix }, []),
  },
  reparent: {
    verb: menu.reparent.label,
    open: (_repo, fix, runEntry) => runEntry({ action: 'reparent', followUp: fix }, []),
  },
  create: null,
  prune: { verb: pruneAction.label, open: (repo, fix) => openPrune(repo, subjectsOf(fix)) },
  exec: null,
}

export function Callout({ repo, problems: [first, ...more] }: { repo: string; problems: StatusProblem[] }) {
  const runEntry = useRunEntry(repo)
  if (!first) return null

  const { tone, icon: Icon, consequence } = calloutOf[first.code]
  const { box, glyph } = look[tone]
  const fix = first.fix ?? undefined
  const command = fix?.command
  const here = command && isMutation(command) ? runsHere[command] : null
  const runs = here !== null
  const act = () => fix && here?.open(repo, fix, runEntry)

  return (
    <section aria-label="To do" className={cn('flex flex-col gap-3 rounded-lg border p-4', box)}>
      <div className="flex items-center gap-2">
        <Icon className={cn('size-4 shrink-0', glyph)} aria-hidden="true" />
        <h3 className="min-w-0 flex-1 text-sm font-medium">{sentence(first.message)}</h3>
        {runs && tone === 'suggestion' && (
          <Button variant="outline" size="sm" onClick={act}>
            {here.verb}
          </Button>
        )}
      </div>
      {fix && !runs && (
        <>
          <p className="text-sm text-muted-foreground">{sentence(fix.message)} from a terminal.</p>
          <CommandLine followUp={fix} />
        </>
      )}
      {fix && runs && tone !== 'suggestion' && (
        <>
          <Button className="w-full" onClick={act}>
            {sentence(fix.message)}
          </Button>
          {consequence && <p className="text-xs text-muted-foreground">{consequence}</p>}
        </>
      )}
      {more.length > 0 && (
        <details className="text-sm">
          <summary className="cursor-pointer text-muted-foreground">
            {more.length} more {more.length === 1 ? 'problem' : 'problems'}
          </summary>
          <ul className="mt-3 flex flex-col gap-3">
            {more.map((problem) => (
              <li key={problem.code + problem.message} className="flex flex-col gap-2">
                {sentence(problem.message)}
                {problem.fix && <CommandLine followUp={problem.fix} />}
              </li>
            ))}
          </ul>
        </details>
      )}
    </section>
  )
}
