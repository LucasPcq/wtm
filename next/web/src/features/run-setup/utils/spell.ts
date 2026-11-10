// The answers run init takes as text, spelled from a row's own facts when the person changes one. The page never
// reads an answer back: the plan hands each row with its spelling, which is sent as it is when the row is unchanged.

export const spellPort = ({ job, variable, base }: { job: string; variable?: string; base?: number }) =>
  variable ? `${job}.${variable}=${base ?? ''}` : `${job}=`

export const spellCommand = ({ job, cmd }: { job: string; cmd: string }) => `${job}=${cmd}`

export const spellSpace = ({
  service,
  name,
  create,
  remove,
}: {
  service: string
  name: string
  create: string
  remove: string
}) => {
  if (!name) return []
  return [`${service}.name=${name}`, `${service}.create=${create}`, ...(remove ? [`${service}.remove=${remove}`] : [])]
}

export const spellProfile = ({ name, jobs }: { name: string; jobs: string[] }) => `${name}=${jobs.join(',')}`
