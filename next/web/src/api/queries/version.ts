import { queryOptions } from '@tanstack/react-query'
import { json } from '@/api/fetch'
import type { WebVersion } from '@/api/types.gen'

export const versionQuery = queryOptions({
  queryKey: ['version'],
  queryFn: () => json<WebVersion>('/api/version'),
  staleTime: Infinity,
})
