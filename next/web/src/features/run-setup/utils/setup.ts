import type {
  DomainAnswered,
  DomainForm,
  DomainSection,
  RuninitCommandRow,
  RuninitLinkRow,
  RuninitPlan,
  RuninitPortRow,
  RuninitProfileRow,
  RuninitSpaceRow,
} from '@/api/types.gen'
import { type Part, parts } from '@/features/run-setup/utils/words'

// the plan's parts, every list a list: the core writes them so, the schema allows null
export type SpaceRow = Omit<RuninitSpaceRow, 'spelled'> & { spelled: string[] }
export type ProfileRow = Omit<RuninitProfileRow, 'jobs'> & { jobs: string[] }
export type SetupPlan = Omit<RuninitPlan, 'parts'> & {
  parts: {
    ports: RuninitPortRow[]
    commands: RuninitCommandRow[]
    spaces: SpaceRow[]
    links: RuninitLinkRow[]
    profiles: ProfileRow[]
  }
}

export const settled = (plan: RuninitPlan): SetupPlan => ({
  ...plan,
  parts: {
    ports: plan.parts.ports ?? [],
    commands: plan.parts.commands ?? [],
    spaces: (plan.parts.spaces ?? []).map((row) => ({ ...row, spelled: row.spelled ?? [] })),
    links: plan.parts.links ?? [],
    profiles: (plan.parts.profiles ?? []).map((row) => ({ ...row, jobs: row.jobs ?? [] })),
  },
})

export type Setup = { form: DomainForm; plan: SetupPlan }

export const fieldAt = (form: DomainForm, path: string) => form.fields.find((field) => field.path === path)

export function pickedOf(form: DomainForm, path: string): string[] {
  const field = fieldAt(form, path)
  return field?.kind === 'multi_select' ? field.picked : []
}

export function optionsOf(form: DomainForm, path: string): string[] {
  const field = fieldAt(form, path)
  if (field?.kind !== 'multi_select' && field?.kind !== 'select' && field?.kind !== 'text_list') return []
  return (field.options ?? []).map(({ value }) => value)
}

export function selectedOf(form: DomainForm, path: string): string {
  const field = fieldAt(form, path)
  if (field?.kind === 'select' || field?.kind === 'text') return field.value ?? ''
  if (field?.kind === 'confirm') return field.value ? 'true' : 'false'
  return ''
}

export function entriesOf(form: DomainForm, path: string): string[] {
  const field = fieldAt(form, path)
  return field?.kind === 'text_list' ? field.entries : []
}

export const sectionOf = (form: DomainForm, part: Part): DomainSection =>
  form.sections?.find((section) => section.id === part) ?? { id: part, state: 'to_do' }

export type Answers = Record<string, string[]>

export const answered = (answers: Answers): DomainAnswered[] =>
  Object.entries(answers).map(([path, values]) => ({ path, values }))

// a value of a multi-select checked or not, the others as the core picked them
export const toggled = (picked: string[], value: string, on: boolean) =>
  on ? [...picked.filter((one) => one !== value), value] : picked.filter((one) => one !== value)

export type PartState = 'to_do' | 'now' | 'done' | 'to_check' | 'skipped'

export function partState({
  form,
  part,
  open,
  reached,
}: {
  form: DomainForm
  part: Part
  open: string
  reached: Part[]
}): PartState {
  const section = sectionOf(form, part)
  if (section.state === 'skipped') return 'skipped'
  if (part === open) return 'now'
  if (!reached.includes(part)) return 'to_do'
  return section.state === 'to_check' ? 'to_check' : 'done'
}

// the part after this one that the core asks, the Write block once there is none
export const nextPart = (form: DomainForm, part: Part): Part | 'write' =>
  parts.slice(parts.indexOf(part) + 1).find((next) => sectionOf(form, next).state !== 'skipped') ?? 'write'

// a re-init opens on run.toml as written: every candidate the core read from it
export const reInit = (plan: SetupPlan) => (plan.detection.candidates ?? []).some((candidate) => candidate.on_disk)

export type PartProps = { setup: Setup; answer: (path: string, values: string[]) => void }
