import { ApiError } from '../apiClient'
import type { LogStream, LogStreamHandlers, LogStreamRequest } from '../streams'
import type { Pod } from '../types'
import { pods } from './mockCluster'
import { podLogs } from './mockLogs'
import { mulberry32 } from './rng'

/**
 * Deterministic simulation of GET /api/logs/stream for mock mode: the same
 * resolution rules and failures as the backend, the backlog from the mock
 * transcripts (mockLogs), then new lines at a steady pace whose text and
 * timestamps come from a seeded RNG on a simulated clock, never the real
 * one. A crash-looping container's log ends after its backlog.
 */

/** The simulated clock's day; the mock transcripts end at 14:30 on it. */
const DAY = '2026-08-11'
/** The transcripts' "now", in seconds since midnight (14:30:00). */
const NOW_SECONDS = (14 * 60 + 30) * 60
/** Pace of new lines, like a moderately busy service. */
const LINE_INTERVAL_MS = 1200
const OPEN_LATENCY_MS = 150
const MAX_PODS = 10

interface Source {
  pod: Pod
  container: string
  /** `pod/container`, the line prefix of selector streams. */
  id: string
  /** Seconds since midnight of the source's last line. */
  clock: number
  rand: () => number
  messages: string[]
}

export function streamMockLogs(request: LogStreamRequest, handlers: LogStreamHandlers): LogStream {
  let closed = false
  let live: ReturnType<typeof setInterval> | undefined
  const opening = setTimeout(() => {
    if (closed) return
    const resolved = resolve(request)
    if (resolved instanceof ApiError) {
      handlers.onFailure(resolved)
      return
    }
    const { sources, failures, matched } = resolved
    handlers.onOpen(
      [...sources.map((s) => s.id), ...failures.map((f) => f.id)].sort(),
      matched,
    )
    const prefix = (s: Source) => (request.selector ? `[${s.id}] ` : '')
    const format = (s: Source, seconds: number, ms: number, text: string) =>
      prefix(s) + (request.timestamps ? `${timestamp(seconds, ms)} ${text}` : text)

    for (const f of failures) handlers.onError(`[${f.id}] ${f.message}`)
    for (const line of backlog(sources, request)) handlers.onLine(format(line.source, line.seconds, line.ms, line.text))

    // The previous instance has ended, and so does a crash-looping container.
    const following = request.previous ? [] : sources.filter((s) => s.pod.status !== 'CrashLoopBackOff')
    if (following.length === 0) {
      handlers.onEnd()
      return
    }
    let turn = 0
    live = setInterval(() => {
      const source = following[turn++ % following.length]
      source.clock += 1 + Math.floor(source.rand() * 4)
      const text = source.messages[Math.floor(source.rand() * source.messages.length)]
      handlers.onLine(format(source, source.clock, Math.floor(source.rand() * 1000), text))
    }, LINE_INTERVAL_MS)
  }, OPEN_LATENCY_MS)

  return {
    close: () => {
      closed = true
      clearTimeout(opening)
      clearInterval(live)
    },
  }
}

/** The sources a request follows, the pods whose logs cannot be followed, and the number of matching pods. */
function resolve(
  request: LogStreamRequest,
): { sources: Source[]; failures: { id: string; message: string }[]; matched: number } | ApiError {
  const { namespace } = request
  if (request.pod) {
    const pod = pods.find((p) => p.namespace === namespace && p.name === request.pod)
    if (!pod) return new ApiError(404, 'kubernetes', `pods "${request.pod}" not found`)
    const container = request.container || pod.containers[0]?.name
    if (!pod.containers.some((c) => c.name === container)) {
      return new ApiError(400, 'kubernetes', `container "${container}" is not valid for pod "${pod.name}"`)
    }
    const problem = unavailable(pod, container, request.previous)
    if (problem) return new ApiError(400, 'kubernetes', problem)
    return { sources: [source(pod, container)], failures: [], matched: 1 }
  }

  const matches = parseSelector(request.selector ?? '')
  if (!matches) return new ApiError(400, 'bad_request', `invalid selector: ${request.selector}`)
  const matching = pods
    .filter((p) => p.namespace === namespace && matches(p.labels))
    .filter((p) => !request.container || p.containers.some((c) => c.name === request.container))
    .sort((a, b) => a.name.localeCompare(b.name))
  if (matching.length === 0) {
    return new ApiError(404, 'not_found', `no pod in namespace "${namespace}" matches "${request.selector}"`)
  }
  const sources: Source[] = []
  const failures: { id: string; message: string }[] = []
  for (const pod of matching.slice(0, MAX_PODS)) {
    const container = request.container || pod.containers[0].name
    const problem = unavailable(pod, container, request.previous)
    if (problem) failures.push({ id: `${pod.name}/${container}`, message: problem })
    else sources.push(source(pod, container))
  }
  return { sources, failures, matched: matching.length }
}

/** Why the container's log cannot be followed, worded like the Kubernetes API, or null. */
function unavailable(pod: Pod, container: string, previous?: boolean): string | null {
  if (pod.status === 'Pending' || pod.status === 'ContainerCreating') {
    return `container "${container}" in pod "${pod.name}" is waiting to start: ContainerCreating`
  }
  if (previous && !pod.restarts) {
    return `previous terminated container "${container}" in pod "${pod.name}" not found`
  }
  return null
}

function source(pod: Pod, container: string): Source {
  const id = `${pod.name}/${container}`
  return {
    pod,
    container,
    id,
    clock: NOW_SECONDS,
    rand: mulberry32(hash(id)),
    messages: podLogs(pod.name, container).map((line) => line.slice(line.indexOf(' ') + 1)),
  }
}

interface BacklogLine {
  source: Source
  seconds: number
  ms: number
  text: string
}

/**
 * Every source's recent lines (tail per container, like the API), oldest
 * first. Replicas share a transcript, so each source's times are shifted
 * by up to three seconds (seeded) to interleave them like real pods.
 */
function backlog(sources: Source[], request: LogStreamRequest): BacklogLine[] {
  const lines: BacklogLine[] = []
  for (const s of sources) {
    const shift = hash(s.id) % 3000
    const transcript = podLogs(s.pod.name, s.container, {
      tail: request.tail || undefined,
      previous: request.previous,
      sinceSeconds: request.sinceSeconds,
    })
    for (const line of transcript) {
      // "HH:MM:SS.mmm text"
      const [hours, minutes, rest] = line.split(':')
      const seconds = Number(hours) * 3600 + Number(minutes) * 60 + Number(rest.slice(0, 2))
      const total = seconds * 1000 + Number(rest.slice(3, 6)) + shift
      lines.push({ source: s, seconds: Math.floor(total / 1000), ms: total % 1000, text: line.slice(line.indexOf(' ') + 1) })
    }
  }
  return lines.sort((a, b) => a.seconds - b.seconds || a.ms - b.ms)
}

/** RFC 3339 with the fraction trimmed, as the Kubernetes API writes it. */
function timestamp(seconds: number, ms: number): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  const day = Math.floor(seconds / 86400)
  const s = seconds % 86400
  const time = `${pad(Math.floor(s / 3600))}:${pad(Math.floor(s / 60) % 60)}:${pad(s % 60)}`
  const fraction = ms ? `.${String(ms).padStart(3, '0').replace(/0+$/, '')}` : ''
  // The simulated clock does not run long enough to need month arithmetic.
  const date = day ? `${DAY.slice(0, 8)}${pad(Number(DAY.slice(8)) + day)}` : DAY
  return `${date}T${time}${fraction}Z`
}

function hash(text: string): number {
  let h = 0x811c9dc5
  for (let i = 0; i < text.length; i++) h = Math.imul(h ^ text.charCodeAt(i), 0x01000193)
  return h >>> 0
}

/**
 * A matcher for a label selector in the Kubernetes string syntax
 * (`a=b`, `a==b`, `a!=b`, `a in (x,y)`, `a notin (x)`, `a`, `!a`), or null
 * when it does not parse.
 */
function parseSelector(selector: string): ((labels: Record<string, string>) => boolean) | null {
  const tests: ((labels: Record<string, string>) => boolean)[] = []
  for (const raw of splitRequirements(selector)) {
    const req = raw.trim()
    let m: RegExpExecArray | null
    if ((m = /^([\w./-]+)\s*(==|=|!=)\s*([\w./-]*)$/.exec(req))) {
      const [, key, op, value] = m
      tests.push(op === '!=' ? (l) => l[key] !== value : (l) => l[key] === value)
    } else if ((m = /^([\w./-]+)\s+(in|notin)\s*\(([^)]*)\)$/.exec(req))) {
      const [, key, op, list] = m
      const values = list.split(',').map((v) => v.trim())
      tests.push(op === 'in' ? (l) => values.includes(l[key]) : (l) => !values.includes(l[key]))
    } else if ((m = /^!([\w./-]+)$/.exec(req))) {
      const key = m[1]
      tests.push((l) => !(key in l))
    } else if ((m = /^([\w./-]+)$/.exec(req))) {
      const key = m[1]
      tests.push((l) => key in l)
    } else {
      return null
    }
  }
  return tests.length ? (labels) => tests.every((test) => test(labels)) : null
}

/** Splits at the commas between requirements, not those inside `( … )`. */
function splitRequirements(selector: string): string[] {
  const parts: string[] = []
  let depth = 0
  let start = 0
  for (let i = 0; i < selector.length; i++) {
    if (selector[i] === '(') depth++
    else if (selector[i] === ')') depth--
    else if (selector[i] === ',' && depth === 0) {
      parts.push(selector.slice(start, i))
      start = i + 1
    }
  }
  parts.push(selector.slice(start))
  return parts.filter((p) => p.trim() !== '')
}
