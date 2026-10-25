// expect: nursery/noJsRestrictedProperties
import * as R from 'react'
export function useRun() { R.useEffect(() => {}, []) }
