// expect: none
import { useQuery } from '@tanstack/react-query'
import { json } from '@/api/fetch'
import { cn } from '@/lib/utils'
import { Glyph } from '@/features/a/glyph'
import { y } from './deep/er/aliased'
export const ok = [useQuery, json, cn, Glyph, y]
