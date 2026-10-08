// Builds Starlight's content from the repository's Markdown without moving it:
// docs/ stays the source of truth that GitHub renders, and this script only adds
// front matter and turns repository-relative links into site routes. Archived
// versions are read from their release tag, so no copy of an old doc is committed.
import { execFileSync } from 'node:child_process';
import { cpSync, existsSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, normalize, posix } from 'node:path';

import { archivedVersions, BASE, DOCS_REF, DOMAIN, GENERATED_DIRS, REPO, SITE } from '../site.config.mjs';

const CONTENT = join(SITE, 'src/content/docs');
const VERSIONS = join(SITE, 'src/content/versions');

const git = (...args) => execFileSync('git', ['-C', REPO, ...args], { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 });

const fromWorktree = {
  list: (dir) => (existsSync(join(REPO, dir)) ? readdirSync(join(REPO, dir)) : []),
  read: (file) => readFileSync(join(REPO, file), 'utf8'),
};

const fromTag = (tag) => ({
  list: (dir) => {
    try {
      return git('ls-tree', '--name-only', `${tag}:${dir}`).split('\n').filter(Boolean);
    } catch {
      return [];
    }
  },
  read: (file) => git('show', `${tag}:${file}`),
});

const routeOf = (file) => {
  if (file === 'README.md') return '';
  if (file === 'CHANGELOG.md') return 'changelog';
  if (file === 'docs/guide/README.md') return 'guide';
  if (file === 'docs/dev/README.md') return 'dev';
  const name = posix.basename(file, '.md');
  if (/^docs\/guide\/[^/]+\.md$/.test(file)) return `guide/${name.replaceAll('.', '')}`;
  if (/^docs\/dev\/[^/]+\.md$/.test(file)) return `dev/${name.replaceAll('.', '')}`;
  if (/^docs\/wtm[^/]*\.md$/.test(file)) return `reference/${name.replaceAll('_', '-')}`;
  return undefined;
};

const sourcesOf = (source) =>
  [
    ...source.list('docs/guide').map((f) => `docs/guide/${f}`),
    ...source.list('docs').map((f) => `docs/${f}`),
    ...source.list('docs/dev').map((f) => `docs/dev/${f}`),
  ].filter((f) => f.endsWith('.md') && routeOf(f) !== undefined);

// The landing page and the changelog exist once, outside versioning: a link to
// them from an archived page goes to the current one.
const rewriteLink = ({ file, target, prefix, ref }) => {
  if (/^([a-z]+:|#|\/)/.test(target)) return target;
  const [path, hash] = target.split('#');
  const resolved = normalize(posix.join(posix.dirname(file), path));
  const fragment = hash ? `#${hash}` : '';
  const route = routeOf(resolved);
  if (route === '') return `${BASE}/${fragment}`;
  if (route === 'changelog') return `${BASE}/changelog/${fragment}`;
  if (route !== undefined) return `${BASE}/${prefix}${route}/${fragment}`;
  if (resolved.startsWith('docs/assets/')) return `${BASE}/${resolved.slice('docs/'.length)}`;
  return `https://github.com/LucasPcq/wtm/blob/${ref}/${resolved}${fragment}`;
};

const rewriteBody = ({ file, body, prefix, ref }) =>
  body
    .replace(/(\]\()([^)\s]+)(\))/g, (_, open, target, close) => open + rewriteLink({ file, target, prefix, ref }) + close)
    .replace(/(src|href)="([^"]+)"/g, (_, attr, target) => `${attr}="${rewriteLink({ file, target, prefix, ref })}"`);

const splitTitle = (text) => {
  const match = text.match(/^#{1,3} (.+)\n/m);
  if (!match) return { title: 'wtm', body: text };
  return { title: match[1].replaceAll('`', ''), body: text.replace(match[0], '') };
};

const yaml = (value, indent = '') =>
  Object.entries(value)
    .filter(([, v]) => v !== undefined)
    .map(([k, v]) => (typeof v === 'object' ? `${indent}${k}:\n${yaml(v, `${indent}  `)}` : `${indent}${k}: ${JSON.stringify(v)}`))
    .join('\n');

const writePage = ({ out, route, isIndex, fields, body }) => {
  const file = join(out, isIndex ? `${route}/index.md` : `${route}.md`);
  mkdirSync(dirname(file), { recursive: true });
  writeFileSync(file, `---\n${yaml(fields)}\n---\n\n${body}`);
};

const syncTree = ({ source, prefix, ref, editable }) => {
  const guideOrder = [...source.read('docs/guide/README.md').matchAll(/\]\(([\w.-]+\.md)\)/g)].map((m) => m[1]);
  const files = sourcesOf(source);
  for (const file of files) {
    const route = routeOf(file);
    const { title, body } = splitTitle(source.read(file));
    const isIndex = route === 'guide' || route === 'dev';
    const order = file.startsWith('docs/guide/') ? guideOrder.indexOf(posix.basename(file)) : -1;
    writePage({
      out: join(CONTENT, prefix),
      route,
      isIndex,
      fields: {
        title,
        editUrl: editable ? `https://github.com/LucasPcq/wtm/edit/main/${file}` : false,
        sidebar: isIndex ? { hidden: true } : order >= 0 ? { order: order + 1 } : undefined,
      },
      body: rewriteBody({ file, body, prefix, ref }),
    });
  }
  return files.length;
};

for (const dir of [...GENERATED_DIRS, ...archivedVersions.map((v) => v.slug)]) rmSync(join(CONTENT, dir), { recursive: true, force: true });
rmSync(join(CONTENT, 'changelog.md'), { force: true });
rmSync(join(CONTENT, 'docs.mdx'), { force: true });
rmSync(VERSIONS, { recursive: true, force: true });

let pages = syncTree({ source: fromWorktree, prefix: '', ref: DOCS_REF, editable: true });

const changelog = splitTitle(fromWorktree.read('CHANGELOG.md'));
writePage({
  out: CONTENT,
  route: 'changelog',
  isIndex: false,
  fields: { title: changelog.title, editUrl: 'https://github.com/LucasPcq/wtm/edit/main/CHANGELOG.md' },
  body: rewriteBody({ file: 'CHANGELOG.md', body: changelog.body, prefix: '', ref: DOCS_REF }),
});
pages += 1;

mkdirSync(VERSIONS, { recursive: true });
for (const version of archivedVersions) {
  pages += syncTree({ source: fromTag(version.tag), prefix: `${version.slug}/`, ref: version.tag, editable: false });
  writeFileSync(join(VERSIONS, `${version.slug}.json`), JSON.stringify({ sidebar: version.sidebar, excluded: ['changelog'] }, null, 2));
}

cpSync(join(REPO, 'docs/assets'), join(SITE, 'public/assets'), { recursive: true });
mkdirSync(join(SITE, 'public/schemas'), { recursive: true });
for (const f of readdirSync(join(REPO, 'internal/schemas')).filter((f) => f.endsWith('.json'))) {
  cpSync(join(REPO, 'internal/schemas', f), join(SITE, 'public/schemas', f));
}

const home = readFileSync(join(SITE, 'src/docs-home.mdx'), 'utf8').replaceAll('__BASE__', BASE);
writeFileSync(join(CONTENT, 'docs.mdx'), home);

rmSync(join(SITE, 'public/CNAME'), { force: true });
if (DOMAIN) writeFileSync(join(SITE, 'public/CNAME'), `${DOMAIN}\n`);

console.log(`synced ${pages} pages (current + ${archivedVersions.map((v) => v.slug).join(', ') || 'no archived version'})`);
