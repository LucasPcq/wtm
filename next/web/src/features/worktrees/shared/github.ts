import type { DomainWarning } from '@/api/types.gen'

// unavailable is why GitHub could not be asked, as a read's warnings say, none when it answered.
export const unavailable = (warnings: DomainWarning[] | undefined) =>
  warnings?.find((warning) => warning.code.endsWith('.prs_unavailable'))
