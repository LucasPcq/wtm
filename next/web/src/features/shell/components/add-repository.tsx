import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { CircleCheck, CircleX, Loader2 } from 'lucide-react'
import { useDeferredValue, useState } from 'react'
import { addRepo, placeQuery, reposQuery } from '@/api/queries/repos'
import { DialogBand } from '@/components/dialog-band'
import { RunDialogContent } from '@/components/run-dialog-content'
import { Button } from '@/components/ui/button'
import { Dialog, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { placeAnswer } from '@/features/shell/utils/words'

export function AddRepository({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const client = useQueryClient()
  const navigate = useNavigate()
  const [typed, setTyped] = useState('')
  const path = useDeferredValue(typed.trim())
  const place = useQuery(placeQuery(path))
  const add = useMutation({
    mutationFn: addRepo,
    onSuccess: async (added) => {
      await client.invalidateQueries({ queryKey: reposQuery.queryKey })
      onOpenChange(false)
      setTyped('')
      await navigate({
        to: '/r/$repo/worktrees',
        params: { repo: added.id ?? '' },
        search: added.state === 'not_set_up' ? { setup: true } : {},
      })
    },
  })

  const answer = place.data && !place.isError && path !== '' ? placeAnswer[place.data.state](place.data) : undefined
  const failure = add.error?.message ?? place.error?.message

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <RunDialogContent running={false}>
        <DialogHeader>
          <DialogTitle>Add a repository</DialogTitle>
          <DialogDescription>A git repository on this machine, by its path.</DialogDescription>
        </DialogHeader>

        <DialogBand />

        <Field className="gap-2" data-invalid={answer?.ok === false}>
          <FieldLabel htmlFor="add-repository-path">Path</FieldLabel>
          <Input
            id="add-repository-path"
            autoFocus
            className="font-mono"
            placeholder="~/dev/billing-api"
            aria-invalid={answer?.ok === false}
            value={typed}
            onChange={(e) => {
              add.reset()
              setTyped(e.target.value)
            }}
          />
          {place.isFetching && path !== '' && (
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              Checking…
            </p>
          )}
          {!place.isFetching && answer && (
            <p className="flex items-center gap-2 text-sm" role="status">
              {answer.ok ? (
                <CircleCheck className="size-4 shrink-0 text-green-500" />
              ) : (
                <CircleX className="size-4 shrink-0 text-destructive" />
              )}
              <span className={answer.ok ? undefined : 'text-destructive'}>{answer.line}</span>
            </p>
          )}
          {failure && <p className="text-sm text-destructive">{failure}</p>}
        </Field>

        <DialogFooter className="items-center">
          <Button
            variant="ghost"
            className="mr-auto"
            onClick={() => {
              setTyped('')
              onOpenChange(false)
            }}
          >
            Cancel
          </Button>
          <Button
            disabled={!answer?.primary || place.isFetching || add.isPending}
            onClick={() => place.data && add.mutate(place.data.path)}
          >
            {answer?.primary ?? 'Add'}
          </Button>
        </DialogFooter>
      </RunDialogContent>
    </Dialog>
  )
}
