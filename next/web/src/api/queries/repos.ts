import { queryOptions } from '@tanstack/react-query'
import { json } from '../fetch'
import type { WebRepos } from '../types.gen'

export const reposQuery = queryOptions({ queryKey: ['repos'], queryFn: () => json<WebRepos>('/api/repos') })
