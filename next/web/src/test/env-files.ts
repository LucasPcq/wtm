import type { EnvshowDetail } from '@/api/types.gen'

// acme's .env files: feat/billing's web PORT out of step and a MAILER_URL that follows nothing, its secret withheld
export const billingFiles = (shown: boolean): EnvshowDetail => ({
  worktree: 'feat/billing',
  path: '/acme.trees/feat-billing',
  main: false,
  strategy: 'example',
  isolation: 'isolated',
  pending: false,
  apps: [
    {
      dir: 'apps/api',
      jobs: ['api'],
      keys: 3,
      following: 1,
      to_check: 1,
      files: [
        {
          target: 'apps/api/.env',
          name: '.env',
          origin: 'template',
          missing: false,
          keys: [
            {
              key: 'API_PORT',
              follows: 'port',
              job: 'api',
              port: 'PORT',
              main: '4000',
              here: '4020',
              writes: '4020',
              withheld: false,
              status: 'in_step',
              out_of_step: false,
            },
            {
              key: 'MAILER_URL',
              follows: 'nothing',
              job: 'stack',
              port: 'MAILPIT_UI',
              main: shown ? 'http://localhost:8025' : '',
              here: shown ? 'http://localhost:8025' : '',
              withheld: !shown,
              status: 'in_step',
              out_of_step: false,
            },
            {
              key: 'JWT_SECRET',
              follows: 'copied',
              main: shown ? 's3cret' : '',
              here: shown ? 's3cret' : '',
              withheld: !shown,
              status: 'in_step',
              out_of_step: false,
            },
          ],
        },
      ],
    },
    {
      dir: 'apps/web',
      jobs: ['web'],
      keys: 1,
      following: 1,
      to_check: 1,
      files: [
        {
          target: 'apps/web/.env',
          name: '.env',
          origin: 'template',
          missing: false,
          keys: [
            {
              key: 'PORT',
              follows: 'port',
              job: 'web',
              port: 'PORT',
              main: '3000',
              here: '3010',
              writes: '3020',
              withheld: false,
              status: 'in_step',
              out_of_step: true,
            },
          ],
        },
      ],
    },
    {
      dir: 'apps/docs',
      jobs: [],
      keys: 0,
      following: 0,
      to_check: 0,
      files: [{ target: 'apps/docs/.env', name: '.env', origin: 'none', missing: true, keys: [] }],
    },
  ],
  checks: [
    { file: 'apps/api/.env', key: 'MAILER_URL', kind: 'unlinked', job: 'stack', port: 'MAILPIT_UI' },
    { file: 'apps/web/.env', key: 'PORT', kind: 'out_of_step', job: 'web', port: 'PORT', here: '3010', writes: '3020' },
  ],
})
