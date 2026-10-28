import { queryOptions } from '@tanstack/react-query'
import { json, refusable } from '@/api/fetch'
import type {
  ApiCommandName,
  CleanDetail,
  CleanPlan,
  CreateDetail,
  CreatePlan,
  DomainAnswered,
  DomainEnvelope,
  DomainForm,
  DomainItem,
  FastforwardDetail,
  FastforwardPlan,
  ListDetail,
  PruneDetail,
  PrunePlan,
  StatusDetail,
  TreeDetail,
  WebCall,
} from '@/api/types.gen'

// no event invalidates ['github', id]: GitHub is read on demand only
export const repoKey = (repo: string) => ['repo', repo] as const
export const githubKey = (repo: string) => ['github', repo] as const

type ReadDetail = { list: ListDetail; tree: TreeDetail; status: StatusDetail }
export type ReadCommand = keyof ReadDetail

type Mutation = {
  create: { plan: CreatePlan; detail: CreateDetail }
  clean: { plan: CleanPlan; detail: CleanDetail }
  prune: { plan: PrunePlan; detail: PruneDetail }
  'fast-forward': { plan: FastforwardPlan; detail: FastforwardDetail }
}
export type MutationCommand = keyof Mutation
export type Planned<C extends MutationCommand> = Mutation[C]['plan']

// the core sends {} for an item it did not read (failed, cancelled)
type Narrowed<Detail, Plan> = Omit<DomainEnvelope, 'plan' | 'items'> & {
  plan: Plan | undefined
  items: (Omit<DomainItem, 'detail'> & { detail: Detail | undefined })[]
}
export type Read<C extends ReadCommand> = Narrowed<ReadDetail[C], never>
export type Applied<C extends MutationCommand> = Narrowed<Mutation[C]['detail'], Planned<C>>

// typed once, here, so that no component casts
export const narrowed = <Detail, Plan = never>(envelope: DomainEnvelope): Narrowed<Detail, Plan> => ({
  ...envelope,
  plan: envelope.plan as Plan | undefined,
  items: (envelope.items ?? []).map((item) => ({
    ...item,
    detail: Object.keys(item.detail).length === 0 ? undefined : (item.detail as unknown as Detail),
  })),
})

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
  })

export const fetchOrigin = (repo: string) => run(repo, 'list', { answers: [{ path: 'fetch', values: ['true'] }] })

export const questions = (repo: string, command: ApiCommandName, answers: DomainAnswered[]) =>
  json<DomainForm>(`/api/repos/${repo}/commands/${command}/questions`, { answers } satisfies WebCall)

export const questionsQuery = (repo: string, command: ApiCommandName) =>
  queryOptions({
    queryKey: [...repoKey(repo), command, 'questions'],
    queryFn: () => questions(repo, command, []),
  })

export const dryRunQuery = <C extends MutationCommand>(repo: string, command: C, answers: DomainAnswered[]) =>
  queryOptions({
    queryKey: [...repoKey(repo), command, 'dry_run', answers],
    // a refusal (clean's unsafe worktrees) is the answer of a dry run, not its failure
    queryFn: () => refusable(`/api/repos/${repo}/commands/${command}`, { answers, dry_run: true } satisfies WebCall),
    select: narrowed<Mutation[C]['detail'], Mutation[C]['plan']>,
  })

// a plan chosen on reads is asked again only once they land, keyed on when: under the repository's key, an event would
// ask it again on its old answers, before the reads it was chosen on change them
export const planQuery = <C extends MutationCommand>(
  repo: string,
  command: C,
  { answers, readAt }: { answers: DomainAnswered[]; readAt: number },
) =>
  queryOptions({
    queryKey: ['plan', repo, command, answers, readAt],
    // its key carries the read: never stale, never kept past the opening that asked it
    staleTime: Infinity,
    gcTime: 0,
    queryFn: () => refusable(`/api/repos/${repo}/commands/${command}`, { answers, dry_run: true } satisfies WebCall),
    select: narrowed<Mutation[C]['detail'], Mutation[C]['plan']>,
  })

// a review that fetches origin is asked once per opening: under the repository's key, every event would fetch again
export const reviewQuery = <C extends MutationCommand>(repo: string, command: C, answers: DomainAnswered[]) =>
  queryOptions({
    ...dryRunQuery(repo, command, answers),
    queryKey: ['review', repo, command, answers],
    gcTime: 0,
    refetchOnWindowFocus: false,
  })
