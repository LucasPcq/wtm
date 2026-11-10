import { HttpResponse, http } from 'msw'
import type { DomainAnswered, DomainForm, DomainWarning, RuninitPlan, WebCall } from '@/api/types.gen'
import { envelope } from '@/test/fixtures'
import captured from '@/test/run-init.json'

// acme's run init as wtm-next ui answered it on the monorepo of next/docs/proof/run-init-repo.sh, with a worktree
// feat/billing never started: its first form and its first dry run, nothing answered
export const runInitForm = captured.form as DomainForm

export const runInitPlan = captured.plan as RuninitPlan

const warnings = captured.warnings as DomainWarning[]

const asked = (answers: DomainAnswered[] | undefined, path: string) => answers?.find((a) => a.path === path)?.values

// the form as the core gives it back: each answer the person gave, and the ports part answered once a command reads
// its variable
export function runInitFormOf(answers: DomainAnswered[] = []): DomainForm {
  const fields = runInitForm.fields.map((field) => {
    const values = asked(answers, field.path)
    if (!values) return field
    switch (field.kind) {
      case 'multi_select':
        return { ...field, picked: values }
      case 'select':
        return { ...field, value: values[0] ?? '' }
      case 'text_list':
        return { ...field, entries: values }
      case 'confirm':
        return { ...field, value: values[0] === 'true' }
      default:
        return field
    }
  })
  const fixed = asked(answers, 'commands')?.length
  const sections = (runInitForm.sections ?? []).map((section) =>
    section.id === 'ports' && fixed
      ? { id: section.id, state: 'answered' as const, waits_for: section.waits_for ?? [] }
      : section,
  )
  return { ...runInitForm, fields, sections }
}

export function runInitPlanOf(answers: DomainAnswered[] = []): RuninitPlan {
  const commands = asked(answers, 'commands') ?? []
  return {
    ...runInitPlan,
    parts: {
      ...runInitPlan.parts,
      commands: (runInitPlan.parts.commands ?? []).map((row) => ({
        ...row,
        changed: commands.includes(row.spelled ?? ''),
      })),
    },
  }
}

export const runInitEnvelope = (call: WebCall) => ({
  ...envelope('run-init', []),
  op: 'op-1',
  fingerprint: 'f-run-init',
  plan: runInitPlanOf(call.answers),
  warnings: asked(call.answers, 'commands')?.length ? [] : warnings,
})

export const runInitApplied = {
  ...envelope('run-init', []),
  op: 'op-2',
  items: [
    { subject: 'docker-compose.yml', status: 'done', detail: { path: 'docker-compose.yml', tracked: true } },
    { subject: 'run.toml', status: 'done', detail: { path: 'run.toml', tracked: false } },
  ],
  follow_ups: [{ command: 'run up', code: 'run-init.up', message: 'start the jobs' }],
}

export const answeringRunInit = () => [
  http.post('/api/repos/r1/commands/run-init/questions', async ({ request }) =>
    HttpResponse.json(runInitFormOf(((await request.json()) as WebCall).answers)),
  ),
  http.post('/api/repos/r1/commands/run-init', async ({ request }) => {
    const call = (await request.json()) as WebCall
    return HttpResponse.json(call.dry_run ? runInitEnvelope(call) : runInitApplied)
  }),
]
