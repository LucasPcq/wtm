import { useQuery } from '@tanstack/react-query'
import { versionQuery } from '@/api/queries/version'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

// The mark of docs/dev/identity.md: a dot that splits into three lanes, one branch and three worktrees.
function Mark() {
  return (
    <svg width="30" height="24" viewBox="0 0 30 24" aria-hidden="true">
      <circle cx="4" cy="12" r="3" className="fill-[#17142a] dark:fill-[#f4f1fa]" />
      <path
        d="M6 12 C14 12 14 4 22 4 H27"
        fill="none"
        strokeWidth="3"
        strokeLinecap="round"
        className="stroke-[#d0582f] dark:stroke-[#e8734a]"
      />
      <path
        d="M6 12 H27"
        fill="none"
        strokeWidth="3"
        strokeLinecap="round"
        className="stroke-[#5b4be0] dark:stroke-[#9b8cff]"
      />
      <path
        d="M6 12 C14 12 14 20 22 20 H27"
        fill="none"
        strokeWidth="3"
        strokeLinecap="round"
        className="stroke-[#159c88] dark:stroke-[#5fd4c0]"
      />
    </svg>
  )
}

export function Logo() {
  return (
    <span className="flex items-center gap-2">
      <Mark />
      <span className="text-lg font-bold">wtm</span>
    </span>
  )
}

export function VersionedLogo() {
  const version = useQuery(versionQuery)
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button type="button" className="w-fit rounded-md outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
          <Logo />
        </button>
      </TooltipTrigger>
      <TooltipContent side="right">wtm {version.data?.version ?? '…'}</TooltipContent>
    </Tooltip>
  )
}
