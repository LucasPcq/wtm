import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { HttpError, json } from '@/api/fetch'
import { narrowed, repoKey } from '@/api/queries/commands'
import type { ConfigsetDetail, ConfigsetPlan, DomainAnswered, DomainEnvelope, WebCall } from '@/api/types.gen'
import { fieldProblem } from '@/features/worktrees/settings/utils/words'

const save = async (repo: string, answers: DomainAnswered[]) => {
  const url = `/api/repos/${repo}/commands/config-set`
  const preview = await json<DomainEnvelope>(url, { answers, dry_run: true } satisfies WebCall)
  const applied = await json<DomainEnvelope>(url, {
    answers,
    expect_fingerprint: preview.fingerprint ?? '',
  } satisfies WebCall)
  return narrowed<ConfigsetDetail, ConfigsetPlan>(applied)
}

// a change of config.toml is quick and loses nothing: it previews and applies in one go, its refusal thrown as the
// core's (an HttpError carrying it)
export function useSaveSetting(repo: string) {
  const client = useQueryClient()
  const mutation = useMutation({
    mutationFn: (answers: DomainAnswered[]) => save(repo, answers),
    onSettled: () => client.invalidateQueries({ queryKey: repoKey(repo) }),
  })

  const atOnce = (saved: string, answers: DomainAnswered[]) =>
    mutation.mutateAsync(answers).then(
      (applied) => {
        const undo = applied.follow_ups?.find(({ code }) => code === 'config-set.undo')
        toast.success('Saved', {
          description: saved,
          ...(undo && { action: { label: 'Undo', onClick: () => void atOnce(saved, undo.answers ?? []) } }),
        })
      },
      (failed: Error) => {
        const fields = failed instanceof HttpError ? (failed.refusal?.fields ?? []) : []
        const description = fields.length > 0 ? fields.map(fieldProblem).join('\n') : failed.message
        toast.error("Couldn't save", { description })
      },
    )

  return { save: mutation.mutateAsync, atOnce, saving: mutation.isPending }
}
