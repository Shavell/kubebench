import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { aliasError, expandAlias, parseKubectl, splitKindName } from './kubectl.ts'

describe('parseKubectl', () => {
  it('ignores input that is not a command', () => {
    assert.equal(parseKubectl(''), null)
    assert.equal(parseKubectl('View: Toggle Panel'), null)
    assert.equal(parseKubectl('getaway'), null)
  })

  it('parses verbs, arguments and flags', () => {
    assert.deepEqual(parseKubectl('get pods -n shop'), {
      verb: 'get',
      args: ['pods'],
      namespace: 'shop',
      allNamespaces: false,
      trailingSpace: false,
      pendingFlag: 'namespace',
    })
    const logs = parseKubectl('logs web-1 --namespace=shop -c app ')
    assert.equal(logs?.verb, 'logs')
    assert.deepEqual(logs?.args, ['web-1'])
    assert.equal(logs?.namespace, 'shop')
    assert.equal(logs?.container, 'app')
    assert.equal(logs?.pendingFlag, undefined)
    assert.equal(logs?.trailingSpace, true)
    assert.equal(parseKubectl('get deploy -A')?.allNamespaces, true)
  })

  it('strips a leading kubectl or k', () => {
    assert.deepEqual(parseKubectl('kubectl get svc')?.args, ['svc'])
    assert.equal(parseKubectl('k ns kube-system')?.verb, 'ns')
  })

  it('reports the flag whose value is being typed', () => {
    assert.equal(parseKubectl('get pods -n')?.pendingFlag, 'namespace')
    assert.equal(parseKubectl('get pods -n ')?.pendingFlag, 'namespace')
    assert.equal(parseKubectl('get pods -n sh')?.namespace, 'sh')
    assert.equal(parseKubectl('get pods -n sh')?.pendingFlag, 'namespace')
    assert.equal(parseKubectl('get pods -n shop ')?.pendingFlag, undefined)
    assert.equal(parseKubectl('exec web -c ')?.pendingFlag, 'container')
  })
})

describe('splitKindName', () => {
  it('splits kind/name', () => {
    assert.deepEqual(splitKindName('pod/web-1'), { kind: 'pod', name: 'web-1' })
    assert.deepEqual(splitKindName('pod/'), { kind: 'pod', name: '' })
    assert.deepEqual(splitKindName('deploy'), { kind: 'deploy' })
  })
})

describe('aliases', () => {
  const aliases = [
    { name: 'kgp', expansion: 'get pods' },
    { name: 'kks', expansion: 'ns kube-system' },
  ]

  it('expands the first word and keeps the rest', () => {
    assert.equal(expandAlias('kgp -n shop', aliases), 'get pods -n shop')
    assert.equal(expandAlias('kks', aliases), 'ns kube-system')
    assert.equal(expandAlias('get kgp', aliases), 'get kgp')
    assert.equal(expandAlias('kgpx', aliases), 'kgpx')
  })

  it('expands one level only', () => {
    assert.equal(expandAlias('a', [{ name: 'a', expansion: 'b' }, { name: 'b', expansion: 'get pods' }]), 'b')
  })

  it('validates new aliases', () => {
    const check = (name: string, expansion: string) => aliasError({ name, expansion }, aliases)
    assert.equal(check('kgd', 'get deploy'), null)
    assert.equal(check('', 'get pods'), 'palette.aliases.errors.nameRequired')
    assert.equal(check('k g', 'get pods'), 'palette.aliases.errors.nameSpaces')
    assert.equal(check('logs', 'get pods'), 'palette.aliases.errors.nameReserved')
    assert.equal(check('kubectl', 'get pods'), 'palette.aliases.errors.nameReserved')
    assert.equal(check('kgp', 'get pods'), 'palette.aliases.errors.nameTaken')
    assert.equal(check('x', ' '), 'palette.aliases.errors.expansionRequired')
    assert.equal(check('x', 'rm -rf /'), 'palette.aliases.errors.expansionInvalid')
  })
})
