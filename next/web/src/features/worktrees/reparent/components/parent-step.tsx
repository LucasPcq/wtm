import { ChevronsUpDown, GitBranch } from 'lucide-react'
import { useId, useState } from 'react'
import type { DomainSelectField } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { refusedWhy } from '@/features/worktrees/utils/reparent'

// what the worktrees can move onto first, the base branch on top; then what the core refuses, disabled with why
export function ParentStep({
  field,
  description,
  value,
  onChange,
}: {
  field: DomainSelectField
  description: string | undefined
  value: string
  onChange: (to: string) => void
}) {
  const [open, setOpen] = useState(false)
  const id = useId()
  const options = [
    ...field.options.filter((option) => option.value === field.pinned),
    ...field.options.filter((option) => option.value !== field.pinned),
  ]
  const refused = (field.refused ?? []).flatMap((error) => {
    const branch = error.params?.value
    return branch ? [{ branch, error }] : []
  })

  return (
    <section className="flex flex-col gap-2">
      <div className="flex flex-col gap-1">
        <label htmlFor={id} className="text-sm font-medium">
          New parent
        </label>
        {description && <p className="text-xs text-muted-foreground">{description}</p>}
      </div>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            id={id}
            variant="outline"
            role="combobox"
            aria-expanded={open}
            className="w-full justify-between font-normal"
          >
            {value || <span className="text-muted-foreground">Choose a branch</span>}
            <ChevronsUpDown className="text-muted-foreground" />
          </Button>
        </PopoverTrigger>
        <PopoverContent align="start" className="w-(--radix-popover-trigger-width) p-0">
          <Command>
            <CommandInput placeholder="Search branches" />
            <CommandList>
              <CommandEmpty>No branch matches.</CommandEmpty>
              <CommandGroup>
                {options.map((option) => (
                  <CommandItem
                    key={option.value}
                    value={option.value}
                    data-checked={option.value === value}
                    onSelect={() => {
                      onChange(option.value)
                      setOpen(false)
                    }}
                  >
                    <GitBranch className="text-muted-foreground" />
                    <span className="min-w-0 flex-1 truncate">{option.value}</span>
                    {option.value.startsWith('origin/') && (
                      <span className="text-xs text-muted-foreground">remote</span>
                    )}
                  </CommandItem>
                ))}
                {refused.map(({ branch, error }) => (
                  <CommandItem key={branch} value={branch} disabled>
                    <GitBranch className="text-muted-foreground" />
                    <span className="flex min-w-0 flex-col">
                      <span className="truncate">{branch}</span>
                      <span className="truncate text-xs text-muted-foreground">{refusedWhy(error)}</span>
                    </span>
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
    </section>
  )
}
