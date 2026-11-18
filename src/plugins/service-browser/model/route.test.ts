import assert from 'node:assert/strict'
import { test } from 'node:test'
import { routeOf, routePublicUrl } from './route.ts'

const route = (spec: Record<string, unknown>) => ({ metadata: { name: 'web' }, spec })

test('an edge Route to a named port', () => {
  const r = routeOf(route({ host: 'web.apps.example.com', path: '/app', to: { kind: 'Service', name: 'web' }, port: { targetPort: 'http' }, tls: { termination: 'edge' } }))
  assert.deepEqual(r, {
    name: 'web', host: 'web.apps.example.com', path: '/app', tls: true, termination: 'edge',
    service: 'web', targetPort: 'http', scheme: 'http',
  })
  assert.equal(routePublicUrl(r!), 'https://web.apps.example.com/app')
})

test('passthrough and re-encrypt Routes reach TLS backends', () => {
  for (const termination of ['passthrough', 'Reencrypt']) {
    const r = routeOf(route({ host: 'h', to: { kind: 'Service', name: 'web' }, port: { targetPort: 8080 }, tls: { termination } }))
    assert.equal(r?.scheme, 'https')
    assert.equal(r?.targetPort, '8080')
  }
})

test('a plain Route without a port', () => {
  const r = routeOf(route({ host: 'h.example.com', to: { kind: 'Service', name: 'web' } }))
  assert.equal(r?.targetPort, '')
  assert.equal(r?.tls, false)
  assert.equal(routePublicUrl(r!), 'http://h.example.com')
})

test('a Route without a Service cannot be browsed', () => {
  assert.equal(routeOf(route({ host: 'h', to: { kind: 'Deployment', name: 'web' } })), null)
  assert.equal(routeOf(null), null)
})
