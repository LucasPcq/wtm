import type { StatusDetail } from '@/api/types.gen'
import { Section } from '@/features/worktrees/detail/section'
import { ago } from '@/lib/time'

export function ActivitySection({ detail: { commits }, now }: { detail: StatusDetail; now: number }) {
  return (
    <Section id="activity" meta={commits?.length ? `last ${commits.length} commits` : ''}>
      {commits?.length ? (
        <ul className="flex flex-col gap-2">
          {commits.map(({ sha, subject, author, at }) => (
            <li key={sha} className="flex flex-col">
              <span className="truncate text-sm">{subject}</span>
              <span className="font-mono text-xs text-muted-foreground">
                {sha.slice(0, 7)} · {author} · {ago(at, now)}
              </span>
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-sm text-muted-foreground">No commit yet.</p>
      )}
    </Section>
  )
}
