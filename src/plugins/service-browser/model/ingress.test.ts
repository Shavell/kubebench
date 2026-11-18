import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { ingressRoutes, routeKey, startPath } from './ingress.ts'
import { targetAddress } from './ports.ts'

const ingress = {
  metadata: { name: 'shop', namespace: 'shop' },
  spec: {
    tls: [{ hosts: ['shop.example.com'], secretName: 'shop-tls' }],
    defaultBackend: { service: { name: 'fallback', port: { number: 8080 } } },
    rules: [
      {
        host: 'shop.example.com',
        http: {
          paths: [
            { path: '/', pathType: 'Prefix', backend: { service: { name: 'web', port: { number: 80 } } } },
            { path: '/admin', pathType: 'Prefix', backend: { service: { name: 'admin', port: { name: 'https' } } } },
            { path: '/assets', backend: { resource: { kind: 'Bucket', name: 'assets' } } },
          ],
        },
      },
      { host: '*.example.com', http: { paths: [{ path: '/', backend: { service: { name: 'web', port: { number: 80 } } } }] } },
      { host: 'api.example.com' },
    ],
  },
}

describe('ingressRoutes', () => {
  it('lists the Service routes of every rule, then the default backend', () => {
    assert.deepEqual(ingressRoutes(ingress), [
      { name: 'shop', host: 'shop.example.com', path: '/', tls: true, defaultBackend: false, service: 'web', port: '80', scheme: 'http' },
      { name: 'shop', host: 'shop.example.com', path: '/admin', tls: true, defaultBackend: false, service: 'admin', port: 'https', scheme: 'https' },
      { name: 'shop', host: '*.example.com', path: '/', tls: false, defaultBackend: false, service: 'web', port: '80', scheme: 'http' },
      { name: 'shop', host: '', path: '', tls: false, defaultBackend: true, service: 'fallback', port: '8080', scheme: 'http' },
    ])
  })

  it('tells a rule without host and path apart from the default backend', () => {
    const routes = ingressRoutes({
      metadata: { name: 'bare' },
      spec: {
        defaultBackend: { service: { name: 'fallback', port: { number: 80 } } },
        rules: [{ http: { paths: [{ pathType: 'ImplementationSpecific', backend: { service: { name: 'app', port: { number: 80 } } } }] } }],
      },
    })
    assert.deepEqual(routes.map((r) => [r.service, r.defaultBackend]), [['app', false], ['fallback', true]])
    assert.notEqual(routeKey(routes[0]), routeKey(routes[1]))
  })

  it('copes with an Ingress without spec', () => {
    assert.deepEqual(ingressRoutes({ metadata: { name: 'empty' } }), [])
  })

  it('keys routes by host and path', () => {
    assert.notEqual(routeKey({ host: 'a', path: '/', defaultBackend: false }), routeKey({ host: '', path: 'a /', defaultBackend: false }))
  })
})

describe('startPath', () => {
  it('starts at plain rule paths and at / otherwise', () => {
    assert.equal(startPath('/shop/cart'), '/shop/cart')
    assert.equal(startPath(''), '/')
    assert.equal(startPath('/api(/|$)(.*)'), '/')
  })
})

describe('targetAddress of an Ingress route', () => {
  const service = { kind: 'services' as const, namespace: 'shop', name: 'web', port: '80', scheme: 'http' as const }
  it('names the rule host, https when the Ingress terminates TLS for it', () => {
    assert.equal(targetAddress({ ...service, ingress: { name: 'shop', host: 'shop.example.com', path: '/', tls: true, defaultBackend: false } }), 'https://shop.example.com')
    assert.equal(targetAddress({ ...service, ingress: { name: 'shop', host: 'shop.example.com', path: '/', tls: false, defaultBackend: false } }), 'http://shop.example.com')
  })
  it('names the Service for the default backend and wildcard hosts', () => {
    assert.equal(targetAddress({ ...service, ingress: { name: 'shop', host: '', path: '', tls: false, defaultBackend: true } }), 'http://web.shop:80')
    assert.equal(targetAddress({ ...service, ingress: { name: 'shop', host: '*.example.com', path: '/', tls: false, defaultBackend: false } }), 'http://web.shop:80')
  })
})
