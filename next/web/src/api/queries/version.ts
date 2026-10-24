import { queryOptions } from '@tanstack/react-query'
import { json } from '../fetch'
import type { WebVersion } from '../types.gen'

export const versionQuery = queryOptions({
  queryKey: ['version'],
  queryFn: () => json<WebVersion>('/api/version'),
  staleTime: Infinity,
})
