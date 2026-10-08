import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightLinksValidator from 'starlight-links-validator';
import starlightLlmsTxt from 'starlight-llms-txt';
import starlightVersions from 'starlight-versions';

import { archivedVersions, BASE, currentLabel, currentSidebar, SITE_URL } from './site.config.mjs';

const archivedPages = archivedVersions.map(({ slug }) => `${slug}/**`);

export default defineConfig({
  site: SITE_URL,
  base: BASE || '/',
  markdown: { smartypants: false },
  integrations: [
    starlight({
      title: 'wtm',
      description: 'One branch, one worktree, one isolated dev stack. A git worktree manager for teams and AI agents.',
      logo: { src: './src/assets/logo.svg' },
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/LucasPcq/wtm' }],
      editLink: { baseUrl: 'https://github.com/LucasPcq/wtm/edit/main/' },
      customCss: ['./src/styles/theme.css'],
      lastUpdated: false,
      plugins: [
        starlightVersions({
          current: { label: currentLabel },
          versions: archivedVersions.map(({ slug, label }) => ({ slug, label })),
          exclude: ['docs.mdx', 'changelog.md'],
        }),
        starlightLinksValidator({
          errorOnLocalLinks: false,
          // An archived version is a released tag: a stale anchor there can no longer be fixed.
          exclude: ({ file, link }) => link.startsWith("#") && archivedVersions.some((v) => file.includes(`/src/content/docs/${v.slug}/`)),
        }),
        starlightLlmsTxt({
          projectName: 'wtm',
          promote: ['guide/getting-started', 'guide/**'],
          demote: archivedPages,
          exclude: archivedPages,
        }),
      ],
      sidebar: currentSidebar,
    }),
  ],
});
