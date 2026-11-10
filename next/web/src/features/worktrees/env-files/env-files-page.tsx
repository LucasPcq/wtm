import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  ChevronDown,
  CircleMinus,
  Copy,
  FileKey,
  GitBranch,
  Link as Linked,
  RefreshCw,
  TriangleAlert,
} from 'lucide-react'
import { useState } from 'react'
import { questionsQuery, readQuery } from '@/api/queries/commands'
import { Centered } from '@/components/centered'
import { PageHeader } from '@/components/page-header'
import { StatePage } from '@/components/state-page'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Glyph } from '@/features/worktrees/components/glyph'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { ReadFailed } from '@/features/worktrees/components/read-failed'
import { EnvDialog } from '@/features/worktrees/env/env-dialog'
import { KeysTable } from '@/features/worktrees/env-files/components/keys-table'
import { ToCheck } from '@/features/worktrees/env-files/components/to-check'
import type { EnvFilesSearch } from '@/features/worktrees/env-files/utils/search'
import { openEnv } from '@/features/worktrees/hooks/use-env-dialog'
import { RunStartDialog } from '@/features/worktrees/run-start/run-start-dialog'

export function EnvFilesPage({ repo, search }: { repo: string; search: EnvFilesSearch }) {
  const form = useQuery(questionsQuery(repo, 'env-show'))
  const field = form.data?.fields.find((one) => one.path === 'worktree')
  const options = field?.kind === 'select' ? field.options : []
  const worktree = search.worktree ?? (field?.kind === 'select' ? field.value : undefined)
  // values are shown for the worktree they were asked for, never kept in the address nor carried to another
  const [shownFor, setShownFor] = useState<string>()
  const shown = worktree !== undefined && shownFor === worktree
  const read = useQuery({
    ...readQuery(repo, 'env-show', [
      { path: 'worktree', values: [worktree ?? ''] },
      { path: 'show_values', values: [String(shown)] },
    ]),
    enabled: worktree !== undefined,
  })
  const detail = read.data?.items[0]?.detail
  const apps = detail?.apps ?? []
  const checks = detail?.checks ?? []

  const reconcile = () => openEnv(repo, { worktree: worktree ?? '' })

  const body = () => {
    if (form.isError) return <ReadFailed message={form.error.message} />
    if (read.isError) return <ReadFailed message={read.error.message} />
    if (read.data?.error) return <ReadFailed message={read.data.error.message} />
    if (!detail) return <Skeleton className="m-8 h-64" />
    if (apps.length === 0)
      return (
        <Centered>
          <StatePage icon={<FileKey />} title="No .env file">
            The project declares no .env file. Declare each one with its template, and every worktree gets its own copy,
            kept in step.
          </StatePage>
        </Centered>
      )

    return (
      <div className="flex flex-col gap-6 px-8 py-6">
        <PlanWarnings warnings={read.data?.warnings ?? []} />
        {checks.length > 0 && (
          <>
            <ToCheck checks={checks} onReconcile={reconcile} />
            <hr />
          </>
        )}
        <section aria-label="Keys by file" className="flex flex-col gap-2">
          <h2 className="text-xs font-medium text-muted-foreground">Keys by file</h2>
          <div className="rounded-md border">
            <KeysTable apps={apps} worktree={detail.worktree} main={detail.main} />
          </div>
        </section>
      </div>
    )
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader
        title=".env files"
        description="What each worktree's .env keys follow, and where each file comes from"
        action={
          <Button onClick={reconcile} disabled={!detail || apps.length === 0}>
            <RefreshCw aria-hidden="true" />
            Reconcile
          </Button>
        }
      />
      <div className="flex flex-wrap items-center gap-4 border-b px-8 py-3 text-sm">
        <span className="text-muted-foreground">Worktree</span>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" aria-label={`Worktree: ${worktree ?? ''}`}>
              <GitBranch aria-hidden="true" />
              {worktree ?? '…'}
              <ChevronDown className="text-muted-foreground" aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            {options.map((option) => (
              <DropdownMenuItem key={option.value} asChild>
                <Link
                  to="/r/$repo/env"
                  params={{ repo }}
                  search={{ ...search, worktree: option.value }}
                  aria-current={option.value === worktree ? 'page' : undefined}
                >
                  {option.value}
                </Link>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
        <span className="text-muted-foreground">
          {detail?.main
            ? 'Its keys and values. Reconcile starts with it.'
            : "Its keys and values, main's beside them. Reconcile starts with it."}
        </span>
        <div className="ml-auto flex items-center gap-4 text-muted-foreground">
          <Glyph icon={Linked} tone="muted">
            follows a job
          </Glyph>
          <Glyph icon={Copy} tone="muted">
            copied
          </Glyph>
          <Glyph icon={CircleMinus} tone="muted">
            in no source
          </Glyph>
          <Glyph icon={TriangleAlert} tone="attention">
            to check
          </Glyph>
          <Label className="flex items-center gap-2 font-normal">
            <Switch
              checked={shown}
              onCheckedChange={(on) => setShownFor(on ? worktree : undefined)}
              aria-label="Show values"
            />
            Show values
          </Label>
        </div>
      </div>
      <div className="flex min-h-0 flex-1 flex-col overflow-auto">{body()}</div>
      <EnvDialog repo={repo} />
      <RunStartDialog repo={repo} />
    </div>
  )
}
