import { queryOptions } from '@tanstack/react-query'
import { json } from '@/api/fetch'
import type { DomainFollowUp, WebCommandLine } from '@/api/types.gen'

export const shellLine = (argv: string[]) =>
  argv.map((arg) => (/^[\w@%+=:,./-]+$/.test(arg) ? arg : `'${arg.replaceAll("'", `'\\''`)}'`)).join(' ')

export const commandLineQuery = (followUp: DomainFollowUp) =>
  queryOptions({
    queryKey: ['command-line', followUp],
    queryFn: () => json<WebCommandLine>('/api/command-line', followUp),
    select: ({ argv }) => (argv.length > 0 ? shellLine(argv) : undefined),
    staleTime: Infinity,
  })
