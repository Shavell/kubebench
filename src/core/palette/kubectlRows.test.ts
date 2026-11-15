import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { parseKubectl } from './kubectl.ts'
import { kubectlNeeds, kubectlRows, verbRows, withLastWord, type KindInfo, type KubectlEnv, type PodTarget } from './kubectlRows.ts'

/** Records what the fake kinds and actions were asked to do. */
function fakeEnv(overrides: Partial<KubectlEnv> = {}) {
  const calls: string[] = []
  const kind = (k: string, names: string[], namespaced: boolean): KindInfo => ({
    kind: k,
    names,
    namespaced,
    openList: (ns) => calls.push(`list ${k} ${ns === null ? 'all' : (ns ?? 'global')}`),
    openDetail: (o) => calls.push(`detail ${k} ${o.namespace ?? '-'}/${o.name}`),
  })
  const target = (t: PodTarget) => `${t.namespace}/${t.pod}/${t.container}`
  const env: KubectlEnv = {
    kinds: [kind('Pod', ['pods', 'pod', 'po'], true), kind('Deployment', ['deployments', 'deployment', 'deploy'], true), kind('Node', ['nodes', 'node', 'no'], false)],
    namespaces: ['shop', 'default', 'kube-system'],
    pods: [
      { namespace: 'shop', name: 'web-1', containers: ['app', 'sidecar'] },
      { namespace: 'shop', name: 'web-2', containers: ['app'] },
      { namespace: 'default', name: 'db-0', containers: ['postgres'] },
    ],
    objects: [
      { namespace: 'shop', name: 'web-1' },
      { namespace: 'default', name: 'db-0' },
    ],
    clusters: [
      { name: 'prod', active: true },
      { name: 'stage', active: false },
      { name: 'bookmark', active: false, disabledReason: 'Bookmark' },
    ],
    loadError: null,
    currentNamespace: '',
    denied: () => null,
    t: (key, values) => (values ? `${key} ${JSON.stringify(values)}` : key),
    actions: {
      setNamespace: (ns) => calls.push(`ns ${ns || '*'}`),
      showLogs: (t) => calls.push(`logs ${target(t)}`),
      openShell: (t) => calls.push(`exec ${target(t)}`),
      switchCluster: (name) => calls.push(`ctx ${name}`),
    },
    ...overrides,
  }
  const rows = (input: string) => kubectlRows(parseKubectl(input)!, input, env)
  return { env, calls, rows }
}

describe('get', () => {
  it('offers kinds while the kind is typed, and opens the list', () => {
    const { rows, calls } = fakeEnv()
    const kinds = rows('get d')
    assert.deepEqual(kinds.map((r) => r.label), ['get deployments'])
    assert.equal(kinds[0].complete, 'get deployments ')
    kinds[0].run!()
    assert.deepEqual(calls, ['list Deployment global'])
  })

  it('opens a namespaced list with -n', () => {
    const { rows, calls } = fakeEnv()
    const [open] = rows('get po -n shop ')
    assert.equal(open.id, 'open-list')
    open.run!()
    assert.deepEqual(calls, ['list Pod shop'])
  })

  it('opens a list of all namespaces with -A, whatever the global filter', () => {
    const { rows, calls } = fakeEnv({ currentNamespace: 'shop' })
    rows('get pods -A')[0].run!()
    rows('get nodes -A')[0].run!()
    assert.deepEqual(calls, ['list Pod all', 'list Node global'])
  })

  it('completes namespace values', () => {
    const { rows } = fakeEnv()
    const options = rows('get pods -n s')
    assert.deepEqual(options.map((r) => r.label), ['shop'])
    assert.equal(options[0].complete, 'get pods -n shop ')
  })

  it('runs once the -n value names a namespace, without a trailing space', () => {
    const { rows, calls } = fakeEnv()
    assert.deepEqual(rows('get pods -n shop').map((r) => r.id), ['open-list'])
    rows('describe pod/web-1 -n shop')[0].run!()
    assert.equal(rows('exec web-1 -n sh')[0].id, 'ns:shop')
    rows('exec web-1 -n shop')[0].run!()
    assert.deepEqual(calls, ['detail Pod shop/web-1', 'exec shop/web-1/app'])
  })

  it('lists objects after the kind and opens one', () => {
    const { rows, calls } = fakeEnv()
    const list = rows('get pods ')
    assert.deepEqual(list.map((r) => r.id), ['open-list', 'obj:shop/web-1', 'obj:default/db-0'])
    rows('get pods we')[0].run!()
    assert.deepEqual(calls, ['detail Pod shop/web-1'])
  })

  it('names an unknown kind', () => {
    const { rows } = fakeEnv()
    assert.equal(rows('get widgets x')[0].id, 'unknown')
  })

  it('shows the reason when the cluster cannot be read', () => {
    const { rows } = fakeEnv({ denied: () => 'Viewers only' })
    const [row] = rows('get pods')
    assert.equal(row.disabledReason, 'Viewers only')
    assert.equal(row.run, undefined)
  })

  it('shows loading, then load errors', () => {
    assert.equal(fakeEnv({ objects: null }).rows('get pods web')[0].id, 'loading')
    assert.equal(fakeEnv({ objects: null, loadError: 'boom' }).rows('get pods web')[0].id, 'error')
  })
})

describe('describe', () => {
  it('completes kind/ and then the name', () => {
    const { rows, calls } = fakeEnv()
    assert.equal(rows('describe no')[0].complete, 'describe nodes/')
    const objects = rows('describe pod/db')
    assert.deepEqual(objects.map((r) => r.label), ['describe pod/db-0'])
    assert.equal(objects[0].complete, 'describe pod/db-0 -n default')
    objects[0].run!()
    assert.deepEqual(calls, ['detail Pod default/db-0'])
  })
})

describe('logs and exec', () => {
  it('shows logs of the first container by default', () => {
    const { rows, calls } = fakeEnv()
    const pods = rows('logs web')
    assert.deepEqual(pods.map((r) => r.id), ['pod:shop/web-1/app', 'pod:shop/web-2/app'])
    pods[0].run!()
    assert.deepEqual(calls, ['logs shop/web-1/app'])
  })

  it('uses -c and completes containers', () => {
    const { rows, calls } = fakeEnv()
    assert.deepEqual(rows('logs web-1 -c ').map((r) => r.label), ['logs shop/web-1', 'logs shop/web-1'])
    rows('exec web-1 -c sidecar ')[0].run!()
    assert.deepEqual(calls, ['exec shop/web-1/sidecar'])
  })

  it('keeps exec disabled without the exec permission, with the reason', () => {
    const { rows } = fakeEnv({ denied: (p) => (p === 'exec' ? 'No shell for viewers' : null) })
    const [row] = rows('exec web-1')
    assert.equal(row.disabledReason, 'No shell for viewers')
    assert.equal(row.run, undefined)
    assert.ok(fakeEnv({ denied: (p) => (p === 'exec' ? 'x' : null) }).rows('logs web-1')[0].run)
  })
})

describe('ns and ctx', () => {
  it('sets the namespace filter, or all namespaces', () => {
    const { rows, calls } = fakeEnv()
    const all = rows('ns ')
    assert.equal(all[0].id, 'ns:*')
    all[0].run!()
    rows('ns kube')[0].run!()
    assert.deepEqual(calls, ['ns *', 'ns kube-system'])
  })

  it('switches clusters, but not to the active one or a bookmark', () => {
    const { rows, calls } = fakeEnv()
    const clusters = rows('ctx ')
    assert.equal(clusters.find((r) => r.id === 'ctx:prod')?.run, undefined)
    assert.equal(clusters.find((r) => r.id === 'ctx:bookmark')?.disabledReason, 'Bookmark')
    clusters.find((r) => r.id === 'ctx:stage')!.run!()
    assert.deepEqual(calls, ['ctx stage'])
  })
})

describe('kubectlNeeds', () => {
  const { env } = fakeEnv()
  const needs = (input: string, ns = '') => kubectlNeeds(parseKubectl(input)!, env.kinds, ns)

  it('loads only what the command needs', () => {
    assert.deepEqual(needs('ns '), { namespaces: true, clusters: false })
    assert.deepEqual(needs('ctx'), { namespaces: false, clusters: true })
    assert.deepEqual(needs('logs web', 'shop').pods, { namespace: 'shop' })
    assert.deepEqual(needs('logs web -A', 'shop').pods, { namespace: '' })
    assert.equal(needs('get pods').objects, undefined)
    assert.equal(needs('get pods ', 'shop').objects?.namespace, 'shop')
    assert.equal(needs('get nodes ', 'shop').objects?.namespace, '')
    assert.equal(needs('get pods -n ').namespaces, true)
  })
})

describe('verbRows and withLastWord', () => {
  it('suggests verbs and aliases for a first word', () => {
    const rows = verbRows('l', [{ name: 'lg', expansion: 'logs web' }], (k) => k)
    assert.deepEqual(rows.map((r) => r.complete), ['logs ', 'lg '])
    assert.deepEqual(verbRows('', [], (k) => k), [])
  })

  it('replaces or appends the last word', () => {
    assert.equal(withLastWord('get pods -n sh', 'shop'), 'get pods -n shop')
    assert.equal(withLastWord('get pods -n ', 'shop'), 'get pods -n shop')
    assert.equal(withLastWord('get pods --namespace=sh', 'shop'), 'get pods --namespace=shop')
  })
})
