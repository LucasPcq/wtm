import { Columns3, Lock } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { columnGroups, columnText, defaultColumns, toggled, type ColumnId } from './columns'

// ColumnsPanel chooses the columns of the list, grouped by what they are about, each with what it shows; the choice
// is kept for this repository in this browser.
export function ColumnsPanel(props: { shown: ColumnId[]; onChange: (shown: ColumnId[]) => void }) {
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" className="data-[state=open]:bg-accent">
          <Columns3 />
          Columns
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[540px] p-0">
        <div className="flex items-center gap-2 border-b px-4 py-3">
          <span className="text-sm font-semibold">Columns</span>
          <span className="text-xs text-muted-foreground">Saved for this repository</span>
          <Button variant="ghost" size="sm" className="ml-auto" onClick={() => props.onChange(defaultColumns)}>
            Reset
          </Button>
        </div>
        <p className="flex items-center gap-1.5 px-4 pt-2.5 text-xs text-muted-foreground">
          <Lock className="size-3" aria-hidden="true" />
          Branch is always shown
        </p>
        <div className="grid grid-cols-2 gap-x-3 gap-y-2 px-2 pt-1 pb-3">
          {columnGroups.map((group) => (
            <fieldset key={group.title} className="flex flex-col">
              <legend className="px-2 py-1 text-xs font-medium text-muted-foreground">{group.title}</legend>
              {group.columns.map((column) => (
                <label
                  key={column}
                  className="flex cursor-pointer items-start gap-2.5 rounded-md px-2 py-1.5 hover:bg-accent"
                >
                  <Checkbox
                    className="mt-0.5"
                    checked={props.shown.includes(column)}
                    onCheckedChange={() => props.onChange(toggled(props.shown, column))}
                  />
                  <span className="flex flex-col">
                    <span className="text-sm font-medium">{columnText[column].label}</span>
                    <span className="text-xs text-muted-foreground">{columnText[column].hint}</span>
                  </span>
                </label>
              ))}
            </fieldset>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  )
}
