import { saved } from '@/api/fetch'

export const downloadDaemonLog = () =>
  saved('/api/daemon/log', { name: 'daemon.log', failure: "The log of wtm's daemon could not be saved" })
