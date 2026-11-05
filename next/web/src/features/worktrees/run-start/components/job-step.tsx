import { useId, useState } from 'react'
import type { DomainOption } from '@/api/types.gen'
import { Item, ItemContent, ItemDescription, ItemTitle } from '@/components/ui/item'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { ItemList } from '@/features/worktrees/components/item-list'
import { SearchRow } from '@/features/worktrees/components/search-row'
import { searched } from '@/features/worktrees/utils/filters'
import { plural } from '@/lib/plural'

// options are the declared jobs not running in the worktree
export function JobStep({
  worktree,
  options,
  value,
  onChange,
}: {
  worktree: string
  options: DomainOption[]
  value: string
  onChange: (job: string) => void
}) {
  const id = useId()
  const [query, setQuery] = useState('')
  const visible = options.filter((option) => searched(option.value, query))

  return (
    <section aria-labelledby={id} className="flex flex-col gap-3">
      <div className="flex flex-col gap-1">
        <h3 id={id} className="text-sm font-medium">
          Job
        </h3>
        <p className="text-xs text-muted-foreground">
          In {worktree} ·{' '}
          {options.length === 0 ? 'every declared job runs already' : `${plural(options.length, 'job')} not running`}
        </p>
      </div>
      <RadioGroup aria-label="Job" value={value} onValueChange={onChange} className="gap-0">
        <ItemList
          label="Jobs"
          header={<SearchRow label="Search the jobs" placeholder="Search jobs" query={query} onQuery={setQuery} />}
        >
          {visible.map((option) => (
            <Item key={option.value} asChild className="items-start gap-3 rounded-none px-4 py-3">
              <label htmlFor={`${id}-${option.value}`}>
                <RadioGroupItem
                  id={`${id}-${option.value}`}
                  aria-label={option.value}
                  value={option.value}
                  className="mt-0.5"
                />
                <ItemContent className="gap-1">
                  <ItemTitle className="min-w-0 font-normal">{option.value}</ItemTitle>
                  {option.kind && <ItemDescription className="text-xs">{option.kind}</ItemDescription>}
                </ItemContent>
              </label>
            </Item>
          ))}
        </ItemList>
      </RadioGroup>
    </section>
  )
}
