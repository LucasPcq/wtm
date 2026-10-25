// expect: none
import { useState } from 'react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { other } from './other'
export function Ok({ title }: { title: string }) {
  const [v] = useState(0)
  return <div className={cn(title)}>{v}{other}<Button /></div>
}
