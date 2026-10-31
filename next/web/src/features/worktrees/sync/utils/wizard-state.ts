import type { DomainAnswered } from '@/api/types.gen'
import { type Switches, switchesOf } from '@/features/worktrees/sync/utils/plan'

export type Step = 'worktrees' | 'confirm'

export type Tick = { on: boolean; risk: string }

type WizardState = { step: Step; checked: Record<string, Tick>; switches: Switches }

type WizardAction =
  | { type: 'step'; step: Step }
  | { type: 'check'; rows: { worktree: string; risk: string }[]; on: boolean }
  | { type: 'switch'; name: keyof Switches; on: boolean }

export const initialWizard = ({ answers, step }: { answers: DomainAnswered[]; step: Step }): WizardState => ({
  step,
  checked: {},
  switches: switchesOf(answers),
})

export function wizardReducer(state: WizardState, action: WizardAction): WizardState {
  switch (action.type) {
    case 'step':
      return { ...state, step: action.step }
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
