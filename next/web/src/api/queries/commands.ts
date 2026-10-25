import { queryOptions } from '@tanstack/react-query'
import { json } from '@/api/fetch'
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
} from '@/api/types.gen'

// no event invalidates ['github', id]: GitHub is read on demand only
export const repoKey = (repo: string) => ['repo', repo] as const
export const githubKey = (repo: string) => ['github', repo] as const

type ReadDetail = { list: ListDetail; tree: TreeDetail; status: StatusDetail }
export type ReadCommand = keyof ReadDetail

type Mutation = { create: { plan: CreatePlan; detail: CreateDetail } }
export type MutationCommand = keyof Mutation

// the core sends {} for an item it did not read (failed, cancelled)
type Narrowed<Detail, Plan> = Omit<DomainEnvelope, 'plan' | 'items'> & {
  plan: Plan | undefined
  items: (Omit<DomainItem, 'detail'> & { detail: Detail | undefined })[]
}
export type Read<C extends ReadCommand> = Narrowed<ReadDetail[C], never>
export type Applied<C extends MutationCommand> = Narrowed<Mutation[C]['detail'], Mutation[C]['plan']>

// typed once, here, so that no component casts
export const narrowed = <Detail, Plan = never>(envelope: DomainEnvelope): Narrowed<Detail, Plan> => ({
  ...envelope,
  plan: envelope.plan as Plan | undefined,
  items: (envelope.items ?? []).map((item) => ({
    ...item,
    detail: Object.keys(item.detail).length === 0 ? undefined : (item.detail as unknown as Detail),
  })),
})

// a branch of the tree with no worktree of its own is none
export const worktreeCount = ({ items }: Read<'list'> | Read<'tree'>) =>
  items.filter(({ detail }) => !(detail && 'virtual' in detail && detail.virtual)).length

export const run = (repo: string, command: ApiCommandName, call: WebCall) =>
  json<DomainEnvelope>(`/api/repos/${repo}/commands/${command}`, call)

export const readQuery = <C extends ReadCommand>(repo: string, command: C, answers: DomainAnswered[] = []) =>
  queryOptions({
    queryKey: [...repoKey(repo), command, answers],
    queryFn: () => run(repo, command, { answers }),
    select: narrowed<ReadDetail[C]>,
  })

const withPRs: DomainAnswered = { path: 'with_prs', values: ['true'] }

export const githubQuery = <C extends ReadCommand>(repo: string, command: C, answers: DomainAnswered[] = []) =>
  queryOptions({
    queryKey: [...githubKey(repo), command, answers],
    queryFn: () => run(repo, command, { answers: [...answers, withPRs] }),
    select: narrowed<ReadDetail[C]>,
    staleTime: Infinity,
  })

export const fetchOrigin = (repo: string) => run(repo, 'list', { answers: [{ path: 'fetch', values: ['true'] }] })

export const questions = (repo: string, command: ApiCommandName, answers: DomainAnswered[]) =>
  json<DomainForm>(`/api/repos/${repo}/commands/${command}/questions`, { answers } satisfies WebCall)

export const questionsQuery = (repo: string, command: ApiCommandName) =>
  queryOptions({
    queryKey: [...repoKey(repo), command, 'questions'],
    queryFn: () => questions(repo, command, []),
    staleTime: Infinity,
  })

export const dryRunQuery = <C extends MutationCommand>(repo: string, command: C, answers: DomainAnswered[]) =>
  queryOptions({
    queryKey: [...repoKey(repo), command, 'dry_run', answers],
    queryFn: () => run(repo, command, { answers, dry_run: true }),
    select: narrowed<Mutation[C]['detail'], Mutation[C]['plan']>,
  })
