import type { ExecStep } from '@/features/worktrees/hooks/use-exec-dialog'

type WizardState = { step: ExecStep; checked: string[]; command: string }

type WizardAction =
  | { type: 'go'; to: ExecStep }
  | { type: 'check'; worktrees: string[]; on: boolean }
  | { type: 'type'; command: string }

export const steps: ExecStep[] = ['worktrees', 'command', 'confirm']

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
    case 'type':
      return { ...state, command: action.command }
  }
}
