import { useState } from 'react'
import type { DomainWarning, RuninitCommandRow, RuninitPortRow } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { CheckRow, ToCheck } from '@/features/run-setup/components/to-check'
import { type PartProps, pickedOf } from '@/features/run-setup/utils/setup'
import { spellCommand, spellPort } from '@/features/run-setup/utils/spell'

export const portIgnored = 'run-init.port_variable_ignored'

export function PortsPart({ setup, answer, warnings }: PartProps & { warnings: DomainWarning[] }) {
  const { form, plan } = setup
  const shared = pickedOf(form, 'shared')
  const ignored = warnings.filter(({ code }) => code === portIgnored)
  const rows = plan.parts.ports
  const groups = [
    { title: 'Per worktree', rows: rows.filter((row) => !shared.includes(row.job)) },
    { title: 'Shared, same port everywhere', rows: rows.filter((row) => shared.includes(row.job)) },
  ].filter((group) => group.rows.length > 0)

  // the commands changed already, from their rows, and this one
  const setCommand = (job: string, spelled: string) =>
    answer('commands', [
      ...plan.parts.commands.filter((row) => row.changed && row.job !== job).map((row) => spellCommand(row)),
      spelled,
    ])

  // the ports of every job as the plan spells them, but the one changed
  const setVariable = (row: RuninitPortRow, variable: string) =>
    answer(
      'ports',
      rows.map((one) => (one === row ? spellPort({ ...row, variable }) : one.spelled)),
    )

  return (
    <div className="flex flex-col gap-6">
      <ToCheck count={ignored.length}>
        {ignored.map(({ params = {} }) => (
          <Ignored
            key={params.job}
            params={params}
            command={plan.parts.commands.find((row) => row.job === params.job)}
            onCommand={(spelled) => setCommand(params.job ?? '', spelled)}
          />
        ))}
      </ToCheck>

      <table className="w-full overflow-hidden rounded-md border text-sm">
        <thead className="bg-muted/50 text-left text-muted-foreground">
          <tr>
            <th className="px-4 py-2 font-normal">Job</th>
            <th className="px-4 py-2 font-normal">Variable</th>
            <th className="px-4 py-2 font-normal">Base</th>
            <th className="px-4 py-2 font-normal">Read from</th>
          </tr>
        </thead>
        {groups.map((group) => (
          <tbody key={group.title}>
            <tr className="border-t bg-muted/30">
              <th colSpan={4} className="px-4 py-2 text-left font-normal text-muted-foreground">
                {group.title}
              </th>
            </tr>
            {group.rows.map((row) => (
              <tr key={row.spelled} className="border-t">
                <td className="px-4 py-2 font-medium">{row.job}</td>
                <td className="px-4 py-2">
                  {row.variable ? (
                    <Input
                      key={row.variable}
                      aria-label={`${row.job} variable`}
                      className="h-8 font-mono text-xs"
                      defaultValue={row.variable}
                      onBlur={(event) => event.target.value !== row.variable && setVariable(row, event.target.value)}
                    />
                  ) : (
                    '—'
                  )}
                </td>
                <td className="px-4 py-2 font-mono">{row.base ?? '—'}</td>
                <td className="px-4 py-2 text-muted-foreground">{row.from ?? 'binds no port'}</td>
              </tr>
            ))}
          </tbody>
        ))}
      </table>
    </div>
  )
}

function Ignored({
  params,
  command,
  onCommand,
}: {
  params: Record<string, string>
  command: RuninitCommandRow | undefined
  onCommand: (spelled: string) => void
}) {
  const [editing, setEditing] = useState(false)
  const [typed, setTyped] = useState(command?.cmd ?? '')
  const job = params.job ?? ''

  if (params.after && command?.spelled)
    return (
      <CheckRow
        subject={job}
        said={`ignores ${params.variable}`}
        action={
          <Button variant="outline" size="sm" onClick={() => onCommand(command.spelled ?? '')}>
            Use this command
          </Button>
        }
      >
        <div className="flex flex-col gap-2">
          <div className="grid grid-cols-[4rem_1fr] gap-x-4 gap-y-1 rounded-md bg-muted/50 p-3 font-mono text-xs">
            <span className="text-muted-foreground">before</span>
            <span>
              {params.before} <span className="text-muted-foreground">(binds {params.port})</span>
            </span>
            <span className="text-muted-foreground">after</span>
            <span>{params.after}</span>
          </div>
          <p className="text-muted-foreground">
            Proposed from the script; wtm can't check it runs. Check it before Write.
          </p>
        </div>
      </CheckRow>
    )

  return (
    <CheckRow
      subject={job}
      said={`ignores ${params.variable}`}
      action={
        !editing && (
          <Button variant="outline" size="sm" onClick={() => setEditing(true)}>
            Edit the command
          </Button>
        )
      }
    >
      <p className="text-muted-foreground">
        Every worktree uses {params.port}; wtm can't tell where its script sets it.
      </p>
      {editing && (
        <div className="mt-2 flex flex-col gap-2">
          <Input
            aria-label={`${job} command`}
            className="font-mono text-xs"
            value={typed}
            onChange={(event) => setTyped(event.target.value)}
          />
          <p className="text-muted-foreground">
            Put ${'{'}
            {params.variable}
            {'}'} where it sets its port. Written when you press Write.
          </p>
          <div className="flex justify-end gap-2">
            <Button variant="ghost" size="sm" onClick={() => setEditing(false)}>
              Cancel
            </Button>
            <Button size="sm" onClick={() => onCommand(spellCommand({ job, cmd: typed }))}>
              Keep this command
            </Button>
          </div>
        </div>
      )}
    </CheckRow>
  )
}
