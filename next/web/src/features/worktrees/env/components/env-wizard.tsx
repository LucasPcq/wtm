import { useQuery } from '@tanstack/react-query'
import { FileCheck2, GitBranch, TriangleAlert } from 'lucide-react'
import { useState } from 'react'
import { dryRunQuery, questionsQuery } from '@/api/queries/commands'
import type { DomainOption, DomainRecapValue, DomainWarning } from '@/api/types.gen'
import { DialogBand } from '@/components/dialog-band'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Item, ItemTitle } from '@/components/ui/item'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert } from '@/features/worktrees/components/alert'
import { Details } from '@/features/worktrees/components/details'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { Facts } from '@/features/worktrees/components/facts'
import { ItemBox } from '@/features/worktrees/components/item-box'
import { ItemList } from '@/features/worktrees/components/item-list'
import { LeftRow } from '@/features/worktrees/components/left-row'
import { ListHeader } from '@/features/worktrees/components/list-header'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { ReasonButton } from '@/features/worktrees/components/reason-button'
import { Stepper } from '@/features/worktrees/components/stepper'
import { leftRowsOf } from '@/features/worktrees/env/utils/left'
import { lineOf, recapLines, unitsOf, writes } from '@/features/worktrees/env/utils/plan'
import { isolationWords, problemWords } from '@/features/worktrees/env/utils/words'
import type { Start } from '@/features/worktrees/hooks/use-run'
import { searched } from '@/features/worktrees/utils/filters'
import { planWide } from '@/features/worktrees/utils/outcome'
import { phaseLabel } from '@/features/worktrees/utils/phases'
import { sentence } from '@/features/worktrees/utils/sentence'
import { plural } from '@/lib/plural'

export function EnvWizard({
  repo,
  worktree,
  again,
  onStart,
  onCancel,
}: {
  repo: string
  worktree: string
  again?: string[] | undefined
  onStart: (start: Start) => void
  onCancel: () => void
}) {
  // the worktrees Next took: Confirm plans them whatever the questions read since
  const [taken, setTaken] = useState<string[] | undefined>(again)
  const confirming = taken !== undefined
  const [ticks, setTicks] = useState<Record<string, boolean>>({})
  const [query, setQuery] = useState('')

  const form = useQuery(questionsQuery(repo, 'env'))
  const field = form.data?.fields.find((one) => one.path === 'worktree')
  const options = field?.kind === 'multi_select' ? field.options : []
  // the core offers those out of step; the worktree on screen goes with them
  // a retry offers the worktrees it runs again, Back included; else the core's pick and the worktree on screen
  const prechecked = again ?? [
    ...(field?.kind === 'multi_select' ? (field.prechecked ?? field.picked) : []),
    ...(worktree ? [worktree] : []),
  ]
  const isChecked = (option: DomainOption) => ticks[option.value] ?? prechecked.includes(option.value)
  const named = options.filter(isChecked).map((option) => option.value)
  const visible = options.filter((option) => searched(option.value, query))
  const tick = (names: string[], on: boolean) =>
    setTicks((now) => ({ ...now, ...Object.fromEntries(names.map((name) => [name, on])) }))

  const answers = [{ path: 'worktree', values: taken ?? [] }]
  const review = useQuery({ ...dryRunQuery(repo, 'env', answers), enabled: confirming })
  const plan = review.data?.error ? undefined : review.data?.plan
  const fingerprint = review.data?.fingerprint
  const warnings = review.data?.warnings ?? []
  const going = (plan?.worktrees ?? []).filter(writes)
  const lines = plan ? recapLines(plan) : new Map<string, DomainRecapValue[]>()
  const changesOf = (name: string) => (lines.get(name) ?? []).filter((value) => value.note !== 'left')
  const changed = going.reduce((count, r) => count + changesOf(r.worktree).length, 0)

  const description = () => {
    if (!confirming || !plan) return "Rewrites the keys that follow a job in each worktree's .env files."
    return `${plural(changed, 'change')} in ${plural(going.length, 'worktree')}.`
  }

  const leftRows = plan ? leftRowsOf(plan, warnings).map((row) => <LeftRow key={row.key} row={row} />) : []

  const body = () => {
    if (!confirming) {
      if (form.isPending) return <Skeleton className="h-40 w-full" />
      if (form.isError) return <Alert tone="red" description={form.error.message} />
      return (
        <ItemList
          label="Worktrees"
          header={
            <ListHeader
              total={options.length}
              visible={visible.map((option) => option.value)}
              bulk={visible.map((option) => option.value)}
              checked={named}
              query={query}
              onQuery={setQuery}
              onCheck={tick}
            />
          }
        >
          {visible.map((option) => (
            <PickRow
              key={option.value}
              option={option}
              checked={isChecked(option)}
              onCheck={(on) => tick([option.value], on)}
            />
          ))}
        </ItemList>
      )
    }

    if (review.isPending) return <Skeleton className="h-32 w-full" />
    if (review.isError) return <Alert tone="red" description={review.error.message} />
    if (review.data.error) return <Alert tone="red" description={review.data.error.message} />
    if (going.length === 0)
      return (
        <>
          <Empty className="gap-4 p-6">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <FileCheck2 />
              </EmptyMedia>
              <EmptyTitle>Nothing to reconcile</EmptyTitle>
              <EmptyDescription>Every key of these worktrees is in step.</EmptyDescription>
            </EmptyHeader>
          </Empty>
          <ItemList left={leftRows}>{[]}</ItemList>
        </>
      )

    return (
      <>
        <PlanWarnings warnings={review.data.warnings.filter(planWide)} />
        <section aria-label="To rewrite" className="flex flex-col gap-2">
          <h3 className="text-xs font-medium text-muted-foreground">To rewrite</h3>
          {going.map((r) => (
            <WorktreePlan
              key={r.worktree}
              worktree={r.worktree}
              changes={changesOf(r.worktree)}
              restart={warnings.find(
                (warning) => warning.code === 'env.restart' && warning.params?.worktree === r.worktree,
              )}
            />
          ))}
        </section>
        <ItemList left={leftRows}>{[]}</ItemList>
        <section className="flex flex-col gap-2">
          <h3 className="text-xs font-medium text-muted-foreground">How it runs</h3>
          <p className="text-sm">Only these keys change; the rest of each file stays as it is.</p>
        </section>
        <Details label="Details" summary="the folder and steps of each worktree">
          {going.map((r) => (
            <Facts
              key={r.worktree}
              heading={r.worktree}
              facts={[
                { label: 'Folder', values: [r.path], mono: true },
                {
                  label: 'Steps',
                  values: (r.phases ?? []).map((phase) =>
                    phase === 'env' ? 'Write its .env files' : phaseLabel[phase],
                  ),
                },
              ]}
            />
          ))}
        </Details>
      </>
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Reconcile .env files</DialogTitle>
        <DialogDescription>{description()}</DialogDescription>
      </DialogHeader>

      <DialogBand>
        <Stepper
          steps={[
            { title: 'Worktrees', state: confirming ? 'done' : 'now' },
            { title: 'Confirm', state: confirming ? 'now' : 'todo' },
          ]}
        />
      </DialogBand>

      <DialogScroll>{body()}</DialogScroll>

      <DialogFooter className="items-center">
        {confirming && plan && going.length === 0 ? (
          <Button onClick={onCancel}>Done</Button>
        ) : (
          <>
            <Button variant="ghost" className="mr-auto" onClick={onCancel}>
              Cancel
            </Button>
            {confirming && (
              <Button variant="outline" onClick={() => setTaken(undefined)}>
                Back
              </Button>
            )}
            {!confirming && (
              <ReasonButton
                disabled={named.length === 0}
                why="Check a worktree to reconcile it"
                onClick={() => setTaken(named)}
              >
                Next
              </ReasonButton>
            )}
            {confirming && (
              <ReasonButton
                disabled={!plan || !fingerprint || review.isFetching}
                why={review.isFetching ? 'Planning' : undefined}
                onClick={() => plan && fingerprint && onStart({ answers, fingerprint, units: unitsOf(plan) })}
              >
                {going.length > 1 ? `Reconcile ${going.length} worktrees` : 'Reconcile'}
              </ReasonButton>
            )}
          </>
        )}
      </DialogFooter>
    </>
  )
}

function PickRow({
  option,
  checked,
  onCheck,
}: {
  option: DomainOption
  checked: boolean
  onCheck: (on: boolean) => void
}) {
  const state = () => {
    const changes = option.out_of_step ?? 0
    if (option.problem)
      return (
        <span className="text-muted-foreground">
          {problemWords[option.problem.code] ?? sentence(option.problem.message)}
        </span>
      )
    if (changes > 0)
      return (
        <span className="flex items-center gap-1.5">
          <TriangleAlert className="size-3.5 shrink-0 text-amber-500" aria-hidden="true" />
          {plural(changes, 'key')} out of step
        </span>
      )
    if (option.isolation) return <span className="text-muted-foreground">{isolationWords[option.isolation]}</span>
    return <span className="text-muted-foreground">in step</span>
  }

  return (
    <Item role="listitem" aria-label={option.value} className="items-center gap-3 rounded-none px-4 py-3">
      <Checkbox
        aria-label={`Reconcile ${option.value}`}
        checked={checked}
        onCheckedChange={(on) => onCheck(on === true)}
      />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <ItemTitle className="min-w-0">{option.value}</ItemTitle>
        <div className="text-xs">{state()}</div>
      </div>
    </Item>
  )
}

function WorktreePlan({
  worktree,
  changes,
  restart,
}: {
  worktree: string
  changes: DomainRecapValue[]
  restart: DomainWarning | undefined
}) {
  return (
    <ItemBox label={worktree}>
      <div className="flex flex-col gap-2 px-4 py-3 text-sm">
        <p className="flex items-center gap-2">
          <GitBranch className="size-4 text-muted-foreground" aria-hidden="true" />
          <span className="font-medium">{worktree}</span>
          <span className="text-xs text-muted-foreground">{plural(changes.length, 'change')}</span>
        </p>
        <ul className="flex flex-col gap-1 pl-6">
          {changes.map((value) => {
            const { file, change } = lineOf(value)
            return (
              <li key={value.text} className="flex min-w-0 gap-2">
                {file && <span className="shrink-0 text-muted-foreground">{file}</span>}
                <span className="truncate font-mono" title={change}>
                  {change}
                </span>
              </li>
            )
          })}
        </ul>
        {restart && (
          <p className="flex items-center gap-1.5 pl-6 text-xs">
            <TriangleAlert className="size-3.5 shrink-0 text-amber-500" aria-hidden="true" />
            {restart.message}
          </p>
        )}
      </div>
    </ItemBox>
  )
}
