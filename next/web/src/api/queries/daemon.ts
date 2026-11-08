import { saved } from '@/api/fetch'

export const downloadDaemonLog = () =>
  saved('/api/daemon/log', { name: 'daemon.log', failure: 'the daemon’s log could not be saved' })
