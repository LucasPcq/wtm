import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { optionsOf, type PartProps, pickedOf, selectedOf, toggled } from '@/features/run-setup/utils/setup'

export function AddressesPart({ setup, answer }: PartProps) {
  const { form, plan } = setup
  const named = pickedOf(form, 'urls')
  const offered = optionsOf(form, 'urls')
  const unoffered = [
    ...new Set(plan.parts.ports.filter((row) => row.variable && !offered.includes(row.job)).map((row) => row.job)),
  ]

  return (
    <div className="flex flex-col gap-6">
      <RadioGroup
        value={selectedOf(form, 'addressing')}
        onValueChange={(value) => answer('addressing', [value])}
        className="grid grid-cols-2 gap-4"
      >
        <Label className="flex items-start gap-2 font-normal">
          <RadioGroupItem value="names" className="mt-0.5" />
          <span className="flex flex-col">
            Named URLs <span className="text-xs text-muted-foreground">Each job under its name, in each worktree.</span>
          </span>
        </Label>
        <Label className="flex items-start gap-2 font-normal">
          <RadioGroupItem value="ports" className="mt-0.5" />
          <span className="flex flex-col">
            Ports <span className="text-xs text-muted-foreground">localhost and the worktree's port.</span>
          </span>
        </Label>
      </RadioGroup>

      {selectedOf(form, 'addressing') === 'names' && (
        <div className="flex flex-col gap-2">
          <h3 className="text-sm text-muted-foreground">Which jobs answer under their name</h3>
          {offered.map((job) => (
            <Label key={job} className="flex items-center gap-3 font-normal">
              <Checkbox
                checked={named.includes(job)}
                onCheckedChange={(on) => answer('urls', toggled(named, job, on === true))}
              />
              <span className="font-medium">{job}</span>
            </Label>
          ))}
          {unoffered.map((job) => (
            <span key={job} className="flex items-center gap-3 text-muted-foreground">
              <Checkbox disabled aria-label={job} />
              {job} <span className="text-xs">not HTTP</span>
            </span>
          ))}
        </div>
      )}
    </div>
  )
}
