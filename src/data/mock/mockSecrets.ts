import { mulberry32 } from './rng'

/**
 * Values the seeded mock Secrets reveal (design §7.2). Deterministic: each
 * value is drawn from an RNG seeded by the Secret and key, so a reveal
 * shows the same demo value every time.
 */

function seedOf(text: string): number {
  let h = 0x811c9dc5
  for (const ch of text) h = Math.imul(h ^ ch.charCodeAt(0), 0x01000193) >>> 0
  return h
}

const ALPHANUMERIC = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789'
const BASE64 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'

function draw(rand: () => number, alphabet: string, length: number): string {
  let out = ''
  for (let i = 0; i < length; i++) out += alphabet[Math.floor(rand() * alphabet.length)]
  return out
}

function pem(rand: () => number, label: string): string {
  const lines = Array.from({ length: 4 }, () => draw(rand, BASE64, 64))
  return `-----BEGIN ${label}-----\n${lines.join('\n')}\n-----END ${label}-----\n`
}

/** The demo value of `key` in the seeded Secret namespace/name. */
export function secretValue(namespace: string, name: string, key: string): string {
  const rand = mulberry32(seedOf(`${namespace}/${name}/${key}`))
  if (key.endsWith('.crt')) return pem(rand, 'CERTIFICATE')
  if (key.endsWith('.key')) return pem(rand, 'PRIVATE KEY')
  if (key === '.dockerconfigjson') {
    const auth = btoa(`demo:${draw(rand, ALPHANUMERIC, 24)}`)
    return JSON.stringify({ auths: { 'ghcr.io': { auth } } })
  }
  if (key.endsWith('user')) return 'admin'
  return draw(rand, ALPHANUMERIC, 20)
}
