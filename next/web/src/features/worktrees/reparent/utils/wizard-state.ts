export type Step = 'worktrees' | 'parent' | 'confirm'

type WizardState = { step: Step; picking: boolean; checked: string[]; to: string }

type WizardAction =
  | { type: 'check'; worktrees: string[]; on: boolean }
  | { type: 'to'; to: string }
  | { type: 'next' }
  | { type: 'back' }

export const initialWizard = ({
  picking,
  worktrees,
  to,
}: {
  picking: boolean
  worktrees: string[]
  to: string | undefined
}): WizardState => {
  const step = () => {
    if (picking) return 'worktrees'
    return to ? 'confirm' : 'parent'
  }
  return { step: step(), picking, checked: worktrees, to: to ?? '' }
}

const order: Step[] = ['worktrees', 'parent', 'confirm']

export function wizardReducer(state: WizardState, action: WizardAction): WizardState {
  const at = order.indexOf(state.step)
  switch (action.type) {
    case 'check':
      return {
        ...state,
        checked: action.on
          ? [...state.checked, ...action.worktrees.filter((worktree) => !state.checked.includes(worktree))]
          : state.checked.filter((worktree) => !action.worktrees.includes(worktree)),
      }
    case 'to':
      return { ...state, to: action.to }
    case 'next':
      return { ...state, step: order[at + 1] ?? state.step }
    case 'back': {
      const back = order[at - 1]
      // one worktree was never picked: there is no step of it to go back to
      return back && (back !== 'worktrees' || state.picking) ? { ...state, step: back } : state
    }
  }
}
