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

// sentence is a message of the core, which reads as a clause ("its pull request is merged"), as a sentence; one that
// opens on a name (a branch, a file) keeps it as it is spelled.
export const sentence = (message: string) =>
  /^[a-z]+ /.test(message) ? message.charAt(0).toUpperCase() + message.slice(1) : message
