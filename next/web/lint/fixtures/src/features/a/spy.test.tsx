// expect: plugin/spy-module
import { vi } from 'vitest'
import * as api from '@/api/fetch'
vi.spyOn(api, 'json')
