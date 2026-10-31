import type { DomainAnswered } from '@/api/types.gen'
import { type Switches, switchesOf } from '@/features/worktrees/sync/utils/plan'

export type Step = 'worktrees' | 'confirm'

export type Tick = { on: boolean; risk: string }

// opening: the review has not landed yet, which decides once whether a named worktree needs consent on step 1
type WizardState = { step: Step; opening: boolean; checked: Record<string, Tick>; switches: Switches }

type WizardAction =
  | { type: 'step'; step: Step }
  | { type: 'opened'; consent: boolean }
  | { type: 'check'; rows: { worktree: string; risk: string }[]; on: boolean }
  | { type: 'switch'; name: keyof Switches; on: boolean }

export const initialWizard = ({ answers, step }: { answers: DomainAnswered[]; step: Step }): WizardState => ({
  step,
  opening: true,
  checked: {},
  switches: switchesOf(answers),
})

export function wizardReducer(state: WizardState, action: WizardAction): WizardState {
  switch (action.type) {
    case 'step':
      return { ...state, step: action.step }
    case 'opened':
      return { ...state, opening: false, step: action.consent ? 'worktrees' : state.step }
    case 'check':
      return {
        ...state,
        checked: {
          ...state.checked,
          ...Object.fromEntries(action.rows.map(({ worktree, risk }) => [worktree, { on: action.on, risk }])),
        },
      }
    case 'switch':
      return { ...state, switches: { ...state.switches, [action.name]: action.on } }
  }
}
