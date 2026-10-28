export type Step = 'worktrees' | 'confirm'

export type Tick = { on: boolean; risk: string }

// named: the worktrees Confirm plans; chosen: the precheck went through untouched, so the command picks them itself
type WizardState = { step: Step; checked: Record<string, Tick>; named: string[]; chosen: boolean }

type WizardAction =
  | { type: 'confirm'; named: string[]; chosen: boolean }
  | { type: 'back' }
  | { type: 'check'; rows: { worktree: string; risk: string }[]; on: boolean }

export const initialWizard = ({ picking, worktrees }: { picking: boolean; worktrees: string[] }): WizardState => ({
  step: picking ? 'worktrees' : 'confirm',
  checked: {},
  named: worktrees,
  chosen: false,
})

export function wizardReducer(state: WizardState, action: WizardAction): WizardState {
  switch (action.type) {
    case 'confirm':
      return { ...state, step: 'confirm', named: action.named, chosen: action.chosen }
    case 'back':
      return { ...state, step: 'worktrees' }
    case 'check':
      return {
        ...state,
        checked: {
          ...state.checked,
          ...Object.fromEntries(action.rows.map(({ worktree, risk }) => [worktree, { on: action.on, risk }])),
        },
      }
  }
}
