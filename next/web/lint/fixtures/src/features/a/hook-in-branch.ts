// expect: correctness/useHookAtTopLevel
import { useState } from 'react'
export function useMaybe(on: boolean) {
  if (on) useState(0)
}
