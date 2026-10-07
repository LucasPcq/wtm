// Copies the repository's Markdown into Starlight's content collection without
// moving it: docs/ stays the source of truth that GitHub renders, and this script
// only adds front matter and turns repository-relative links into site routes.
import { cpSync, existsSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, normalize, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const SITE = dirname(dirname(fileURLToPath(import.meta.url)));
const REPO = dirname(SITE);
const OUT = join(SITE, 'src/content/docs');
const BASE = process.env.DOCS_BASE ?? '/wtm';
const REF = process.env.DOCS_REF ?? 'main';
const GITHUB = `https://github.com/LucasPcq/wtm/blob/${REF}`;

const pages = new Map();

const add = (source, route) => pages.set(normalize(source), route);

add('README.md', '');
add('CHANGELOG.md', 'changelog');
for (const f of readdirSync(join(REPO, 'docs/guide'))) {
  if (f.endsWith('.md')) add(`docs/guide/${f}`, f === 'README.md' ? 'guide' : `guide/${f.slice(0, -3).replaceAll('.', '')}`);
}
for (const f of readdirSync(join(REPO, 'docs'))) {
  if (/^wtm.*\.md$/.test(f)) add(`docs/${f}`, `reference/${f.slice(0, -3).replaceAll('_', '-')}`);
}
for (const f of readdirSync(join(REPO, 'docs/dev'))) {
  if (f.endsWith('.md')) add(`docs/dev/${f}`, f === 'README.md' ? 'dev' : `dev/${f.slice(0, -3)}`);
}

const guideOrder = [...readFileSync(join(REPO, 'docs/guide/README.md'), 'utf8').matchAll(/\]\(([\w-]+\.md)\)/g)].map((m) => m[1]);

const rewriteLink = (source, target) => {
  if (/^([a-z]+:|#|\/)/.test(target)) return target;
  const [path, hash = ''] = target.split('#');
  const resolved = normalize(join(dirname(source), path));
  const fragment = hash ? `#${hash}` : '';
  if (pages.has(resolved)) return `${BASE}/${pages.get(resolved)}${pages.get(resolved) ? '/' : ''}${fragment}`;
  if (resolved.startsWith('docs/assets/')) return `${BASE}/${relative('docs', resolved)}`;
  return `${GITHUB}/${resolved}${fragment}`;
};

const rewriteBody = (source, body) =>
  body
    .replace(/(\]\()([^)\s]+)(\))/g, (_, open, target, close) => open + rewriteLink(source, target) + close)
    .replace(/(src|href)="([^"]+)"/g, (_, attr, target) => `${attr}="${rewriteLink(source, target)}"`);

const splitTitle = (text) => {
  const match = text.match(/^#{1,3} (.+)\n/m);
  if (!match) return { title: 'wtm', body: text };
  return { title: match[1].replaceAll('`', ''), body: text.replace(match[0], '') };
};

const frontMatter = (fields) =>
  `---\n${Object.entries(fields)
    .filter(([, v]) => v !== undefined)
    .map(([k, v]) => `${k}: ${typeof v === 'object' ? `\n${Object.entries(v).map(([kk, vv]) => `  ${kk}: ${JSON.stringify(vv)}`).join('\n')}` : JSON.stringify(v)}`)
    .join('\n')}\n---\n\n`;

const landing = (text) => {
  const body = text.slice(text.indexOf('## Why wtm'));
  return (
    frontMatter({ title: 'wtm', description: 'One branch, one worktree, one isolated dev stack.', template: 'splash' }).replace(
      '---\n\n',
      `hero:\n  tagline: A worktree manager for teams that work on several branches at once, and let their agents do too.\n  image:\n    file: ../../../../docs/assets/hero.gif\n  actions:\n    - text: Get started\n      link: ${BASE}/guide/getting-started/\n      icon: right-arrow\n    - text: GitHub\n      link: https://github.com/LucasPcq/wtm\n      icon: github\n      variant: minimal\n---\n\n`,
    ) + rewriteBody('README.md', body)
  );
};

rmSync(OUT, { recursive: true, force: true });
for (const [source, route] of pages) {
  const text = readFileSync(join(REPO, source), 'utf8');
  const file = join(OUT, route === '' ? 'index.md' : `${route}${['guide', 'dev'].includes(route) ? '/index' : ''}.md`);
  mkdirSync(dirname(file), { recursive: true });
  if (source === 'README.md') {
    writeFileSync(file, landing(text));
    continue;
  }
  const { title, body } = splitTitle(text);
  const order = source.startsWith('docs/guide/') ? guideOrder.indexOf(source.slice('docs/guide/'.length)) : -1;
  writeFileSync(file, frontMatter({ title, editUrl: `https://github.com/LucasPcq/wtm/edit/main/${source}`, sidebar: order >= 0 ? { order: order + 1 } : ['guide', 'dev'].includes(route) ? { hidden: true } : undefined }) + rewriteBody(source, body));
}

const assets = join(REPO, 'docs/assets');
if (existsSync(assets)) cpSync(assets, join(SITE, 'public/assets'), { recursive: true });

console.log(`synced ${pages.size} pages into ${relative(REPO, OUT)}`);
