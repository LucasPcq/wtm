import type { Choice } from '@/features/worktrees/remove/utils/plan'

export type Step = 'worktrees' | 'children' | 'confirm'

type WizardState = { step: Step; choices: Record<string, Choice>; reparent: boolean }

type WizardAction =
  | { type: 'go'; to: Step }
  | { type: 'choose'; worktree: string; choice: Choice }
  | { type: 'reparent'; reparent: boolean }

export const initialWizard: WizardState = { step: 'worktrees', choices: {}, reparent: false }

// a choice is made on the worktrees: one that turned unsafe on a later step brings the wizard back to them
export function wizardReducer(state: WizardState, action: WizardAction): WizardState {
  switch (action.type) {
    case 'go':
      return { ...state, step: action.to }
    case 'choose':
      return { ...state, step: 'worktrees', choices: { ...state.choices, [action.worktree]: action.choice } }
    case 'reparent':
      return { ...state, reparent: action.reparent }
  }
}
