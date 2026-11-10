import { useMutation, useQuery } from '@tanstack/react-query'
import { type Applied, dryRunQuery, narrowed, run } from '@/api/queries/commands'
import type { DomainForm, InitializeDetail, InitializePlan } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert } from '@/features/worktrees/components/alert'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { Stepper } from '@/features/worktrees/components/stepper'
import { EnvStep } from '@/features/worktrees/set-up/components/env-step'
import { HooksStep } from '@/features/worktrees/set-up/components/hooks-step'
import { SetUpConfirm } from '@/features/worktrees/set-up/components/set-up-confirm'
import { WorktreesStep } from '@/features/worktrees/set-up/components/worktrees-step'
import { useSetUp } from '@/features/worktrees/set-up/hooks/use-set-up'
import { refusalOf, sentence } from '@/features/worktrees/utils/sentence'

export type SetUpDone = Applied<'init'>

export function SetUpWizard({
  repo,
  name,
  form: initial,
  onCancel,
  onDone,
}: {
  repo: string
  name: string
  form: DomainForm
  onCancel: () => void
  onDone: (envelope: SetUpDone) => void
}) {
  const setUp = useSetUp({ repo, form: initial })
  const { form, problems, failure, at, section, confirming, stepper, back, next } = setUp
  const answers = setUp.answers()
  const dry = useQuery({ ...dryRunQuery(repo, 'init', answers), enabled: confirming })
  const apply = useMutation({
    mutationFn: (fingerprint: string) =>
      run(repo, 'init', { answers, expect_fingerprint: fingerprint }).then(narrowed<InitializeDetail, InitializePlan>),
    onSuccess: (envelope) => envelope.ok && onDone(envelope),
    // a plan refused or not written is asked again: the file may have moved since
    onSettled: (envelope) => envelope?.ok || void dry.refetch(),
  })

  const folder = setUp.field('base_path', 'text')
  const branch = setUp.field('base_branch', 'select')
  const strategy = setUp.field('env_strategy', 'select')
  const files = setUp.field('env_files', 'multi_select')
  const onCreate = setUp.field('on_create', 'hooks')
  const onClean = setUp.field('on_clean', 'hooks')

  // a write that failed keeps the answers: the cause says what to fix, then the same plan is set up again
  const couldNot = () => {
    if (apply.isError) return sentence(apply.error.message)
    const envelope = apply.data
    if (!envelope || envelope.ok) return undefined
    const item = envelope.items.find((candidate) => candidate.error)
    return sentence(item?.error?.message ?? envelope.error?.message ?? 'Nothing was written.')
  }
  const notWritten = couldNot()

  const confirm = () => {
    if (dry.isPending) return <Skeleton className="h-40 w-full" />
    if (dry.isError) return <Alert tone="red" description={sentence(dry.error.message)} />
    if (dry.data.error) return <Alert tone="red" description={refusalOf(dry.data.error)} />
    if (!dry.data.plan) return null
    return <SetUpConfirm name={name} settings={dry.data.plan.settings} />
  }

  const description = () => {
    if (notWritten) return 'Nothing was written.'
    if (confirming) return 'Nothing is written yet. These settings stay on this machine, out of git.'
    return 'Its settings stay on this machine, out of git.'
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Set up {name}</DialogTitle>
        <DialogDescription>{description()}</DialogDescription>
      </DialogHeader>

      <DialogBand>
        <Stepper steps={stepper} />
      </DialogBand>

      <DialogScroll>
        {failure && <Alert tone="red" description={sentence(failure.message)} />}
        {problems[''] && <Alert tone="red" description={sentence(problems[''])} />}
        {section === 'worktrees' && folder && branch && (
          <WorktreesStep form={form} name={name} folder={folder} branch={branch} problems={problems} />
        )}
        {section === 'env_files' && strategy && files && (
          <EnvStep form={form} strategy={strategy} files={files} problems={problems} />
        )}
        {section === 'hooks' && onCreate && onClean && (
          <HooksStep form={form} onCreate={onCreate} onClean={onClean} problems={problems} />
        )}
        {confirming && notWritten && (
          <Alert tone="red" title={`Couldn't save ${name}'s settings`} description={notWritten} />
        )}
        {confirming && confirm()}
      </DialogScroll>

      <DialogFooter className="items-center">
        <Button variant="ghost" className="mr-auto" onClick={onCancel}>
          Cancel
        </Button>
        {at > 0 && (
          <Button variant="outline" disabled={apply.isPending} onClick={back}>
            Back
          </Button>
        )}
        {!confirming && (
          <form.Subscribe selector={(s) => s.isValidating}>
            {(validating) => (
              <Button disabled={validating} onClick={next}>
                Next
              </Button>
            )}
          </form.Subscribe>
        )}
        {confirming && (
          <Button
            disabled={!dry.data?.fingerprint || apply.isPending}
            onClick={() => dry.data?.fingerprint && apply.mutate(dry.data.fingerprint)}
          >
            {notWritten ? 'Set up again' : `Set up ${name}`}
          </Button>
        )}
      </DialogFooter>
    </>
  )
}
