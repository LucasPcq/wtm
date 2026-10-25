import { queryOptions } from '@tanstack/react-query'
import { json } from '@/api/fetch'
import type { WebRepos } from '@/api/types.gen'

export const reposQuery = queryOptions({ queryKey: ['repos'], queryFn: () => json<WebRepos>('/api/repos') })
