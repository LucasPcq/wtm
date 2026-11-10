import { useQuery, useQueryClient } from '@tanstack/react-query'
import { FolderPlus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { questionsQuery } from '@/api/queries/commands'
import { reposQuery } from '@/api/queries/repos'
import { Centered } from '@/components/centered'
import { RunDialogContent } from '@/components/run-dialog-content'
import { StatePage } from '@/components/state-page'
import { Button } from '@/components/ui/button'
import { Dialog, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { type SetUpDone, SetUpWizard } from '@/features/worktrees/set-up/components/set-up-wizard'
import { sentence } from '@/features/worktrees/utils/sentence'
import { repoName } from '@/lib/repo'

// a repository the web serves but wtm hasn't set up: its page is this, until its settings are written
export function SetUpGate({ repo, root, opened = false }: { repo: string; root: string; opened?: boolean }) {
  const client = useQueryClient()
  const [open, setOpen] = useState(opened)
  // each opening asks afresh: Cancel then Set up starts from what the repository holds now
  const form = useQuery({ ...questionsQuery(repo, 'init'), enabled: open, gcTime: 0 })
  const name = repoName(root)

  const done = async (envelope: SetUpDone) => {
    setOpen(false)
    const runInit = envelope.follow_ups.some((followUp) => followUp.code === 'init.run_init')
    toast.success(`Set up ${name}`, {
      ...(runInit && { description: 'Next, set up run: what runs, its ports and addresses.' }),
    })
    await client.invalidateQueries({ queryKey: reposQuery.queryKey })
  }

  const content = () => {
    if (form.data)
      return (
        <SetUpWizard
          repo={repo}
          name={name}
          form={form.data}
          onCancel={() => setOpen(false)}
          onDone={(envelope) => void done(envelope)}
        />
      )

    return (
      <>
        <DialogHeader>
          <DialogTitle>Set up {name}</DialogTitle>
          <DialogDescription>
            {form.isError ? sentence(form.error.message) : `Reading what ${name} holds…`}
          </DialogDescription>
        </DialogHeader>
        {form.isPending && <Skeleton className="h-48 w-full" />}
      </>
    )
  }

  return (
    <Centered>
      <StatePage
        icon={<FolderPlus />}
        title={`Set up ${name}`}
        action={
          <Dialog open={open} onOpenChange={setOpen}>
            <Button onClick={() => setOpen(true)}>Set up {name}</Button>
            <RunDialogContent running={false} className="sm:max-w-2xl">
              {content()}
            </RunDialogContent>
          </Dialog>
        }
      >
        This repository isn't set up yet. Three steps, each filled from what's found in it: worktrees, .env files and
        hooks. Nothing is written until the last.
      </StatePage>
    </Centered>
  )
}
