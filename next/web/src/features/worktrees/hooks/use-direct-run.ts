import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { type Applied, dryRunQuery, type MutationCommand, narrowed, type Planned } from '@/api/queries/commands'
import type { DomainAnswered } from '@/api/types.gen'
import { useRun } from '@/features/worktrees/hooks/use-run'
import { todo, type Unit } from '@/features/worktrees/utils/progress'

export type Told = { tone: 'success' | 'error' | 'info'; title: string; description?: string | undefined }

// answers are the dry run's, which reads the facts afresh; applying, the apply's, which holds to what it read
export type DirectRun<C extends MutationCommand> = {
  subjects: string[]
  answers: DomainAnswered[]
  applying: DomainAnswered[]
  unitsOf: (plan: Planned<C>) => Unit[]
  doing: string
  told: (envelope: Applied<C> | undefined) => Told
  shown: () => boolean
  onDecide: () => void
  onDetails: (subjects: string[]) => void
}

// a quick, reversible action on one row runs at once: its dry run, then its apply under that plan's fingerprint, the
// row under way from the click (the run is taken in the store before its plan is known) and toasts for the person; a
// plan with anything to decide (a refusal, a warning, nothing to do) opens its dialog instead
export function useDirectRun<C extends MutationCommand>(repo: string, command: C) {
  const client = useQueryClient()
  const run = useRun(repo, command)

  return async ({ subjects, answers, applying, unitsOf, doing, told, shown, onDecide, onDetails }: DirectRun<C>) => {
    // one run of a command at a time: the one under way, still planning or not yet read, shows instead
    const held = run.claim(subjects.map((subject) => todo(subject, [])))
    if (held) return onDetails(held)
    const id = toast.loading(doing)
    // asked again, never from the cache: a plan read earlier would be on facts since moved
    const dry = await client
      .fetchQuery({ ...dryRunQuery(repo, command, answers), staleTime: 0 })
      .then((envelope) => narrowed<Applied<C>['items'][number]['detail'], Planned<C>>(envelope))
      .catch(() => undefined)
    const units = dry?.plan ? unitsOf(dry.plan) : []
    if (!dry?.fingerprint || dry.error || dry.warnings.length > 0 || units.length === 0) {
      run.dismiss()
      toast.dismiss(id)
      return onDecide()
    }

    run.start({ answers: applying, fingerprint: dry.fingerprint, units }, (envelope) => {
      // the dialog open on the run says its outcome already
      if (shown()) return void toast.dismiss(id)
      const { tone, title, description } = told(envelope)
      const details = tone === 'error' ? { action: { label: 'Details', onClick: () => onDetails(subjects) } } : {}
      toast[tone](title, { id, description, ...details })
    })
  }
}
