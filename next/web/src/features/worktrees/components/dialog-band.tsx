import type { ReactNode } from 'react'

// the stepper or the progress bar under a run dialog's header, ruled off from the body across the whole dialog
export function DialogBand({ children }: { children: ReactNode }) {
  return <div className="-mx-6 border-b px-6 pb-6">{children}</div>
}
