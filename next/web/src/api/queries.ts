import { queryOptions } from '@tanstack/react-query'
import { json } from './fetch'
import type { DomainAnswered, DomainEnvelope, DomainForm, WebCall, WebRepos, WebVersion } from './types.gen'

export const versionQuery = queryOptions({
  queryKey: ['version'],
  queryFn: () => json<WebVersion>('/api/version'),
  staleTime: Infinity,
})

export const reposQuery = queryOptions({ queryKey: ['repos'], queryFn: () => json<WebRepos>('/api/repos') })

// Everything a repository answers lives under ['repo', id]: an event of that repository invalidates it whole.
export const readQuery = (repo: string, command: string, answers: DomainAnswered[] = []) =>
  queryOptions({
    queryKey: ['repo', repo, command, answers],
    queryFn: () => json<DomainEnvelope>(`/api/repos/${repo}/commands/${command}`, { answers } satisfies WebCall),
  })

export const questions = (repo: string, command: string, answers: DomainAnswered[]) =>
  json<DomainForm>(`/api/repos/${repo}/commands/${command}/questions`, { answers } satisfies WebCall)
