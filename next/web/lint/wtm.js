// The web front's own lint rules, an oxlint JS plugin (.oxlintrc.json): what the built-in rules cannot say. Each
// import is resolved, `@/` and relative alike, to the layer it lands in, so no spelling and no depth gets past.
import path from 'node:path'

// A file's place in the app: the layer under src/ it belongs to, and whether it is a test or the test kit.
function placeOf(filename) {
  const at = filename.lastIndexOf(`${path.sep}src${path.sep}`)
  if (at < 0) return undefined
  const rel = filename
    .slice(at + 5)
    .split(path.sep)
    .join('/')
  return {
    rel,
    layer: layerOf(rel),
    test: /\.test\.tsx?$/.test(rel),
    component: /\.test\.tsx$/.test(rel) || rel.startsWith('test/'),
  }
}

// layerOf names the layer of a path under src/: features keep their name, every other folder is one layer.
function layerOf(rel) {
  const [first, second] = rel.split('/')
  if (first === 'features') return `features/${second}`
  if (first === 'components' && second === 'ui') return 'components/ui'
  if (!rel.includes('/')) return rel.replace(/\.[^.]+$/, '')
  return first
}

// targetOf is what an import of spec from the file at rel reaches: a layer of the app, or a package by its name.
function targetOf(rel, spec) {
  if (spec.startsWith('@/')) return { layer: layerOf(spec.slice(2)) }
  if (spec.startsWith('.')) {
    const resolved = path.posix.normalize(path.posix.join(path.posix.dirname(rel), spec))
    return resolved.startsWith('..') ? { outside: true } : { layer: layerOf(resolved) }
  }
  const parts = spec.split('/')
  return { pkg: spec.startsWith('@') ? parts.slice(0, 2).join('/') : parts[0] }
}

const testing = ['vitest', 'msw', '@testing-library/react', '@testing-library/user-event', '@testing-library/dom']

// May the file at place import target? Each layer is an allowlist: what it may reach, nothing else.
function refusal(place, target, typeOnly) {
  if (target.outside) return 'reaches out of src/'
  if (target.pkg && testing.includes(target.pkg) && !place.test && place.layer !== 'test')
    return `${target.pkg} is for tests only`
  if (target.pkg?.startsWith('eventsource-parser') && place.layer !== 'api')
    return 'streams are parsed in src/api/ only'
  if (place.layer === 'test' || place.layer === 'main') return undefined
  // a test reaches what its file reaches, and the test kit
  if (place.test && (target.layer === 'test' || testing.includes(target.pkg))) return undefined

  const own = place.layer
  switch (true) {
    case own === 'api':
      return target.layer && target.layer !== 'api'
        ? 'src/api/ is the bottom layer: it imports nothing of the app'
        : undefined
    case own === 'lib':
      if (target.layer) return target.layer === 'lib' ? undefined : 'lib/ is pure helpers: nothing of the app'
      return ['cn', 'clsx', 'tailwind-merge'].includes(target.pkg)
        ? undefined
        : `lib/ is pure helpers: not ${target.pkg}`
    case own === 'routes':
      if (target.layer)
        return target.layer.startsWith('features/') || target.layer === 'routes'
          ? undefined
          : 'a route is a path and a feature: it imports features only'
      if (target.pkg === '@tanstack/react-router') return undefined
      if (target.pkg === '@tanstack/react-query' && typeOnly) return undefined
      return `a route is a path and a feature: not ${target.pkg}`
    case own === 'components':
      return target.layer?.startsWith('features/') || target.layer === 'routes'
        ? 'components/ holds what features share: it never reaches into a feature'
        : undefined
    case own.startsWith('features/'):
      if (target.layer?.startsWith('features/') && target.layer !== own)
        return 'a feature reaches another only once the code moved to components/ or lib/'
      return target.layer === 'routes' ? 'a feature does not import routes' : undefined
    default:
      return undefined
  }
}

const layers = {
  meta: {
    type: 'problem',
    docs: { description: 'Each layer of the web front imports only what its allowlist names.' },
  },
  create(context) {
    const place = placeOf(context.filename)
    if (!place) return {}
    const check = (node, spec, typeOnly) => {
      if (typeof spec !== 'string') return
      const why = refusal(place, targetOf(place.rel, spec), typeOnly)
      if (why) context.report({ node, message: `${spec}: ${why} (docs/dev/web-front.md)` })
    }
    return {
      ImportDeclaration: (node) => check(node, node.source.value, node.importKind === 'type'),
      ExportNamedDeclaration: (node) => node.source && check(node, node.source.value, node.exportKind === 'type'),
      ExportAllDeclaration: (node) => check(node, node.source.value, node.exportKind === 'type'),
      ImportExpression: (node) => node.source.type === 'Literal' && check(node, node.source.value, false),
    }
  },
}

const effectNames = ['useEffect', 'useLayoutEffect', 'useInsertionEffect']
const noEffect =
  'No effects: derive while rendering, put data and side effects in TanStack Query/Router/Form, a DOM hook in a ref callback (docs/dev/web-front.md).'

const effects = {
  meta: { type: 'problem', docs: { description: 'No React effect, by any spelling.' } },
  create(context) {
    const reacts = new Set()
    return {
      ImportDeclaration(node) {
        if (node.source.value !== 'react') return
        for (const spec of node.specifiers) {
          if (spec.type !== 'ImportSpecifier') reacts.add(spec.local.name)
          else if (effectNames.includes(spec.imported.name ?? spec.imported.value))
            context.report({ node: spec, message: noEffect })
        }
      },
      MemberExpression(node) {
        if (
          node.object.type === 'Identifier' &&
          (reacts.has(node.object.name) || node.object.name === 'React') &&
          effectNames.includes(node.property.name)
        )
          context.report({ node, message: noEffect })
      },
    }
  },
}

const reachers = ['fetch', 'EventSource', 'XMLHttpRequest', 'WebSocket']
const globals = ['window', 'globalThis', 'self']
const noReach =
  'The server is reached through src/api/ only: read with useQuery, write with useMutation over api/queries/.'

const network = {
  meta: { type: 'problem', docs: { description: 'Nothing but src/api/ reaches the network, by any global.' } },
  create(context) {
    const place = placeOf(context.filename)
    if (!place || place.layer === 'api') return {}
    return {
      MemberExpression(node) {
        const name = node.property.name
        const on = node.object.type === 'Identifier' ? node.object.name : node.object.property?.name
        if ((globals.includes(on) && reachers.includes(name)) || (on === 'navigator' && name === 'sendBeacon'))
          context.report({ node, message: noReach })
      },
    }
  },
}

const testDoubles = {
  meta: {
    type: 'problem',
    docs: { description: 'A test uses the app as its user does: no hook alone, no module mocked.' },
  },
  create(context) {
    const place = placeOf(context.filename)
    if (!place || !(place.test || place.layer === 'test')) return {}
    const modules = new Set()
    return {
      ImportDeclaration(node) {
        for (const spec of node.specifiers) {
          if (spec.type === 'ImportNamespaceSpecifier') modules.add(spec.local.name)
          if (
            node.source.value === '@testing-library/react' &&
            spec.type === 'ImportSpecifier' &&
            spec.imported.name === 'renderHook'
          )
            context.report({
              node: spec,
              message: 'Test a component through what its user does, never a hook alone (docs/dev/web-front.md).',
            })
        }
      },
      CallExpression(node) {
        const callee = node.callee
        if (callee.type !== 'MemberExpression' || callee.object.name !== 'vi') return
        const method = callee.property.name
        if (place.component && (method === 'mock' || method === 'doMock'))
          context.report({
            node,
            message: `vi.${method}: a component test mocks the server with msw (src/test/server.ts), never a module of the app.`,
          })
        const [object] = node.arguments
        if (method === 'spyOn' && object?.type === 'Identifier' && modules.has(object.name))
          context.report({
            node,
            message: 'vi.spyOn on a module of the app replaces it: mock the server with msw instead.',
          })
      },
    }
  },
}

export default {
  meta: { name: 'wtm' },
  rules: { layers, effects, network, 'test-doubles': testDoubles },
}
