import type { RunUpStep } from '@/features/worktrees/hooks/use-run-up-dialog'

// force and exclusive are the person's consent to a refusal, given for these worktrees and this profile only
type WizardState = { step: RunUpStep; checked: string[]; profile: string; force: boolean; exclusive: boolean }

type WizardAction =
  | { type: 'go'; to: RunUpStep }
  | { type: 'check'; worktrees: string[]; on: boolean }
  | { type: 'pick'; profile: string }
  | { type: 'consent'; to: 'force' | 'exclusive' }

export const steps: RunUpStep[] = ['worktrees', 'profile', 'confirm']

export function wizardReducer(state: WizardState, action: WizardAction): WizardState {
  switch (action.type) {
    case 'go':
      return { ...state, step: action.to }
    case 'check':
      return {
        ...state,
        force: false,
        exclusive: false,
        checked: action.on
          ? [...state.checked, ...action.worktrees.filter((worktree) => !state.checked.includes(worktree))]
          : state.checked.filter((worktree) => !action.worktrees.includes(worktree)),
      }
    case 'pick':
      return { ...state, profile: action.profile, force: false, exclusive: false }
    case 'consent':
      return { ...state, [action.to]: true }
  }
}
