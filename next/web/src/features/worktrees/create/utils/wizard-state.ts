import type { DomainForm } from '@/api/types.gen'

type WizardState = { at: number; remember: string[] }

type WizardAction = { type: 'go'; to: number } | { type: 'remember'; path: string }

export const initialWizard = (form: DomainForm): WizardState => ({
  at: 0,
  remember: form.fields.filter((field) => 'remember' in field && field.remember).map((field) => field.path),
})

export function wizardReducer(state: WizardState, action: WizardAction): WizardState {
  switch (action.type) {
    case 'go':
      return { ...state, at: action.to }
    case 'remember':
      return {
        ...state,
        remember: state.remember.includes(action.path)
          ? state.remember.filter((path) => path !== action.path)
          : [...state.remember, action.path],
      }
  }
}
