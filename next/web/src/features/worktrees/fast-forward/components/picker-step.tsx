import { TriangleAlert } from 'lucide-react'
import type { DomainOption } from '@/api/types.gen'
import { Checkbox } from '@/components/ui/checkbox'
import { Item, ItemTitle } from '@/components/ui/item'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { ItemList } from '@/features/worktrees/components/item-list'
import { ListHeader } from '@/features/worktrees/components/list-header'
import { optionWords } from '@/features/worktrees/fast-forward/utils/words'
import { searched } from '@/features/worktrees/utils/filters'

// the box checks what is safe only: one click must not take a worktree with uncommitted changes
export function PickerStep({
  options,
  isChecked,
  onCheck,
  query,
  onQuery,
}: {
  options: DomainOption[]
  isChecked: (option: DomainOption) => boolean
  onCheck: (options: DomainOption[], on: boolean) => void
  query: string
  onQuery: (query: string) => void
}) {
  const visible = options.filter((option) => searched(option.value, query))

  return (
    <ItemList
      label="Worktrees"
      header={
        <ListHeader
          total={options.length}
          visible={visible.map((option) => option.value)}
          bulk={visible.filter((option) => !option.changes).map((option) => option.value)}
          checked={options.filter(isChecked).map((option) => option.value)}
          query={query}
          onQuery={onQuery}
          onCheck={(names, on) =>
            onCheck(
              visible.filter((option) => names.includes(option.value)),
              on,
            )
          }
        />
      }
    >
      {visible.map((option) => (
        <OptionItem
          key={option.value}
          option={option}
          checked={isChecked(option)}
          onCheck={(on) => onCheck([option], on)}
        />
      ))}
    </ItemList>
  )
}

function OptionItem({
  option,
  checked,
  onCheck,
}: {
  option: DomainOption
  checked: boolean
  onCheck: (on: boolean) => void
}) {
  const { attention, muted } = optionWords(option)

  return (
    <Item role="listitem" aria-label={option.value} className="items-center gap-3 rounded-none px-4 py-3">
      <Checkbox
        aria-label={`Fast-forward ${option.value}`}
        checked={checked}
        onCheckedChange={(on) => onCheck(on === true)}
      />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <ItemTitle className="min-w-0">{option.value}</ItemTitle>
        <div className="flex flex-col items-start gap-1 text-xs">
          {attention.map((line) => (
            <span key={line} className="flex items-center gap-1.5">
              <TriangleAlert className="size-3.5 shrink-0 text-amber-500" aria-hidden="true" />
              {line}
            </span>
          ))}
          {muted.map((line) => (
            <Tooltip key={line}>
              <TooltipTrigger asChild>
                {/* biome-ignore lint/a11y/noNoninteractiveTabindex: where it stands shows to a keyboard as to a pointer */}
                <span tabIndex={0} className="text-muted-foreground">
                  {line}
                </span>
              </TooltipTrigger>
              <TooltipContent>As origin was last fetched</TooltipContent>
            </Tooltip>
          ))}
        </div>
      </div>
    </Item>
  )
}
