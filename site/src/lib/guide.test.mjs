import { test } from 'node:test';
import assert from 'node:assert/strict';

import { guideSections } from './guide.mjs';

const readme = `# wtm user guide

Intro with a [command reference](../wtm.md).

## Start here

- **[Getting started](getting-started.md)**: ten minutes.

## Dev stacks with \`wtm run\`

Opt-in.

- **[How \`wtm run\` works](how-run-works.md)**: ports.
- **[\`run.toml\` reference](run-toml.md)**: every key.

## Upgrading

- **[Migrating to 0.29](migrating-to-0.29.md)**: envelopes.
`;

test('each section of the index is a sidebar group, its links the pages in their order', () => {
  assert.deepEqual(guideSections(readme), [
    { label: 'Start here', collapsed: false, items: [{ slug: 'guide/getting-started', label: 'Getting started' }] },
    {
      label: 'Dev stacks with wtm run',
      collapsed: false,
      items: [
        { slug: 'guide/how-run-works', label: 'How wtm run works' },
        { slug: 'guide/run-toml', label: 'run.toml reference' },
      ],
    },
    { label: 'Upgrading', collapsed: true, items: [{ slug: 'guide/migrating-to-029', label: 'Migrating to 0.29' }] },
  ]);
});

test('a heading without links gives no group', () => {
  assert.deepEqual(guideSections('# t\n\n## Empty\n\nNo pages yet.\n'), []);
});
