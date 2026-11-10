import { SortableKeyboardPlugin } from '@dnd-kit/dom/sortable'
import { DragDropProvider, KeyboardSensor, PointerSensor } from '@dnd-kit/react'
import { isSortable, useSortable } from '@dnd-kit/react/sortable'
import { GripVertical, Plus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { changed, hookOf, moved } from '@/features/worktrees/set-up/utils/hooks'

// a Select item cannot be the empty string: the root, where a hook runs with no cwd, is this value
const root = '.'

export function HookList({
  id,
  label,
  values,
  dirs,
  onChange,
}: {
  id: string
  label: string
  values: string[]
  dirs: string[]
  onChange: (values: string[]) => void
}) {
  return (
    <DragDropProvider
      sensors={[PointerSensor, KeyboardSensor]}
      onDragEnd={({ canceled, operation }) => {
        const { source, target } = operation
        if (canceled || !isSortable(source) || !isSortable(target) || source.index === target.index) return
        onChange(moved(values, source.index, target.index))
      }}
    >
      <ul id={id} aria-label={label} className="divide-y rounded-lg border">
        {values.map((value, i) => (
          <HookRow
            // a row is its place in the list: the list is answered again, in its new order, once a row is dropped
            // biome-ignore lint/suspicious/noArrayIndexKey: two rows may hold the same command
            key={i}
            group={id}
            index={i}
            value={value}
            dirs={dirs}
            onChange={(change) => onChange(changed(values, i, change))}
            onRemove={() => onChange(values.filter((_, at) => at !== i))}
          />
        ))}
        <li className="px-3 py-2">
          <Button variant="ghost" size="sm" className="-ml-2" onClick={() => onChange([...values, ''])}>
            <Plus />
            Add a command
          </Button>
        </li>
      </ul>
    </DragDropProvider>
  )
}

function HookRow({
  group,
  index,
  value,
  dirs,
  onChange,
  onRemove,
}: {
  group: string
  index: number
  value: string
  dirs: string[]
  onChange: (change: { cmd?: string; cwd?: string; continue_on_error?: boolean }) => void
  onRemove: () => void
}) {
  // no optimistic sorting: it moves the rows in the DOM while React keys them by place, so the list would show its old
  // order over the new one; the list is answered again on the drop instead
  const { ref, handleRef } = useSortable({ id: `${group}-${index}`, index, group, plugins: [SortableKeyboardPlugin] })
  const hook = hookOf(value)
  const where = hook.cwd ?? root

  return (
    <li ref={ref} className="flex items-center gap-2 px-3 py-2">
      <button
        type="button"
        ref={handleRef}
        aria-label={`Move ${hook.cmd || 'this command'}`}
        className="cursor-grab text-muted-foreground"
      >
        <GripVertical className="size-4" />
      </button>
      <Input
        aria-label="Command"
        placeholder="pnpm install"
        className="min-w-0 flex-1 font-mono"
        value={hook.cmd}
        onChange={(e) => onChange({ cmd: e.target.value })}
      />
      <Select value={where} onValueChange={(cwd) => onChange({ cwd: cwd === root ? '' : cwd })}>
        <SelectTrigger aria-label="Where it runs" className="w-36 shrink-0">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={root}>root</SelectItem>
          {[...new Set([...dirs, ...(hook.cwd ? [hook.cwd] : [])])].map((dir) => (
            <SelectItem key={dir} value={dir}>
              {dir}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <label
        htmlFor={`${group}-${index}-go-on`}
        className="flex shrink-0 items-center gap-1.5 text-xs text-muted-foreground"
      >
        <Checkbox
          id={`${group}-${index}-go-on`}
          checked={hook.continue_on_error ?? false}
          onCheckedChange={(on) => onChange({ continue_on_error: on === true })}
        />
        Go on if it fails
      </label>
      <Button variant="ghost" size="icon" className="size-7" aria-label={`Remove ${hook.cmd}`} onClick={onRemove}>
        <X />
      </Button>
    </li>
  )
}
