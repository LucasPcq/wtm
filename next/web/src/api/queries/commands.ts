import { queryOptions } from '@tanstack/react-query'
import { json } from '../fetch'
import type { ApiCommandName, DomainAnswered, DomainEnvelope, DomainForm, WebCall } from '../types.gen'

// Everything a repository answers lives under ['repo', id]: an event of that repository invalidates it whole.
export const repoKey = (repo: string) => ['repo', repo] as const

// readQuery is a read of the catalogue (list, tree, status): its envelope, kept fresh by the event stream.
export const readQuery = (repo: string, command: ApiCommandName, answers: DomainAnswered[] = []) =>
  queryOptions({
    queryKey: [...repoKey(repo), command, answers],
    queryFn: () => run(repo, command, { answers }),
  })

// run executes a command of the catalogue; a refusal throws the core's message.
export const run = (repo: string, command: ApiCommandName, call: WebCall) =>
  json<DomainEnvelope>(`/api/repos/${repo}/commands/${command}`, call)

// questions asks a command's questions as the repository stands, with the answers so far.
export const questions = (repo: string, command: ApiCommandName, answers: DomainAnswered[]) =>
  json<DomainForm>(`/api/repos/${repo}/commands/${command}/questions`, { answers } satisfies WebCall)
