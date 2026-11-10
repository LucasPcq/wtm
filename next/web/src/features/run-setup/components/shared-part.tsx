import { useState } from 'react'
import type { RuninitCandidate } from '@/api/types.gen'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { type PartProps, pickedOf, type SpaceRow, toggled } from '@/features/run-setup/utils/setup'
import { spellSpace } from '@/features/run-setup/utils/spell'

const noSpace = (service: string): SpaceRow => ({ service, name: '', create: '', remove: '', spelled: [] })

export function SharedPart({ setup, answer }: PartProps) {
  const { form, plan } = setup
  const jobs = pickedOf(form, 'jobs')
  const shared = pickedOf(form, 'shared')
  const touches = pickedOf(form, 'touches')
  const tasks = pickedOf(form, 'tasks').filter((task) => jobs.includes(task))
  const services = (plan.detection.candidates ?? []).filter((c) => c.compose && jobs.includes(c.name))
  const spaces = plan.parts.spaces

  // the spaces of every service as the plan spells them, but the one changed
  const setSpace = (row: SpaceRow) =>
    answer('spaces', [
      ...spaces.filter((one) => one.service !== row.service).flatMap((one) => one.spelled),
      ...spellSpace(row),
    ])

  return (
    <div className="flex flex-col gap-6">
      <div className="overflow-hidden rounded-md border">
        {services.map((service) => (
          <Service
            key={service.name}
            service={service}
            shared={shared.includes(service.name)}
            space={spaces.find((row) => row.service === service.name) ?? noSpace(service.name)}
            onShared={(on) => answer('shared', toggled(shared, service.name, on))}
            onSpace={setSpace}
          />
        ))}
      </div>

      {tasks.length > 0 && (
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1">
            <h3 className="font-medium">Tasks that change data</h3>
            <p className="text-sm text-muted-foreground">
              wtm refuses to run them against data the worktree doesn't own.
            </p>
          </div>
          <div className="flex flex-col gap-2">
            {tasks.map((task) => (
              <Label key={task} className="flex items-center gap-3 font-normal">
                <Checkbox
                  checked={touches.includes(task)}
                  onCheckedChange={(on) => answer('touches', toggled(touches, task, on === true))}
                />
                <span className="font-medium">{task}</span>
                <span className="text-muted-foreground">changes the data of the shared services</span>
              </Label>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function Service({
  service,
  shared,
  space,
  onShared,
  onSpace,
}: {
  service: RuninitCandidate
  shared: boolean
  space: SpaceRow
  onShared: (on: boolean) => void
  onSpace: (row: SpaceRow) => void
}) {
  // its own space chosen with nothing typed yet is the page's until a name makes it an answer
  const [own, setOwn] = useState(space.name !== '')
  const owned = own || space.name !== ''

  const choose = (value: string) => {
    setOwn(value === 'own')
    if (value === 'nothing') onSpace(noSpace(service.name))
  }

  return (
    <div className="flex flex-col gap-3 border-b px-4 py-3 last:border-b-0">
      <div className="flex items-center gap-3">
        <span className="font-medium">{service.name}</span>
        <span className="flex-1 font-mono text-xs text-muted-foreground">{service.image}</span>
        <Select value={shared ? 'shared' : 'per'} onValueChange={(value) => onShared(value === 'shared')}>
          <SelectTrigger className="w-36" aria-label={`${service.name} scope`}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="shared">Shared</SelectItem>
            <SelectItem value="per">Per worktree</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {!shared && <p className="text-sm text-muted-foreground">Its own copy, its container named per worktree.</p>}
      {shared && (
        <div className="flex flex-col gap-3">
          <span className="text-sm text-muted-foreground">Each worktree gets</span>
          <RadioGroup value={owned ? 'own' : 'nothing'} onValueChange={choose} className="grid grid-cols-2 gap-4">
            <Label className="flex items-start gap-2 font-normal">
              <RadioGroupItem value="nothing" className="mt-0.5" />
              <span className="flex flex-col">
                Nothing of its own <span className="text-xs text-muted-foreground">Worktrees share it as it is.</span>
              </span>
            </Label>
            <Label className="flex items-start gap-2 font-normal">
              <RadioGroupItem value="own" className="mt-0.5" />
              <span className="flex flex-col">
                Its own space <span className="text-xs text-muted-foreground">Made by a command.</span>
              </span>
            </Label>
          </RadioGroup>
          {owned && <SpaceFields key={space.spelled.join('\n')} space={space} onSpace={onSpace} />}
        </div>
      )}
    </div>
  )
}

// typed freely, sent once a field is left
function SpaceFields({ space, onSpace }: { space: SpaceRow; onSpace: (row: SpaceRow) => void }) {
  const [draft, setDraft] = useState(space)
  const fields = [
    { key: 'name', label: 'Name' },
    { key: 'create', label: 'Create on start' },
    { key: 'remove', label: 'Drop on clean' },
  ] as const

  return (
    <div className="grid grid-cols-[8rem_1fr] items-center gap-x-4 gap-y-2">
      {fields.map(({ key, label }) => (
        <Label key={key} className="contents font-normal">
          <span className="text-sm text-muted-foreground">{label}</span>
          <Input
            className="font-mono text-xs"
            value={draft[key]}
            onChange={(event) => setDraft({ ...draft, [key]: event.target.value })}
            onBlur={() => onSpace(draft)}
          />
        </Label>
      ))}
    </div>
  )
}
