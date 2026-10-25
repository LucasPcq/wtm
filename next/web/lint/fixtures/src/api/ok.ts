// expect: none
import { createParser } from 'eventsource-parser'
import { token } from './token'
export const get = () => fetch('/x', { headers: { a: token } })
export const s = () => new EventSource('/x')
export const p = createParser
