import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightLinksValidator from 'starlight-links-validator';
import starlightLlmsTxt from 'starlight-llms-txt';
import starlightVersions from 'starlight-versions';

import { archivedVersions, BASE, currentLabel, currentSidebar, SITE_URL } from './site.config.mjs';

const archivedPages = archivedVersions.map(({ slug }) => `${slug}/**`);
const root = `${BASE}/`;
const socialImage = new URL(`${root}social-preview.png`, SITE_URL).href;

export default defineConfig({
  site: SITE_URL,
  base: BASE || '/',
  markdown: { smartypants: false },
  integrations: [
    starlight({
      title: 'wtm',
      description: 'Every branch, running. A git worktree manager that gives each branch its own ports, Compose project, database and URL.',
      logo: { src: './src/assets/logo.svg' },
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/LucasPcq/wtm' }],
      editLink: { baseUrl: 'https://github.com/LucasPcq/wtm/edit/main/' },
      customCss: ['./src/styles/theme.css'],
      // Shell blocks get no window dots: docs/dev/identity.md, Showing the product.
      expressiveCode: { defaultProps: { frame: 'code' } },
      head: [
        { tag: 'meta', attrs: { property: 'og:image', content: socialImage } },
        { tag: 'meta', attrs: { property: 'og:image:width', content: '1280' } },
        { tag: 'meta', attrs: { property: 'og:image:height', content: '640' } },
        { tag: 'meta', attrs: { name: 'twitter:image', content: socialImage } },
        { tag: 'link', attrs: { rel: 'apple-touch-icon', href: `${root}apple-touch-icon.png` } },
        { tag: 'link', attrs: { rel: 'preconnect', href: 'https://fonts.googleapis.com' } },
        { tag: 'link', attrs: { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: true } },
        { tag: 'link', attrs: { rel: 'stylesheet', href: 'https://fonts.googleapis.com/css2?family=Archivo:wdth,wght@62..125,300..900&family=Martian+Mono:wght@300..600&display=swap' } },
      ],
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
