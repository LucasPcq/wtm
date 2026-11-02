import { plural } from '@/lib/plural'

export const commits = (n: number) => plural(n, 'commit')

export const changes = (n: number | undefined) =>
  n ? `${n} uncommitted ${n === 1 ? 'change' : 'changes'}` : 'uncommitted changes'
