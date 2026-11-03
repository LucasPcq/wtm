import type { StepState } from '@/features/worktrees/components/stepper'

export type Step = 'worktrees' | 'children' | 'data' | 'confirm'

// the steps a plan may leave out: asked only when it has children to move, or data a service down holds
export type Asked = Record<'children' | 'data', boolean>

const order: Step[] = ['worktrees', 'children', 'data', 'confirm']

const askedOf = (step: Step, asked: Asked) => step === 'worktrees' || step === 'confirm' || asked[step]

// the step after (or before) one, past those the plan does not ask
export const nextStep = (step: Step, asked: Asked) =>
  order.slice(order.indexOf(step) + 1).find((one) => askedOf(one, asked)) ?? 'confirm'
export const previousStep = (step: Step, asked: Asked) =>
  order
    .slice(0, order.indexOf(step))
    .reverse()
    .find((one) => askedOf(one, asked)) ?? 'worktrees'

// the step reached, moved on once a fresh plan no longer asks it
export const settledStep = ({ chosen, asked, fresh }: { chosen: Step; asked: Asked; fresh: boolean }): Step =>
  fresh && !askedOf(chosen, asked) ? nextStep(chosen, asked) : chosen

export function stepState({ of, step, asked }: { of: Step; step: Step; asked: Asked }): StepState {
  if (of === step) return 'now'
  if (order.indexOf(of) > order.indexOf(step)) return 'todo'
  return askedOf(of, asked) ? 'done' : 'skipped'
}

export type Tick = { on: boolean; risk: string }

type WizardState = {
  step: Step
  checked: Record<string, Tick>
  confirmed: string[]
  reparent: boolean
  dropData: boolean
}

type WizardAction =
  | { type: 'go'; to: Step; confirmed?: string[] }
  | { type: 'check'; rows: { worktree: string; risk: string }[]; on: boolean }
  | { type: 'reparent'; reparent: boolean }
  | { type: 'drop_data'; dropData: boolean }

export const initialWizard: WizardState = {
  step: 'worktrees',
  checked: {},
  confirmed: [],
  reparent: false,
  dropData: false,
}

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
    case 'drop_data':
      return { ...state, dropData: action.dropData }
  }
}
