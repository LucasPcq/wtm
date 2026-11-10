import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useReducer } from 'react'
import { refusable } from '@/api/fetch'
import { dryRunQuery, narrowed, questionsQuery, repoKey } from '@/api/queries/commands'
import type { RuninitDetail, RuninitPlan, WebCall } from '@/api/types.gen'
import { type Answers, answered, reInit, type Setup, settled } from '@/features/run-setup/utils/setup'
import { type Part, parts } from '@/features/run-setup/utils/words'

export type Open = 'overview' | Part | 'write' | 'none'

type State = { read: boolean; answers: Answers; open: Open; reached: Part[] }

type Action =
  | { type: 'read'; reInit: boolean }
  | { type: 'answer'; path: string; values: string[] }
  | { type: 'open'; to: Open }
  | { type: 'continue'; from: Part; to: Open }

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case 'read':
      // a re-init opens on every part folded from run.toml; a first init on the overview
      return action.reInit
        ? { ...state, read: true, open: 'none', reached: [...parts] }
        : { ...state, read: true, open: 'overview' }
    case 'answer':
      return { ...state, answers: { ...state.answers, [action.path]: action.values } }
    case 'open':
      return { ...state, open: action.to }
    case 'continue':
      return {
        ...state,
        open: action.to,
        reached: [...state.reached.filter((part) => part !== action.from), action.from],
      }
  }
}

// the page's answers, sent whole on each change: the core answers with the form as it stands and the plan it makes
export function useRunSetup(repo: string) {
  const client = useQueryClient()
  const [state, dispatch] = useReducer(reducer, { read: false, answers: {}, open: 'overview', reached: [] })
  const sent = answered(state.answers)

  const form = useQuery({ ...questionsQuery(repo, 'run-init', sent), placeholderData: keepPreviousData })
  const planned = useQuery({ ...dryRunQuery(repo, 'run-init', sent), placeholderData: keepPreviousData })
  const plan = planned.data?.error || !planned.data?.plan ? undefined : settled(planned.data.plan)

  if (plan && !state.read) dispatch({ type: 'read', reInit: reInit(plan) })

  const write = useMutation({
    mutationFn: (fingerprint: string) =>
      refusable(`/api/repos/${repo}/commands/run-init`, {
        answers: sent,
        expect_fingerprint: fingerprint,
      } satisfies WebCall).then(narrowed<RuninitDetail, RuninitPlan>),
  })

  // the files are read again, the answers kept: a file changed on disk is planned anew
  const readAgain = () => {
    write.reset()
    return client.invalidateQueries({ queryKey: repoKey(repo) })
  }

  const setup: Setup | undefined = form.data && plan ? { form: form.data, plan } : undefined

  return {
    state,
    dispatch,
    setup,
    fingerprint: planned.data?.fingerprint,
    warnings: planned.data?.warnings ?? [],
    failure: form.error ?? planned.error ?? (planned.data?.error ? new Error(planned.data.error.message) : null),
    planning: planned.isFetching || form.isFetching,
    write,
    readAgain,
  }
}

export type RunSetup = ReturnType<typeof useRunSetup>
