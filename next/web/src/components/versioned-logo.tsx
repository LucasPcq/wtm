import { useQuery } from '@tanstack/react-query'
import { versionQuery } from '@/api/queries/version'
import { Logo } from '@/components/logo'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

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
