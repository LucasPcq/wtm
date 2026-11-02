import { cva } from 'class-variance-authority'

// the tree's children and a worktree's jobs hang from one rail: the list draws it down its left edge, each item an
// elbow from it at the height of its dot; past the last elbow (a pixel below, to keep its corner) the rail is hidden
export const rail = cva('flex flex-col border-l', {
  variants: {
    // node: under the parent's dot (the row's padding, its grip, the gap, half the dot); job: under the group's name
    row: { node: 'ml-[39px] pl-5', job: 'ml-2 pl-3.5' },
  },
})

export const railItem = cva(
  'relative before:absolute before:border-t before:border-border last:after:absolute last:after:bottom-0 last:after:w-px last:after:bg-background',
  {
    variants: {
      row: {
        node: 'before:top-6 before:-left-5 before:w-5 last:after:top-[25px] last:after:-left-[21px]',
        job: 'before:top-4 before:-left-3.5 before:w-3.5 last:after:top-[17px] last:after:-left-[15px]',
      },
    },
  },
)
