import { queryOptions } from '@tanstack/react-query'
import { json } from '@/api/fetch'
import type { WebPlace, WebRepos } from '@/api/types.gen'

export const reposQuery = queryOptions({ queryKey: ['repos'], queryFn: () => json<WebRepos>('/api/repos') })

// where a typed path stands, asked again as it changes: never kept past the dialog that asked it
export const placeQuery = (path: string) =>
  queryOptions({
    queryKey: ['place', path],
    queryFn: () => json<WebPlace>('/api/repos/check', { path }),
    enabled: path !== '',
    gcTime: 0,
    retry: false,
  })

export const addRepo = (path: string) => json<WebPlace>('/api/repos', { path })
