import { CloudOff, ExternalLink, GitMerge, GitPullRequestClosed, type LucideIcon } from 'lucide-react'
import type { PruneCandidate, PruneReason } from '@/api/types.gen'

const reasonLook: Record<PruneReason, { icon: LucideIcon; said: (number: number | undefined) => string }> = {
  merged: { icon: GitMerge, said: (number) => `PR #${number} merged` },
  closed: { icon: GitPullRequestClosed, said: (number) => `PR #${number} closed` },
  gone: { icon: CloudOff, said: () => 'Deleted on origin' },
}

// a state, not a judgement: grey, whatever the reason
export function ReasonLine({ candidate: { reason, pr } }: { candidate: PruneCandidate }) {
  const { icon: Icon, said } = reasonLook[reason]

  return (
    <span className="flex items-center gap-1.5 text-muted-foreground">
      <Icon className="size-3.5 shrink-0" aria-hidden="true" />
      {said(pr?.number)}
      {pr && (
        <a href={pr.url} target="_blank" rel="noreferrer" aria-label={`Open pull request #${pr.number}`}>
          <ExternalLink className="size-3.5" />
        </a>
      )}
    </span>
  )
}
