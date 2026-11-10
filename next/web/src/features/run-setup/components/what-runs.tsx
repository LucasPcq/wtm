import { Box, Folder } from 'lucide-react'
import type { RuninitCandidate } from '@/api/types.gen'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { fieldAt, type PartProps, pickedOf, reInit, selectedOf, toggled } from '@/features/run-setup/utils/setup'

// the candidates by where they come from: docker-compose.yml's services, then each folder's scripts
function groupsOf(candidates: RuninitCandidate[], compose: string) {
  const groups: { title: string; compose: boolean; candidates: RuninitCandidate[] }[] = []
  for (const candidate of candidates) {
    const title = candidate.compose ? compose : candidate.place
    const group = groups.find((one) => one.title === title)
    if (group) group.candidates.push(candidate)
    else groups.push({ title, compose: candidate.compose, candidates: [candidate] })
  }
  return groups
}

export function WhatRuns({ setup, answer }: PartProps) {
  const { form, plan } = setup
  const jobs = pickedOf(form, 'jobs')
  const tasks = pickedOf(form, 'tasks')
  const runner = plan.detection.runner
  const asksRunner = fieldAt(form, 'runner')?.skip === undefined && runner !== undefined
  const fresh = reInit(plan)

  return (
    <div className="flex flex-col gap-6">
      <div className="overflow-hidden rounded-md border">
        {groupsOf(plan.detection.candidates ?? [], plan.detection.compose).map((group) => (
          <section key={group.title} aria-label={group.title}>
            <div className="flex items-center gap-2 border-b bg-muted/50 px-4 py-2 text-sm">
              {group.compose ? (
                <Box className="size-4" aria-hidden="true" />
              ) : (
                <Folder className="size-4" aria-hidden="true" />
              )}
              <span className="font-mono">{group.title}</span>
              {group.compose && <span className="text-muted-foreground">started and stopped by docker compose</span>}
            </div>
            {group.candidates.map((candidate) => (
              <div key={candidate.name} className="flex items-center gap-3 border-b px-4 py-2.5 last:border-b-0">
                <Checkbox
                  id={`job-${candidate.name}`}
                  checked={jobs.includes(candidate.name)}
                  onCheckedChange={(on) => answer('jobs', toggled(jobs, candidate.name, on === true))}
                />
                <Label htmlFor={`job-${candidate.name}`} className="w-28 font-medium">
                  {candidate.name}
                </Label>
                <span className="min-w-0 flex-1 truncate font-mono text-xs text-muted-foreground">
                  {candidate.image ?? candidate.cmd}
                </span>
                {fresh && !candidate.on_disk && <Badge variant="outline">new</Badge>}
                <Select
                  value={tasks.includes(candidate.name) ? 'task' : 'service'}
                  disabled={!jobs.includes(candidate.name)}
                  onValueChange={(kind) => answer('tasks', toggled(tasks, candidate.name, kind === 'task'))}
                >
                  <SelectTrigger className="w-28" aria-label={`${candidate.name} kind`}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="service">service</SelectItem>
                    <SelectItem value="task">task</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            ))}
          </section>
        ))}
      </div>

      {asksRunner && (
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1">
            <h3 className="font-medium">The root dev script starts the apps</h3>
            <p className="text-sm text-muted-foreground">
              Run its apps one by one, each with its own port, logs and restart, or keep{' '}
              <code className="font-mono">{runner.cmd}</code> as one job.
            </p>
          </div>
          <RadioGroup
            value={selectedOf(form, 'runner')}
            onValueChange={(value) => answer('runner', [value])}
            className="flex gap-6"
          >
            <Label className="flex items-center gap-2 font-normal">
              <RadioGroupItem value="one_by_one" /> One by one
            </Label>
            <Label className="flex items-center gap-2 font-normal">
              <RadioGroupItem value="through" /> Through {runner.cmd}
            </Label>
          </RadioGroup>
        </div>
      )}
    </div>
  )
}
