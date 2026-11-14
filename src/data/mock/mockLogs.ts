import type { PodLogOptions } from '../types'
import { mulberry32 } from './rng'

/**
 * Deterministic per-pod log transcripts. Timestamps count up to a fixed
 * "now" (14:30) so output is stable across reloads.
 */

const APP_LINES: Record<string, string[]> = {
  'shop-frontend': [
    'INFO  server listening on :8080',
    'INFO  GET /api/products 200 12ms',
    'INFO  GET /api/cart 200 4ms',
    'INFO  GET /healthz 200 1ms',
    'INFO  GET /api/products/61 200 9ms',
    'WARN  POST /api/checkout 502 upstream shop-payments unavailable',
    'INFO  retrying checkout via fallback queue',
    'INFO  GET /static/main.css 304 0ms',
  ],
  'shop-cart': [
    'INFO  cart service ready (redis ok)',
    'INFO  cart 8f3a2c updated (3 items)',
    'INFO  cart b2e991 created',
    'INFO  session gc: 214 expired carts removed',
    'INFO  GET /healthz 200',
  ],
  'shop-payments': [
    'INFO  starting payment service v3.1.0-rc.2',
    'INFO  loading config from /etc/payments/config.yaml',
    'INFO  connecting to gateway https://gw.pay.example.com',
    'WARN  gateway handshake slow (3.0s > 2.5s budget)',
    'ERROR gateway healthcheck failed: context deadline exceeded',
    'ERROR startup probe failed, exiting',
    'INFO  starting payment service v3.1.0-rc.2',
    'INFO  connecting to gateway https://gw.pay.example.com',
    'ERROR gateway healthcheck failed: context deadline exceeded',
    'ERROR startup probe failed, exiting',
  ],
  postgres: [
    'LOG:  database system is ready to accept connections',
    'LOG:  checkpoint starting: time',
    'LOG:  checkpoint complete: wrote 42 buffers (0.3%)',
    'LOG:  automatic vacuum of table "shop.public.carts"',
  ],
  prometheus: [
    'level=info msg="Server is ready to receive web requests."',
    'level=info component=tsdb msg="head GC completed" duration=48ms',
    'level=warn component=scrape msg="target down" target=shop-payments:9102',
    'level=info component=tsdb msg="WAL segment loaded" segment=212',
  ],
  coredns: [
    '[INFO] plugin/ready: Still waiting on: "kubernetes"',
    '[INFO] 10.244.1.21:44312 - 51234 "A IN shop-cart.shop.svc.cluster.local. udp 50 false 512" NOERROR qr,aa,rd 94 0.0001s',
    '[INFO] 10.244.2.21:38911 - 8871 "A IN gw.pay.example.com. udp 36 false 512" NOERROR qr,rd,ra 62 0.0021s',
  ],
}

const DEFAULT_LINES = [
  'INFO  process started',
  'INFO  configuration loaded',
  'INFO  ready',
  'INFO  heartbeat ok',
]

/** The fixed "now" the transcripts end at: 14:30:00. */
const NOW_SECONDS = (14 * 60 + 30) * 60

/**
 * Log lines of one container instance. `previous` gives the transcript of
 * the instance before the last restart, which ended with a crash;
 * `sinceSeconds` and `tail` filter like the Kubernetes log API.
 */
export function podLogs(pod: string, container: string, options: PodLogOptions = {}): string[] {
  const lines = [...(APP_LINES[container] ?? DEFAULT_LINES)]
  if (options.previous) lines.push('ERROR received SIGTERM, shutting down (exit code 1)')
  const rand = mulberry32(pod.length * 2654435761 + (options.previous ? 7 : 0))
  // The previous instance ended a few minutes before the current one started.
  let seconds = NOW_SECONDS - lines.length * 120 - (options.previous ? 600 : 0)
  const out: string[] = []
  for (const line of lines) {
    seconds += 60 + Math.floor(rand() * 60)
    const ms = Math.floor(rand() * 1000)
    if (options.sinceSeconds !== undefined && seconds < NOW_SECONDS - options.sinceSeconds) continue
    out.push(`${clock(seconds)}.${String(ms).padStart(3, '0')} ${line}`)
  }
  return options.tail !== undefined ? out.slice(-options.tail) : out
}

function clock(totalSeconds: number): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(Math.floor(totalSeconds / 3600))}:${pad(Math.floor(totalSeconds / 60) % 60)}:${pad(totalSeconds % 60)}`
}
