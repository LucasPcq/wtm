import type { StatusReview } from '@/api/types.gen'
import { Skeleton } from '@/components/ui/skeleton'
import { Section } from '@/features/worktrees/detail/components/section'
import { Checks } from '@/features/worktrees/shared/components/checks'
import { prState } from '@/features/worktrees/shared/utils/github'

export function ReviewSection({
  review,
  pending,
  warning,
  askedAt,
}: {
  review: StatusReview | null | undefined
  pending: boolean
  warning: string | undefined
  askedAt: number
}) {
  const meta = askedAt
    ? `as of ${new Date(askedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`
    : ''

  const body = () => {
    if (pending) return <Skeleton className="h-10 w-full" />
    if (warning) return <p className="text-sm text-muted-foreground">{warning}</p>
    if (!review) return <p className="text-sm text-muted-foreground">No pull request.</p>

    const { url, number, title, state, draft, checks, decision } = review
    return (
      <div className="flex flex-col gap-2 text-sm">
        <a href={url} target="_blank" rel="noreferrer" className="font-medium underline-offset-4 hover:underline">
          #{number} {title ?? state}
        </a>
        <div className="flex flex-wrap gap-4">
          {draft && <span className="text-muted-foreground">draft</span>}
          {prState[state] && <span className="text-muted-foreground">{prState[state]}</span>}
          {checks && <Checks checks={checks} />}
          {decision && <span className="text-muted-foreground">{decision.toLowerCase().replaceAll('_', ' ')}</span>}
        </div>
      </div>
    )
  }

  return (
    <Section id="review" meta={meta}>
      {body()}
    </Section>
  )
}
