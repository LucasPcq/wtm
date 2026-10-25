// expect: wtm(effects)
import React from 'react'
export function useRun() {
  React.useInsertionEffect(() => {}, [])
}
