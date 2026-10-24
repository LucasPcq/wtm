import { queryOptions } from '@tanstack/react-query'
import { json } from './fetch'
import type { DomainAnswered, DomainEnvelope, DomainForm } from './types.gen'

// A Repo is a repository the server knows, as GET /api/repos lists it; the published schema holds the catalogue only.
export type Repo = { id: string; root: string; common_dir: string }

export const versionQuery = queryOptions({
  queryKey: ['version'],
  queryFn: () => json<{ version: string }>('/api/version'),
  staleTime: Infinity,
})

export const reposQuery = queryOptions({ queryKey: ['repos'], queryFn: () => json<Repo[]>('/api/repos') })

// Everything a repository answers lives under ['repo', id]: an event of that repository invalidates it whole.
export const readQuery = (repo: string, command: string, answers: DomainAnswered[] = []) =>
  queryOptions({
    queryKey: ['repo', repo, command, answers],
    queryFn: () => json<DomainEnvelope>(`/api/repos/${repo}/commands/${command}`, { answers }),
  })

export const questions = (repo: string, command: string, answers: DomainAnswered[]) =>
  json<DomainForm>(`/api/repos/${repo}/commands/${command}/questions`, { answers })
