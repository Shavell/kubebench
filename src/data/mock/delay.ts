import { mulberry32 } from './rng'

/** Simulated network latency (seeded) so loading states are exercised for real. */
const latencyRand = mulberry32(0x6b85)

export function delay<T>(value: T): Promise<T> {
  const ms = 150 + latencyRand() * 150
  return new Promise((resolve) => setTimeout(() => resolve(value), ms))
}

/** Rejects after the same simulated latency. */
export function delayReject<T = never>(error: Error): Promise<T> {
  return delay(undefined).then(() => Promise.reject(error))
}
