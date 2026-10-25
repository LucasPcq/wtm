// The sections of the detail, in its order; each may be hidden, as the columns of the list.
export const sectionIds = ['run', 'review', 'changes', 'activity', 'links'] as const
export type SectionId = (typeof sectionIds)[number]

export const sectionTitle: Record<SectionId, string> = {
  run: 'Run',
  review: 'Review',
  changes: 'Changes',
  activity: 'Activity',
  links: 'Links',
}

// shownSections is a stored choice as the sections to show, in their order; every one when nothing was stored.
export function shownSections(stored: unknown): SectionId[] {
  if (!Array.isArray(stored)) return [...sectionIds]
  return sectionIds.filter((id) => stored.includes(id))
}
