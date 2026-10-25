// expect: none
import { useQuery } from '@tanstack/react-query'
import { json } from '@/api/fetch'
import { cn } from '@/lib/utils'
import { Glyph } from '../a/glyph'
export const ok = [useQuery, json, cn, Glyph]
