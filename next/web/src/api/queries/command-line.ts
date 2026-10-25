import { queryOptions } from '@tanstack/react-query'
import { json } from '../fetch'
import type { DomainFollowUp, WebCommandLine } from '../types.gen'

// commandLineQuery is a follow-up the catalogue cannot run, as the command line runs it: its argv, empty when it
// cannot be spelled. It never changes for a follow-up.
export const commandLineQuery = (followUp: DomainFollowUp) =>
  queryOptions({
    queryKey: ['command-line', followUp],
    queryFn: () => json<WebCommandLine>('/api/command-line', followUp),
    staleTime: Infinity,
  })
