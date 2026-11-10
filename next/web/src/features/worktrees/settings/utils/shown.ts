import type { DomainField } from '@/api/types.gen'
import { plural } from '@/lib/plural'

// a setting's value on one line, as its well and the changed-on-disk alert show it
export function shown(field: DomainField): string {
  switch (field.kind) {
    case 'select':
    case 'text':
      return field.value
    case 'multi_select':
      return plural(field.picked.length, 'file')
    case 'hooks':
      return field.entries.map(({ cmd }) => cmd).join(', ') || 'none'
    case 'text_list':
      return field.entries.join(', ')
    case 'confirm':
      return String(field.value)
  }
}
