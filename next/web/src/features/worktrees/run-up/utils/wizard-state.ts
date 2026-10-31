import type { RunUpStep } from '@/features/worktrees/hooks/use-run-up-dialog'

type WizardState = { step: RunUpStep; checked: string[]; profile: string }

type WizardAction =
  | { type: 'go'; to: RunUpStep }
  | { type: 'check'; worktrees: string[]; on: boolean }
  | { type: 'pick'; profile: string }

export const steps: RunUpStep[] = ['worktrees', 'profile', 'confirm']

export function wizardReducer(state: WizardState, action: WizardAction): WizardState {
  switch (action.type) {
    case 'go':
      return { ...state, step: action.to }
    case 'check':
      return {
        ...state,
        checked: action.on
          ? [...state.checked, ...action.worktrees.filter((worktree) => !state.checked.includes(worktree))]
          : state.checked.filter((worktree) => !action.worktrees.includes(worktree)),
      }
    case 'pick':
      return { ...state, profile: action.profile }
  }
}
