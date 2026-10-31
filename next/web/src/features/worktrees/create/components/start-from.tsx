import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

export type StartFromValue = 'branch' | 'pull_request'

const label: Record<StartFromValue, string> = { branch: 'New branch', pull_request: 'Pull request' }

export function StartFrom({ value, onChange }: { value: StartFromValue; onChange: (value: StartFromValue) => void }) {
  return (
    <ToggleGroup
      type="single"
      aria-label="Start from"
      value={value}
      onValueChange={(next: string) => {
        if (next === 'branch' || next === 'pull_request') onChange(next)
      }}
      className="w-full rounded-lg bg-muted p-1"
    >
      {(['branch', 'pull_request'] as const).map((option) => (
        <ToggleGroupItem
          key={option}
          value={option}
          className="flex-1 text-muted-foreground data-[state=on]:bg-background data-[state=on]:text-foreground data-[state=on]:shadow-sm"
        >
          {label[option]}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )
}
