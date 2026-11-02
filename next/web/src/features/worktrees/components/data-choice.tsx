import { ChoiceOptions } from '@/components/choice-options'
import { listed } from '@/features/worktrees/utils/listed'

export function DataChoice({
  down,
  dropData,
  onDropData,
}: {
  down: string[]
  dropData: boolean
  onDropData: (on: boolean) => void
}) {
  const services = listed(down)

  return (
    <section aria-label="Data" className="flex flex-col gap-2">
      <p className="text-sm">{`${services} ${down.length === 1 ? 'is' : 'are'} down: the data these worktrees made there can't be dropped yet.`}</p>
      <ChoiceOptions
        id="drop-data"
        label="What becomes of their data"
        value={dropData ? 'start' : 'defer'}
        options={[
          {
            value: 'defer',
            title: `Keep it until ${services} next starts`,
            description: 'The next run up or prune drops it.',
          },
          {
            value: 'start',
            title: `Start ${services} and drop it now`,
            description: 'Stopped again once the data is dropped.',
          },
        ]}
        onChange={(value) => onDropData(value === 'start')}
      />
    </section>
  )
}
