import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { browserKind, guessScheme, normalizePath, parsePortList, podPorts, servicePorts, targetAddress } from './ports.ts'

describe('servicePorts', () => {
  it('keeps TCP ports with their names and guesses the scheme', () => {
    const service = {
      spec: {
        ports: [
          { name: 'http', port: 80, protocol: 'TCP' },
          { name: 'dns', port: 53, protocol: 'UDP' },
          { name: 'metrics', port: 9090 },
          { name: 'web', port: 8443, appProtocol: 'https' },
        ],
      },
    }
    assert.deepEqual(servicePorts(service), [
      { port: 80, name: 'http', scheme: 'http' },
      { port: 9090, name: 'metrics', scheme: 'http' },
      { port: 8443, name: 'web', scheme: 'https' },
    ])
  })

  it('copes with objects without ports', () => {
    assert.deepEqual(servicePorts({}), [])
    assert.deepEqual(servicePorts({ spec: { ports: 'bogus' } }), [])
  })
})

describe('podPorts', () => {
  it('lists the container ports once each', () => {
    const pod = {
      spec: {
        containers: [
          { ports: [{ containerPort: 8080, name: 'http' }] },
          { ports: [{ containerPort: 8080 }, { containerPort: 9443, name: 'webhook-tls' }] },
          {},
        ],
      },
    }
    assert.deepEqual(podPorts(pod), [
      { port: 8080, name: 'http', scheme: 'http' },
      { port: 9443, name: 'webhook-tls', scheme: 'https' },
    ])
  })
})

describe('parsePortList', () => {
  it('reads kubectl-style lists, TCP only', () => {
    assert.deepEqual(parsePortList('80:31204/TCP, 53/UDP,443/TCP'), [
      { port: 80, scheme: 'http' },
      { port: 443, scheme: 'https' },
    ])
  })
})

describe('helpers', () => {
  it('guesses https from names and well-known ports', () => {
    assert.equal(guessScheme(80, 'https-web'), 'https')
    assert.equal(guessScheme(6443), 'https')
    assert.equal(guessScheme(3000, 'grafana'), 'http')
  })

  it('formats the in-cluster address', () => {
    assert.equal(
      targetAddress({ kind: 'services', namespace: 'shop', name: 'web', port: '80', scheme: 'http' }),
      'http://web.shop:80',
    )
    assert.equal(targetAddress({ kind: 'pods', namespace: 'shop', name: 'web-1', port: '8443', scheme: 'https' }), 'https://web-1:8443')
  })

  it('maps kinds and paths', () => {
    assert.equal(browserKind('Service'), 'services')
    assert.equal(browserKind('Ingress'), 'ingresses')
    assert.equal(browserKind('ConfigMap'), null)
    assert.equal(normalizePath(' api/x?y=1 '), '/api/x?y=1')
    assert.equal(normalizePath('/'), '/')
  })
})
