import { pickedOf, type Setup, selectedOf } from '@/features/run-setup/utils/setup'
import type { Part } from '@/features/run-setup/utils/words'
import { plural } from '@/lib/plural'

const list = (names: string[]) => {
  if (names.length <= 1) return names.join('')
  return `${names.slice(0, -1).join(', ')} and ${names.at(-1)}`
}

// a part folded into one line, from the same dry run the preview and the Write block read
export function summaryOf({ form, plan }: Setup, part: Part): string {
  const jobs = pickedOf(form, 'jobs')
  const candidates = plan.detection.candidates ?? []
  const fromCompose = candidates.filter((c) => c.compose && jobs.includes(c.name)).length
  const shared = pickedOf(form, 'shared')

  switch (part) {
    case 'runs':
      return `${plural(jobs.length, 'job')}: ${fromCompose} from compose, ${jobs.length - fromCompose} from scripts`
    case 'shared': {
      const own = candidates.filter((c) => c.compose && jobs.includes(c.name) && !shared.includes(c.name))
      const said = [
        shared.length > 0 && `${list(shared)} shared`,
        own.length > 0 && `${list(own.map((c) => c.name))} per worktree`,
      ]
      return said.filter(Boolean).join('; ') || 'nothing shared'
    }
    case 'ports': {
      const per = new Set(
        plan.parts.ports.filter((row) => row.variable && !shared.includes(row.job)).map((row) => row.job),
      )
      return `${plural(per.size, 'job')} get${per.size === 1 ? 's' : ''} a port per worktree`
    }
    case 'addresses': {
      if (selectedOf(form, 'addressing') === 'ports') return 'every job by its port'
      const named = pickedOf(form, 'urls')
      return named.length > 0 ? `${list(named)} by name` : 'no job by name'
    }
    case 'env': {
      const linked = plan.parts.links.filter((link) => link.linked)
      return `${plural(linked.length, 'key')} in ${plural(new Set(linked.map((link) => link.file)).size, 'file')} follow the worktree`
    }
    case 'profiles':
      return plan.parts.profiles.map((p) => (p.default ? `${p.name} (default)` : p.name)).join(', ') || 'no profile'
  }
}
