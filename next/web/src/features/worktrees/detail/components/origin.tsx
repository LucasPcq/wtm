import type { StatusDetail } from '@/api/types.gen'
import { CopyPath } from '@/features/worktrees/components/copy-path'
import { ago } from '@/lib/time'

export function Origin({ detail, now }: { detail: StatusDetail; now: number }) {
  const lineage = () => {
    if (detail.main) return 'main worktree'
    return detail.parent ? `from ${detail.parent}` : 'a root'
  }
  const facts = [lineage(), detail.created_at && `created ${ago(detail.created_at, now)}`, detail.isolation]

  return (
    <>
      <p className="text-sm text-muted-foreground">{facts.filter(Boolean).join(' · ')}</p>
      <CopyPath path={detail.path} />
    </>
  )
}
