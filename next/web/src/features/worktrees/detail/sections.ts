export const sectionIds = ['run', 'review', 'changes', 'activity', 'links'] as const
export type SectionId = (typeof sectionIds)[number]

export const sectionTitle: Record<SectionId, string> = {
  run: 'Run',
  review: 'Review',
  changes: 'Changes',
  activity: 'Activity',
  links: 'Links',
}
