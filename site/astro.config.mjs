import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightLinksValidator from 'starlight-links-validator';
import starlightLlmsTxt from 'starlight-llms-txt';

export default defineConfig({
  site: 'https://lucaspcq.github.io',
  base: process.env.DOCS_BASE ?? '/wtm',
  markdown: { smartypants: false },
  integrations: [
    starlight({
      title: 'wtm',
      description: 'One branch, one worktree, one isolated dev stack.',
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/LucasPcq/wtm' }],
      editLink: { baseUrl: 'https://github.com/LucasPcq/wtm/edit/main/' },
      lastUpdated: false,
      plugins: [starlightLinksValidator({ errorOnLocalLinks: false }), starlightLlmsTxt()],
      sidebar: [
        { label: 'Guide', items: [{ autogenerate: { directory: 'guide' } }] },
        { label: 'Command reference', collapsed: true, items: [{ autogenerate: { directory: 'reference' } }] },
        { label: 'Changelog', link: '/changelog/' },
        { label: 'Contributing', collapsed: true, items: [{ autogenerate: { directory: 'dev' } }] },
      ],
    }),
  ],
});
