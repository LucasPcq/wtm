import { RowBox } from '@/features/worktrees/daemon/components/row-box'
import type { RepoRows } from '@/features/worktrees/daemon/utils/rows'

export function RepoRowsList({ repos }: { repos: RepoRows[] }) {
  return repos.map(({ repo, rows }) => (
    <section key={repo} className="flex flex-col gap-2" aria-label={repo}>
      <h3 className="text-sm text-muted-foreground">{repo}</h3>
      <RowBox rows={rows} />
    </section>
  ))
}
