import { DragDropProvider } from '@dnd-kit/react'
import { isSortable, useSortable } from '@dnd-kit/react/sortable'
import { GripVertical, Plus, X } from 'lucide-react'
import { useState } from 'react'
import type { ProjectHook } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

type Row = { id: number; hook: ProjectHook }

const root = '.'

export function HooksEditor({
  entries,
  dirs,
  disabled,
  onChange,
}: {
  entries: ProjectHook[]
  dirs: string[]
  disabled: boolean
  onChange: (hooks: ProjectHook[]) => void
}) {
  const [rows, setRows] = useState<Row[]>(() => entries.map((hook, id) => ({ id, hook })))
  const [added, setAdded] = useState<number>()

  const change = (next: Row[]) => {
    setRows(next)
    onChange(next.map(({ hook }) => hook))
  }
  const edit = (id: number, hook: ProjectHook) => change(rows.map((row) => (row.id === id ? { id, hook } : row)))
  const add = () => {
    const id = Math.max(-1, ...rows.map((row) => row.id)) + 1
    setAdded(id)
    change([...rows, { id, hook: { cmd: '' } }])
  }

  return (
    <DragDropProvider
      onDragEnd={({ canceled, operation }) => {
        const { source } = operation
        if (canceled || !isSortable(source) || source.initialIndex === source.index) return
        const next = [...rows]
        const [moved] = next.splice(source.initialIndex, 1)
        if (moved) change(next.toSpliced(source.index, 0, moved))
      }}
    >
      <div className="flex flex-col divide-y rounded-lg border">
        {rows.map((row, index) => (
          <HookRow
            key={row.id}
            row={row}
            index={index}
            dirs={dirs}
            disabled={disabled}
            focused={row.id === added}
            onEdit={(hook) => edit(row.id, hook)}
            onRemove={() => change(rows.filter(({ id }) => id !== row.id))}
          />
        ))}
        <button
          type="button"
          disabled={disabled}
          onClick={add}
          className="flex items-center gap-3 px-4 py-3 text-sm hover:bg-muted/40 disabled:opacity-60"
        >
          <Plus className="size-4" aria-hidden="true" />
          Add a command
        </button>
      </div>
    </DragDropProvider>
  )
}

function HookRow({
  row: { id, hook },
  index,
  dirs,
  disabled,
  focused,
  onEdit,
  onRemove,
}: {
  row: Row
  index: number
  dirs: string[]
  disabled: boolean
  focused: boolean
  onEdit: (hook: ProjectHook) => void
  onRemove: () => void
}) {
  const { ref, handleRef } = useSortable({ id, index, disabled })
  const where = hook.cwd ?? root
  const goOnId = `hook-${id}-go-on`

  return (
    <div ref={ref} className="flex items-center gap-3 bg-background px-3 py-2">
      <button
        type="button"
        ref={handleRef}
        className="cursor-grab text-muted-foreground"
        aria-label={`Move ${hook.cmd || 'the command'}`}
      >
        <GripVertical className="size-4" />
      </button>
      <Input
        aria-label="Command"
        className="min-w-0 flex-1 font-mono"
        value={hook.cmd}
        disabled={disabled}
        ref={(input) => {
          if (focused) input?.focus()
        }}
        onChange={(e) => onEdit({ ...hook, cmd: e.target.value })}
      />
      <Select
        value={where}
        disabled={disabled}
        onValueChange={(cwd) => {
          const { cwd: _, ...rest } = hook
          onEdit(cwd === root ? rest : { ...rest, cwd })
        }}
      >
        <SelectTrigger aria-label="Where it runs" className="w-40 shrink-0">
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
      <label htmlFor={goOnId} className="flex shrink-0 items-center gap-2 text-sm text-muted-foreground">
        <Checkbox
          id={goOnId}
          disabled={disabled}
          checked={hook.continue_on_error ?? false}
          onCheckedChange={(on) => onEdit({ ...hook, continue_on_error: on === true })}
        />
        Go on if it fails
      </label>
      <Button
        variant="ghost"
        size="icon-sm"
        disabled={disabled}
        aria-label={`Remove ${hook.cmd || 'the command'}`}
        onClick={onRemove}
      >
        <X />
      </Button>
    </div>
  )
}
