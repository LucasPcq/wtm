// expect: none
import { createRoot } from 'react-dom/client'
import { json } from '@/api/fetch'
import { routeTree } from './routeTree.gen'
export const m = [createRoot, json, routeTree]
