// expect: wtm(test-doubles)
import { vi } from 'vitest'
import * as api from '@/api/fetch'
vi.spyOn(api, 'json')
