import { useStore } from '@tanstack/react-form'
import { useDebouncedValue } from '@tanstack/react-pacer'
import { useQuery } from '@tanstack/react-query'
import { CircleX, GitPullRequest, Search } from 'lucide-react'
import { useState } from 'react'
import { pullRequestQuery } from '@/api/queries/commands'
import type { DomainFieldError, DomainOption } from '@/api/types.gen'
import { keyOf } from '@/components/command-form/answers'
import type { CommandForm } from '@/components/command-form/use-command-form'
import { Button } from '@/components/ui/button'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { InputGroup, InputGroupAddon, InputGroupInput } from '@/components/ui/input-group'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { PrRow } from '@/features/worktrees/create/components/pr-row'
import {
  checkoutWords,
  type PrFilter,
  prFilterLabel,
  prFilters,
  refusalLine,
} from '@/features/worktrees/create/utils/checkout-words'

const pr = keyOf('pr')
const mine = keyOf('mine')
const review = keyOf('review')

export function PrPicker({
  repo,
  form,
  options,
  chosen,
  errors,
  onAsk,
  onNewBranch,
}: {
  repo: string
  form: CommandForm
  options: DomainOption[]
  chosen: string | undefined
  errors: DomainFieldError[]
  onAsk: () => void
  onNewBranch: () => void
}) {
  const [query, setQuery] = useState('')
  const filter = useStore(form.store, (state): PrFilter => {
    if (state.values[mine]?.[0] === 'true') return 'mine'
    if (state.values[review]?.[0] === 'true') return 'review'
    return 'all'
  })
  const { label, help } = checkoutWords(undefined).pr

  const typed = query.trim()
  const byNumber = /^\d+$/.test(typed)
  const listed = options.find((option) => option.value === typed)
  // a number no listed pull request is, once typing pauses, is looked up on GitHub, which finds a closed or merged one
  // too; looked up, it is shown, never chosen nor fetched until the person picks it
  const [paused] = useDebouncedValue(byNumber && !listed ? typed : '', { wait: 300 })
  const lookup = useQuery({ ...pullRequestQuery(repo, paused), enabled: paused !== '' && paused === typed })
  const lookedUp = lookup.data?.fields.find((field) => field.path === 'pr')
  const found = lookedUp?.kind === 'select' ? lookedUp.options.find((option) => option.value === paused) : undefined
  const unknown = lookup.data?.problems.find((problem) => problem.path === 'pr' && problem.code === 'not_found')

  const matches = (option: DomainOption) => {
    const of = option.pull_request
    if (!of || !typed) return true
    if (byNumber) return String(of.number).startsWith(typed)
    return (of.title ?? '').toLowerCase().includes(typed.toLowerCase())
  }
  const shown = [...options.filter(matches), ...(found && !listed && paused === typed ? [found] : [])]
  const base = options[0]?.pull_request?.url.replace(/^https?:\/\//, '').replace(/\/pull\/\d+$/, '')

  const choose = (value: string) => {
    form.setFieldValue(pr, [value])
    onAsk()
  }

  const enter = () => {
    const target = listed ?? (paused === typed ? found : undefined)
    if (target && !target.problem) choose(target.value)
  }

  const pick = (to: PrFilter) => {
    form.setFieldValue(mine, [String(to === 'mine')])
    form.setFieldValue(review, [String(to === 'review')])
    onAsk()
  }

  const empty = options.length === 0 && !typed && filter === 'all'

  const searchLine = () => {
    if (!byNumber || paused !== typed) return undefined
    if (unknown) return refusalLine(unknown)
    if (lookup.isError) return lookup.error.message
    return undefined
  }

  return (
    <section className="flex flex-col gap-2" aria-label={label}>
      <div className="flex flex-col gap-1">
        <h3 className="text-sm font-medium">{label}</h3>
        <p className="text-xs text-pretty text-muted-foreground">
          {base ? `Open pull requests on ${base}; ${help}` : `Open pull requests; ${help}`}
        </p>
      </div>

      <div className="flex flex-col rounded-lg border">
        <InputGroup className="rounded-b-none border-0 border-b shadow-none">
          <InputGroupAddon>
            <Search />
          </InputGroupAddon>
          <InputGroupInput
            aria-label="Search pull requests"
            placeholder="Search by title or number"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key !== 'Enter') return
              e.preventDefault()
              enter()
            }}
          />
        </InputGroup>

        {!empty && !typed && (
          <ToggleGroup
            type="single"
            variant="outline"
            size="sm"
            aria-label="Pull requests"
            value={filter}
            onValueChange={(value: string) => {
              const next = prFilters.find((known) => known === value)
              if (next) pick(next)
            }}
            className="px-3 pt-3"
          >
            {prFilters.map((known) => (
              <ToggleGroupItem key={known} value={known} className="rounded-full px-3">
                {prFilterLabel[known]}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        )}

        {empty && (
          <Empty className="p-6">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <GitPullRequest />
              </EmptyMedia>
              <EmptyTitle>No open pull request</EmptyTitle>
              <EmptyDescription>
                {base ?? 'The repository'} has none open. A number finds a closed or merged one.
              </EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <Button variant="outline" size="sm" onClick={onNewBranch}>
                New branch
              </Button>
            </EmptyContent>
          </Empty>
        )}

        {searchLine() && <p className="px-4 py-3 text-sm text-muted-foreground">{searchLine()}</p>}

        {!empty && shown.length === 0 && !searchLine() && (
          <p className="px-4 py-3 text-sm text-muted-foreground">No pull request matches.</p>
        )}

        {shown.length > 0 && (
          <ul className="flex flex-col p-1.5">
            {shown.map((option) =>
              option.pull_request ? (
                <PrRow
                  key={option.value}
                  option={option}
                  pr={option.pull_request}
                  chosen={option.value === chosen}
                  onChoose={() => choose(option.value)}
                />
              ) : null,
            )}
          </ul>
        )}
      </div>

      {errors.map((error) => (
        <p key={error.code} role="alert" className="flex items-start gap-2 text-sm text-destructive">
          <CircleX className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
          {refusalLine(error)}
        </p>
      ))}
    </section>
  )
}
