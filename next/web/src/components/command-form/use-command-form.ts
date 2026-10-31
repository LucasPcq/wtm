import { useForm } from '@tanstack/react-form'
import { useRef, useState } from 'react'
import { questions } from '@/api/queries/commands'
import type { ApiCommandName, DomainAnswered, DomainForm } from '@/api/types.gen'
import { answersOf, problemsByKey, valuesOf } from '@/components/command-form/answers'

// The core validates: a change asks the questions again once typing pauses, check asks them at once, and the
// core's answer is the form as it stands now (a field it no longer asks, a default that followed another answer).
export function useCommandForm({
  repo,
  command,
  form: initial,
  extend = (answers) => answers,
  onAnswered,
}: {
  repo: string
  command: ApiCommandName
  form: DomainForm
  extend?: (answers: DomainAnswered[]) => DomainAnswered[]
  onAnswered?: (sent: DomainAnswered[], answered: DomainForm) => void
}) {
  const [asked, setAsked] = useState(initial)
  // the last ask that failed (GitHub down, the server gone), until one goes through
  const [failure, setFailure] = useState<Error>()
  // each call is numbered: an answer overtaken by a later call's is dropped, never kept over it
  const calls = useRef(0)

  const form = useForm({
    defaultValues: valuesOf(initial.fields),
    validators: {
      onChangeAsyncDebounceMs: 300,
      onChangeAsync: async () => {
        await check().catch(() => undefined)
        return undefined
      },
    },
  })

  // only the fields the person changed: a default or a remembered answer stays the core's to decide
  const answers = (): DomainAnswered[] =>
    answersOf(asked.fields, form.state.values, (key) => form.getFieldMeta(key)?.isDirty ?? false)

  // extend adds what only the questions take, never the plan nor the run
  const check = async () => {
    const call = ++calls.current
    const sent = extend(answers())
    try {
      const answered = await questions(repo, command, sent)
      onAnswered?.(sent, answered)
      if (call === calls.current) {
        setAsked(answered)
        setFailure(undefined)
      }
      return answered
    } catch (error) {
      if (call === calls.current) setFailure(error instanceof Error ? error : new Error(String(error)))
      throw error
    }
  }

  return { form, asked, answers, check, failure, problems: problemsByKey(asked.fields, asked.problems) }
}

export type CommandForm = ReturnType<typeof useCommandForm>['form']
