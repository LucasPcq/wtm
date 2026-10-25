import { queryOptions } from '@tanstack/react-query'
import { json } from '../fetch'
import type {
  ApiCommandName,
  CreateDetail,
  CreatePlan,
  DomainAnswered,
  DomainEnvelope,
  DomainForm,
  DomainItem,
  ListDetail,
  StatusDetail,
  TreeDetail,
  WebCall,
} from '../types.gen'

// Everything a repository answers lives under ['repo', id]: an event of that repository invalidates it whole.
export const repoKey = (repo: string) => ['repo', repo] as const

// What GitHub answered lives under ['github', id], which no event invalidates: it is read on demand only.
export const githubKey = (repo: string) => ['github', repo] as const

// The detail of each read's items, as the catalogue's schema names them.
type ReadDetail = { list: ListDetail; tree: TreeDetail; status: StatusDetail }
export type ReadCommand = keyof ReadDetail

// A ReadItem is one subject of a read: its detail, none when the error that kept it from being read left nothing.
export type ReadItem<C extends ReadCommand> = Omit<DomainItem, 'detail'> & { detail: ReadDetail[C] | undefined }
export type Read<C extends ReadCommand> = Omit<DomainEnvelope, 'items'> & { items: ReadItem<C>[] }

const typed =
  <C extends ReadCommand>() =>
  (envelope: DomainEnvelope): Read<C> => ({
    ...envelope,
    items: (envelope.items ?? []).map((item) => ({
      ...item,
      detail:
        item.error && Object.keys(item.detail).length === 0 ? undefined : (item.detail as unknown as ReadDetail[C]),
    })),
  })

// readQuery is a read of the catalogue (list, tree, status) without GitHub: its envelope, kept fresh by the event
// stream.
export const readQuery = <C extends ReadCommand>(repo: string, command: C, answers: DomainAnswered[] = []) =>
  queryOptions({
    queryKey: [...repoKey(repo), command, answers],
    queryFn: () => run(repo, command, { answers }),
    select: typed<C>(),
  })

const withPRs: DomainAnswered = { path: 'with_prs', values: ['true'] }

// githubQuery is the same read asking GitHub too, once, then again only when asked (refreshGithub): GitHub is never
// asked on an event of the stream nor by polling.
export const githubQuery = <C extends ReadCommand>(repo: string, command: C, answers: DomainAnswered[] = []) =>
  queryOptions({
    queryKey: [...githubKey(repo), command, answers],
    queryFn: () => run(repo, command, { answers: [...answers, withPRs] }),
    select: typed<C>(),
    staleTime: Infinity,
  })

// refreshList fetches origin and asks GitHub, as the picker's refresh does: the list it reads is GitHub's list read.
export const refreshList = (repo: string) =>
  run(repo, 'list', { answers: [{ path: 'fetch', values: ['true'] }, withPRs] })

// run executes a command of the catalogue; a refusal throws the core's message.
export const run = (repo: string, command: ApiCommandName, call: WebCall) =>
  json<DomainEnvelope>(`/api/repos/${repo}/commands/${command}`, call)

// questionsQuery is a command's form as the repository stands, before any answer.
export const questionsQuery = (repo: string, command: ApiCommandName) =>
  queryOptions({
    queryKey: [...repoKey(repo), command, 'questions'],
    queryFn: () => questions(repo, command, []),
    staleTime: Infinity,
  })

// The plan of each mutation the web runs and the detail of its items, as the catalogue's schema names them.
type Mutation = { create: { plan: CreatePlan; detail: CreateDetail } }
export type MutationCommand = keyof Mutation

// An Applied is a mutation's envelope, a dry run's or an apply's, its plan and its items' details typed.
export type Applied<C extends MutationCommand> = Omit<DomainEnvelope, 'plan' | 'items'> & {
  plan: Mutation[C]['plan'] | undefined
  items: (Omit<DomainItem, 'detail'> & { detail: Mutation[C]['detail'] })[]
}

// applied narrows a mutation's envelope once, here, so that no component casts it.
export const applied = (envelope: DomainEnvelope): Applied<MutationCommand> => ({
  ...envelope,
  plan: envelope.plan as Mutation[MutationCommand]['plan'] | undefined,
  items: (envelope.items ?? []).map((item) => ({
    ...item,
    detail: item.detail as unknown as Mutation[MutationCommand]['detail'],
  })),
})

// dryRunQuery is what a command would do with answers, and the fingerprint its apply is held to.
export const dryRunQuery = (repo: string, command: MutationCommand, answers: DomainAnswered[]) =>
  queryOptions({
    queryKey: [...repoKey(repo), command, 'dry_run', answers],
    queryFn: () => run(repo, command, { answers, dry_run: true }),
    select: applied,
  })

// questions asks a command's questions as the repository stands, with the answers so far.
export const questions = (repo: string, command: ApiCommandName, answers: DomainAnswered[]) =>
  json<DomainForm>(`/api/repos/${repo}/commands/${command}/questions`, { answers } satisfies WebCall)
