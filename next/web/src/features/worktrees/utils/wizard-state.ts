export type Step = 'worktrees' | 'children' | 'confirm'

export type Tick = { on: boolean; risk: string }

type WizardState = { step: Step; checked: Record<string, Tick>; confirmed: string[]; reparent: boolean }

type WizardAction =
  | { type: 'go'; to: Step; confirmed?: string[] }
  | { type: 'check'; rows: { worktree: string; risk: string }[]; on: boolean }
  | { type: 'reparent'; reparent: boolean }

export const initialWizard: WizardState = { step: 'worktrees', checked: {}, confirmed: [], reparent: false }

export function wizardReducer(state: WizardState, action: WizardAction): WizardState {
  switch (action.type) {
    case 'go':
      return { ...state, step: action.to, confirmed: action.confirmed ?? state.confirmed }
    case 'check':
      return {
        ...state,
        checked: {
          ...state.checked,
          ...Object.fromEntries(action.rows.map(({ worktree, risk }) => [worktree, { on: action.on, risk }])),
        },
      }
    case 'reparent':
      return { ...state, reparent: action.reparent }
  }
}
