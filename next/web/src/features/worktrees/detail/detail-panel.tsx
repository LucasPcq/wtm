import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ExternalLink, X } from 'lucide-react'
import { Fragment, type ReactNode } from 'react'
import { githubQuery, readQuery } from '@/api/queries/commands'
import type { DomainFollowUp } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ActionsMenu } from '@/features/worktrees/detail/components/actions-menu'
import { ActivitySection } from '@/features/worktrees/detail/components/activity-section'
import { ChangesSection } from '@/features/worktrees/detail/components/changes-section'
import { LinksSection } from '@/features/worktrees/detail/components/links-section'
import { Origin } from '@/features/worktrees/detail/components/origin'
import { Problems } from '@/features/worktrees/detail/components/problems'
import { ReviewSection } from '@/features/worktrees/detail/components/review-section'
import { RunSection } from '@/features/worktrees/detail/components/run-section'
import { SectionsChooser } from '@/features/worktrees/detail/components/sections-chooser'
import { type SectionId, sectionIds } from '@/features/worktrees/detail/utils/sections'
import { overlaid } from '@/features/worktrees/shared/utils/condition'
import { unavailable } from '@/features/worktrees/shared/utils/github'
import { useChoice } from '@/features/worktrees/store/ui-store'

export function DetailPanel({
  repo,
  worktree,
  actions,
}: {
  repo: string
  worktree: string
  actions: DomainFollowUp[]
}) {
  const answers = [{ path: 'worktree', values: [worktree] }]
  const live = useQuery(readQuery(repo, 'status', answers))
  // asked again each time the panel opens, never on an event
  const github = useQuery({ ...githubQuery(repo, 'status', answers), refetchOnMount: 'always' })
  const sections = useChoice('sections', repo, sectionIds, sectionIds)

  const detail = live.data?.items[0]?.detail
  const asked = github.data?.items[0]?.detail
  const shown = detail && overlaid(detail, asked)
  const pr = asked?.review
  const now = live.dataUpdatedAt
  const commands = actions.filter((action) => !action.url)

  const section = (id: SectionId, of: NonNullable<typeof detail>): ReactNode => {
    switch (id) {
      case 'run':
        return <RunSection detail={of} now={now} />
      case 'review':
        return (
          <ReviewSection
            review={pr}
            pending={github.isPending}
            warning={unavailable(github.data?.warnings)?.message}
            askedAt={github.dataUpdatedAt}
          />
        )
      case 'changes':
        return <ChangesSection detail={of} />
      case 'activity':
        return <ActivitySection detail={of} now={now} />
      case 'links':
        return <LinksSection detail={of} />
    }
  }

  return (
    <aside aria-label={worktree} className="flex w-[440px] shrink-0 flex-col border-l bg-background">
      <header className="flex flex-col gap-1 px-6 pt-5 pb-4">
        <div className="flex items-center gap-1">
          <h2 className="min-w-0 flex-1 truncate text-lg font-semibold">{worktree}</h2>
          {pr && (
            <Button variant="ghost" size="sm" asChild>
              <a
                href={pr.url}
                target="_blank"
                rel="noreferrer"
                aria-label={`Open pull request #${pr.number} on GitHub`}
              >
                <ExternalLink />#{pr.number}
              </a>
            </Button>
          )}
          {commands.length > 0 && <ActionsMenu actions={commands} />}
          <Button variant="ghost" size="icon-sm" aria-label="Close the detail" asChild>
            <Link from="/r/$repo/worktrees" search={(search) => ({ ...search, worktree: undefined })}>
              <X />
            </Link>
          </Button>
        </div>
        {detail && <Origin detail={detail} now={now} />}
      </header>

      <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-6 pb-6">
        {live.isPending && <Skeleton className="h-32 w-full" />}
        {live.isError && <p className="text-sm text-destructive">{live.error.message}</p>}
        <Problems problems={shown?.problems ?? []} />
        {detail && sections.shown.map((id) => <Fragment key={id}>{section(id, detail)}</Fragment>)}
      </div>

      <footer className="border-t px-6 py-3">
        <SectionsChooser shown={sections.shown} onToggle={sections.toggle} />
      </footer>
    </aside>
  )
}
