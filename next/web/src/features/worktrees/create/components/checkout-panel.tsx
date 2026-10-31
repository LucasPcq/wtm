import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { HttpError } from '@/api/fetch'
import { checkoutFormQuery } from '@/api/queries/commands'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { NextButton } from '@/features/worktrees/components/next-button'
import { Stepper } from '@/features/worktrees/components/stepper'
import { CheckoutWizard } from '@/features/worktrees/create/components/checkout-wizard'
import { GitHubAlert } from '@/features/worktrees/create/components/github-alert'
import { ParentPlaceholder } from '@/features/worktrees/create/components/parent-placeholder'
import { unavailableLine } from '@/features/worktrees/create/utils/checkout-words'
import type { Start } from '@/features/worktrees/hooks/use-run'

export function CheckoutPanel({
  repo,
  startFrom,
  onStart,
  onCancel,
  onNewBranch,
}: {
  repo: string
  startFrom: ReactNode
  onStart: (start: Start) => void
  onCancel: () => void
  onNewBranch: () => void
}) {
  const form = useQuery(checkoutFormQuery(repo))

  if (form.data)
    return (
      <CheckoutWizard
        repo={repo}
        form={form.data}
        startFrom={startFrom}
        onStart={onStart}
        onCancel={onCancel}
        onNewBranch={onNewBranch}
      />
    )

  // GitHub could not be asked: no form at all, the step keeps its shape, and a new branch is the way on
  const refusal = form.error instanceof HttpError ? form.error.refusal : undefined
  const why = form.isError ? unavailableLine(refusal, form.error.message).next : 'Asking GitHub for its pull requests'

  return (
    <>
      <DialogHeader>
        <DialogTitle>Create worktrees</DialogTitle>
        <DialogDescription>Each branch gets its own folder, .env and ports.</DialogDescription>
      </DialogHeader>

      <DialogBand>
        <Stepper
          steps={[
            { title: 'Branch', state: 'now' },
            { title: 'Environment', state: 'todo' },
            { title: 'Confirm', state: 'todo' },
          ]}
        />
      </DialogBand>

      <DialogScroll>
        {startFrom}
        {form.isPending && <Skeleton className="h-48 w-full" />}
        {form.isError && <GitHubAlert refusal={refusal} message={form.error.message} onNewBranch={onNewBranch} />}
        <ParentPlaceholder />
      </DialogScroll>

      <DialogFooter className="items-center">
        <Button variant="ghost" className="mr-auto" onClick={onCancel}>
          Cancel
        </Button>
        <NextButton disabled why={why} onClick={() => undefined} />
      </DialogFooter>
    </>
  )
}
