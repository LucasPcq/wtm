import { useQuery } from '@tanstack/react-query'
import { reposQuery } from '@/api/queries/repos'
import { PageHeader } from '@/components/page-header'
import { Skeleton } from '@/components/ui/skeleton'
import { AddressesPart } from '@/features/run-setup/components/addresses-part'
import { Contents } from '@/features/run-setup/components/contents'
import { EnvPart } from '@/features/run-setup/components/env-part'
import { OpenPart } from '@/features/run-setup/components/open-part'
import { Overview } from '@/features/run-setup/components/overview'
import { PartRow } from '@/features/run-setup/components/part-row'
import { PortsPart } from '@/features/run-setup/components/ports-part'
import { PreviewPanel } from '@/features/run-setup/components/preview-panel'
import { ProfilesPart } from '@/features/run-setup/components/profiles-part'
import { SharedPart } from '@/features/run-setup/components/shared-part'
import { WhatRuns } from '@/features/run-setup/components/what-runs'
import { WriteBlock } from '@/features/run-setup/components/write-block'
import { useRunSetup } from '@/features/run-setup/hooks/use-run-setup'
import { nextPart, type PartProps, type PartState, partState, sectionOf } from '@/features/run-setup/utils/setup'
import { summaryOf } from '@/features/run-setup/utils/summary'
import { isPart, type Part, parts, partWords, skippedWhy } from '@/features/run-setup/utils/words'
import { plural } from '@/lib/plural'
import { repoName } from '@/lib/repo'

export function RunSetupPage({ repo }: { repo: string }) {
  const repos = useQuery(reposQuery)
  const name = repoName(repos.data?.find((one) => one.id === repo)?.root ?? '')
  const { state, dispatch, setup, fingerprint, warnings, failure, planning, write, readAgain } = useRunSetup(repo)

  if (!setup)
    return (
      <div className="flex flex-col">
        <PageHeader title="Set up run" description={failure ? failure.message : `wtm is reading ${name}…`} />
        {!failure && (
          <div className="flex gap-8 px-8 py-6">
            <Skeleton className="h-80 w-50" />
            <Skeleton className="h-96 flex-1" />
            <Skeleton className="h-80 w-92" />
          </div>
        )}
      </div>
    )

  const { form, plan } = setup
  const stateOf = (part: Part) => partState({ form, part, open: state.open, reached: state.reached })
  const states: Record<Part, PartState> = {
    runs: stateOf('runs'),
    shared: stateOf('shared'),
    ports: stateOf('ports'),
    addresses: stateOf('addresses'),
    env: stateOf('env'),
    profiles: stateOf('profiles'),
  }
  const shown = parts.filter((part) => states[part] !== 'skipped')
  // a part opens once every part it waits for is answered, a skipped one counting as answered
  const reachable = (part: Part) =>
    (sectionOf(form, part).waits_for ?? []).every(
      (before) => !isPart(before) || state.reached.includes(before) || states[before] === 'skipped',
    )
  const blocked = () => {
    if (!allReached) return 'Answer every part first'
    if (planning) return 'Planning…'
    return undefined
  }
  const allReached = shown.every((part) => state.reached.includes(part))
  const answer: PartProps['answer'] = (path, values) => {
    write.reset()
    dispatch({ type: 'answer', path, values })
  }

  const detection = plan.detection
  const apps = (detection.candidates ?? []).filter((c) => !c.compose).length
  const read = [
    plural(apps, 'script'),
    detection.compose && `${plural(detection.services?.length ?? 0, 'service')} in ${detection.compose}`,
    plural(detection.env_files?.length ?? 0, '.env file'),
  ].filter(Boolean)
  const description = () => {
    if (write.data?.ok) return `${name}'s run is set up. Open any part to change it.`
    if (state.open === 'none') return 'Read from run.toml. Nothing changed yet.'
    return `wtm read ${name}: ${read.join(', ')}. Nothing is written until the end.`
  }

  const body = (part: Part) => {
    const props = { setup, answer }
    switch (part) {
      case 'runs':
        return <WhatRuns {...props} />
      case 'shared':
        return <SharedPart {...props} />
      case 'ports':
        return <PortsPart {...props} warnings={warnings} />
      case 'addresses':
        return <AddressesPart {...props} />
      case 'env':
        return <EnvPart {...props} warnings={warnings} />
      case 'profiles':
        return <ProfilesPart {...props} />
    }
  }

  // what a part not reached waits for: the overview for the first, the part before for the others
  const waitingFor = (part: Part, at: number) => {
    if (states[part] !== 'to_do') return undefined
    const previous = parts[at - 1]
    if (!previous) return state.open === 'overview' ? 'the overview' : undefined
    return reachable(part) ? undefined : partWords[previous].title
  }

  const titleOf = (next: Part | 'write') => (next === 'write' ? 'Write' : partWords[next].title)

  return (
    <div className="flex flex-col">
      <PageHeader title="Set up run" description={description()} />
      <div className="flex items-start gap-8 px-8 py-6">
        <Contents
          states={states}
          open={state.open}
          reachable={reachable}
          onOpen={(to) => dispatch({ type: 'open', to })}
        />

        <main className="flex min-w-0 flex-1 flex-col gap-4">
          {state.open === 'overview' && (
            <Overview shown={shown} onStart={() => dispatch({ type: 'open', to: shown[0] ?? 'write' })} />
          )}
          {parts.map((part, at) => {
            if (part === state.open) {
              const next = nextPart(form, part)
              return (
                <OpenPart
                  key={part}
                  part={part}
                  number={at + 1}
                  next={titleOf(next)}
                  onContinue={() => dispatch({ type: 'continue', from: part, to: next })}
                >
                  {body(part)}
                </OpenPart>
              )
            }
            const waiting = waitingFor(part, at)
            return (
              <PartRow
                key={part}
                part={part}
                number={at + 1}
                state={states[part]}
                summary={summaryOf(setup, part)}
                waiting={waiting}
                skippedWhy={skippedWhy(sectionOf(form, part))}
                onEdit={waiting || states[part] === 'skipped' ? undefined : () => dispatch({ type: 'open', to: part })}
              />
            )
          })}
          <WriteBlock
            plan={plan}
            blocked={blocked()}
            writing={write.isPending}
            written={write.data}
            failure={write.error}
            onWrite={() => fingerprint && write.mutate(fingerprint)}
            onReadAgain={() => void readAgain()}
          />
        </main>

        <PreviewPanel
          setup={setup}
          open={state.open}
          reached={(part) => state.reached.includes(part) || states[part] === 'skipped'}
          onWorktree={(worktree) => dispatch({ type: 'answer', path: 'preview', values: [worktree] })}
        />
      </div>
    </div>
  )
}
