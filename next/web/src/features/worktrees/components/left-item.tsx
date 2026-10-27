import { Item, ItemTitle } from '@/components/ui/item'

export function LeftItem({ worktree, why }: { worktree: string; why: string }) {
  return (
    <Item role="listitem" aria-label={worktree} className="flex-col items-start gap-1 rounded-none px-4 py-3">
      <ItemTitle className="text-muted-foreground">{worktree}</ItemTitle>
      <span className="text-xs text-muted-foreground">{why}</span>
    </Item>
  )
}
