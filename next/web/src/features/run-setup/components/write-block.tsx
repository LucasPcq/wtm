import { Link } from '@tanstack/react-router'
import { ChevronRight, CircleCheck, FileCog, FilePlus, TriangleAlert } from 'lucide-react'
import type { Applied } from '@/api/queries/commands'
import type { RuninitFile, RuninitPlan } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { writtenWhere } from '@/features/run-setup/utils/words'
import { plural } from '@/lib/plural'

const fileChanged = 'run-init.file_changed'

// what a file of the plan changes, as the Write block says it
function changeOf(file: RuninitFile, plan: RuninitPlan) {
  if (file.path === 'run.toml') return `${file.new ? 'created' : 'rewritten'}, ${writtenWhere['run.toml']}`
  if (file.path === 'config.toml')
    return `${plural(plan.config?.added?.length ?? 0, '.env file')} added to the copied ones`
  const what = [...new Set((plan.compose?.edits ?? []).map((edit) => (edit.what === 'port' ? 'host ports' : 'names')))]
  return `${plural(plan.compose?.edits?.length ?? 0, 'line')}: ${what.join(', ')} · tracked, to commit`
}

const pathOf = (file: RuninitFile) => (file.tracked ? file.path : `.git/wtm/${file.path}`)

export function WriteBlock({
  plan,
  blocked,
  writing,
  written,
  failure,
  onWrite,
  onReadAgain,
}: {
  plan: RuninitPlan
  blocked: string | undefined
  writing: boolean
  written: Applied<'run-init'> | undefined
  failure: Error | null
  onWrite: () => void
  onReadAgain: () => void
}) {
  const files = plan.files ?? []
  const edits = plan.compose?.edits ?? []
  const changed = written?.items.find((item) => item.error?.code === fileChanged)

  if (written?.ok) return <Outcome plan={plan} written={written} />

  return (
    <section aria-label="Write" className="flex flex-col gap-5 rounded-lg border p-6">
      <h2 className="font-semibold">Write the configuration</h2>

      {changed && (
        <div className="flex flex-col gap-3 rounded-md border border-amber-500/35 bg-amber-500/8 p-4 text-sm">
          <span className="flex items-center gap-2 font-medium">
            <TriangleAlert className="size-4 text-amber-500" aria-hidden="true" />
            Couldn't write: {changed.subject} changed on disk since it was read
          </span>
          <span className="text-muted-foreground">Nothing was written. Your answers are kept.</span>
          <Button variant="outline" size="sm" className="self-start" onClick={onReadAgain}>
            Read {changed.subject} again
          </Button>
        </div>
      )}
      {!changed && (failure || written?.error) && (
        <p className="text-sm text-destructive">{failure?.message ?? written?.error?.message}</p>
      )}

      {files.length === 0 ? (
        <p className="text-sm text-muted-foreground">Nothing changed yet: run.toml holds these answers already.</p>
      ) : (
        <ul className="flex flex-col overflow-hidden rounded-md border text-sm">
          {files.map((file) => (
            <li key={file.path} className="flex items-center gap-3 border-b px-4 py-3 last:border-b-0">
              {file.new ? (
                <FilePlus className="size-4" aria-hidden="true" />
              ) : (
                <FileCog className="size-4" aria-hidden="true" />
              )}
              <span className="font-mono">{pathOf(file)}</span>
              <span className="text-muted-foreground">{changeOf(file, plan)}</span>
            </li>
          ))}
        </ul>
      )}

      {edits.length > 0 && (
        <Collapsible>
          <CollapsibleTrigger className="group flex items-center gap-2 text-sm">
            <ChevronRight
              className="size-4 transition-transform group-data-[state=open]:rotate-90"
              aria-hidden="true"
            />
            Details <span className="text-muted-foreground">· {plan.compose?.file}, line by line</span>
          </CollapsibleTrigger>
          <CollapsibleContent className="mt-3 grid grid-cols-[3rem_1fr] gap-x-3 gap-y-1 rounded-md bg-muted/50 p-3 font-mono text-xs">
            {edits.map((edit) => (
              <div key={`${edit.at.line}:${edit.at.column}`} className="contents">
                <span className="text-muted-foreground">{edit.at.line}</span>
                <span>
                  <span className="text-muted-foreground line-through">{edit.before}</span> {edit.after}
                </span>
              </div>
            ))}
          </CollapsibleContent>
        </Collapsible>
      )}

      <div className="flex justify-end">
        <Button disabled={blocked !== undefined || writing || files.length === 0} title={blocked} onClick={onWrite}>
          {writing ? 'Writing…' : `Write ${plural(files.length, 'file')}`}
        </Button>
      </div>
    </section>
  )
}

function Outcome({ plan, written }: { plan: RuninitPlan; written: Applied<'run-init'> }) {
  const files = written.items.map((item) => item.subject)
  const up = plan.preview.up

  return (
    <section aria-label="Written" className="flex flex-col gap-5 rounded-lg border p-6">
      <div className="flex gap-3">
        <CircleCheck className="mt-0.5 size-5 text-emerald-500" aria-hidden="true" />
        <div className="flex flex-col gap-1">
          <h2 className="font-semibold">Run is set up</h2>
          <p className="text-sm text-muted-foreground">
            {up.name ? `wtm run up starts the profile ${up.name} in any worktree.` : 'wtm run up starts every job.'}
          </p>
        </div>
      </div>
      <p className="flex items-center gap-3 text-sm">
        <CircleCheck className="size-4 text-emerald-500" aria-hidden="true" />
        <span className="font-medium">{plural(files.length, 'file')}</span>
        <span className="text-muted-foreground">{files.join(', ')}</span>
      </p>
      <div className="flex justify-end">
        <Button asChild>
          <Link to="/r/$repo/running" from="/r/$repo/run-setup">
            Open Running
          </Link>
        </Button>
      </div>
    </section>
  )
}
