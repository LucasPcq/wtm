import type { APIRoute } from 'astro';
import { SITE_URL } from '../../site.config.mjs';
import { base } from '../lib/base.mjs';

export const GET: APIRoute = () => new Response(`User-agent: *\nAllow: /\n\nSitemap: ${new URL(`${base}sitemap-index.xml`, SITE_URL).href}\n`);
