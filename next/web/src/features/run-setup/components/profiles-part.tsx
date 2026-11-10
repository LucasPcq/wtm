import { useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { type PartProps, type ProfileRow, pickedOf, reInit, toggled } from '@/features/run-setup/utils/setup'
import { spellProfile } from '@/features/run-setup/utils/spell'

export function ProfilesPart({ setup, answer }: PartProps) {
  const { form, plan } = setup
  const rows = plan.parts.profiles
  const jobs = pickedOf(form, 'jobs')
  const [adding, setAdding] = useState(false)

  // the profiles run.toml will hold as the plan spells them, one changed, removed or added; one the plan only offers
  // is not among them until added
  const setProfiles = (next: ProfileRow[]) =>
    answer(
      'profiles',
      next.filter((row) => row.written).map((row) => row.spelled),
    )
  const replace = (row: ProfileRow, jobs: string[]) =>
    setProfiles(
      rows.map((one) => (one === row ? { ...row, jobs, spelled: spellProfile({ name: row.name, jobs }) } : one)),
    )

  const groups = reInit(plan)
    ? [
        { title: 'Proposed for the new jobs', rows: rows.filter((row) => !row.written) },
        { title: 'In run.toml', rows: rows.filter((row) => row.written) },
      ]
    : [{ title: 'Proposed by run init: all starts every job, the others one app with what it needs', rows }]

  return (
    <div className="flex flex-col gap-6">
      {groups
        .filter((group) => group.rows.length > 0)
        .map((group) => (
          <div key={group.title} className="flex flex-col gap-3">
            <h3 className="text-sm text-muted-foreground">{group.title}</h3>
            {group.rows.map((row) => (
              <Profile
                key={row.name}
                row={row}
                jobs={jobs}
                onJobs={(next) => replace(row, next)}
                onDefault={() => answer('default_profile', [row.name])}
                onRemove={() => setProfiles(rows.filter((one) => one !== row))}
                onAdd={() => setProfiles(rows.map((one) => (one === row ? { ...row, written: true } : one)))}
              />
            ))}
          </div>
        ))}

      {adding ? (
        <NewProfile
          jobs={jobs}
          onAdd={(name, picked) => {
            setProfiles([
              ...rows,
              {
                name,
                jobs: picked,
                default: false,
                proposed: false,
                written: true,
                spelled: spellProfile({ name, jobs: picked }),
              },
            ])
            setAdding(false)
          }}
          onCancel={() => setAdding(false)}
        />
      ) : (
        <Button variant="outline" className="self-start" onClick={() => setAdding(true)}>
          New profile
        </Button>
      )}
    </div>
  )
}

function Profile({
  row,
  jobs,
  onJobs,
  onDefault,
  onRemove,
  onAdd,
}: {
  row: ProfileRow
  jobs: string[]
  onJobs: (jobs: string[]) => void
  onDefault: () => void
  onRemove: () => void
  onAdd: () => void
}) {
  const [editing, setEditing] = useState(false)

  return (
    <section aria-label={`Profile ${row.name}`} className="flex flex-col gap-3 rounded-md border p-4">
      <div className="flex items-center gap-2">
        <span className="font-medium">{row.name}</span>
        {row.default && <Badge variant="secondary">default</Badge>}
        <span className="flex-1 text-sm text-muted-foreground">
          {row.jobs.length} job{row.jobs.length === 1 ? '' : 's'}
        </span>
        {!row.written ? (
          <Button variant="outline" size="sm" onClick={onAdd}>
            Add it
          </Button>
        ) : (
          <>
            {!row.default && (
              <Button variant="ghost" size="sm" onClick={onDefault}>
                Make it the default
              </Button>
            )}
            <Button variant="ghost" size="sm" onClick={() => setEditing(!editing)}>
              {editing ? 'Done' : 'Edit'}
            </Button>
            <Button variant="ghost" size="sm" onClick={onRemove}>
              Remove
            </Button>
          </>
        )}
      </div>
      {editing ? (
        <JobChecks jobs={jobs} picked={row.jobs} onPicked={onJobs} />
      ) : (
        <p className="font-mono text-xs text-muted-foreground">{row.jobs.join(', ')}</p>
      )}
    </section>
  )
}

function JobChecks({
  jobs,
  picked,
  onPicked,
}: {
  jobs: string[]
  picked: string[]
  onPicked: (picked: string[]) => void
}) {
  return (
    <div className="flex flex-wrap gap-x-5 gap-y-2">
      {jobs.map((job) => (
        <Label key={job} className="flex items-center gap-2 font-normal">
          <Checkbox
            checked={picked.includes(job)}
            onCheckedChange={(on) => onPicked(toggled(picked, job, on === true))}
          />
          {job}
        </Label>
      ))}
    </div>
  )
}

function NewProfile({
  jobs,
  onAdd,
  onCancel,
}: {
  jobs: string[]
  onAdd: (name: string, jobs: string[]) => void
  onCancel: () => void
}) {
  const [name, setName] = useState('')
  const [picked, setPicked] = useState<string[]>([])

  return (
    <div className="flex flex-col gap-3 rounded-md border p-4">
      <Input
        aria-label="Profile name"
        placeholder="name"
        value={name}
        onChange={(event) => setName(event.target.value)}
      />
      <JobChecks jobs={jobs} picked={picked} onPicked={setPicked} />
      <div className="flex justify-end gap-2">
        <Button variant="ghost" size="sm" onClick={onCancel}>
          Cancel
        </Button>
        <Button size="sm" disabled={!name.trim() || picked.length === 0} onClick={() => onAdd(name.trim(), picked)}>
          Add the profile
        </Button>
      </div>
    </div>
  )
}
