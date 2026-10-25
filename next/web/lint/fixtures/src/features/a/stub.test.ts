// expect: nursery/noJsRestrictedProperties
import { vi } from 'vitest'
vi.stubGlobal('fetch', () => 1)
