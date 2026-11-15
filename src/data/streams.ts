import type { ApiError } from './apiClient'

/**
 * Contracts of the long-lived data capabilities (design §7.1): following
 * container logs and interactive terminals. Unlike the request/response
 * providers they report through handlers until closed. Both the mock and
 * the live implementation call no handler after `close()`.
 */

/** Which container logs a stream follows (GET /api/logs/stream). */
export interface LogStreamRequest {
  namespace: string
  /** One pod … */
  pod?: string
  /** … or the pods a label selector matches (at most 10; lines start with `[pod/container] `). */
  selector?: string
  /** Defaults to each pod's default container. */
  container?: string
  /** Lines from the end to start with; 0 = the whole log (server default 500, max 10 000). */
  tail?: number
  /** Follow the previous, terminated instance of the container. */
  previous?: boolean
  /** Prefix every line with its RFC 3339 timestamp (after the `[pod/container] ` prefix). */
  timestamps?: boolean
  /** Only lines written in the last N seconds. */
  sinceSeconds?: number
}

export interface LogStreamHandlers {
  /** The stream is open: the containers it follows (`pod/container`) and how many pods matched. */
  onOpen(containers: string[], matched: number): void
  /** One log line, as the server sent it. */
  onLine(line: string): void
  /** One followed log failed; the others go on until `onEnd`. */
  onError(message: string): void
  /** Every followed log ended; the stream is closed. */
  onEnd(): void
  /**
   * The stream was refused or cut off before its end; it is closed. A
   * refusal carries the server's status and code, a broken connection
   * status 0 (code `network`).
   */
  onFailure(error: ApiError): void
}

export interface LogStream {
  close(): void
}

/** The container of an interactive terminal (GET /api/exec/ws). */
export interface TerminalTarget {
  namespace: string
  pod: string
  container: string
  /** A shell from the server's allowlist; by default bash, else sh. */
  command?: string
}

export interface TerminalHandlers {
  onOpen(): void
  /** Terminal output, as the process wrote it. */
  onOutput(data: Uint8Array): void
  /** The process exited with `code`; the connection closes next. */
  onExit(code: number): void
  /** The session failed (the server's reason); the connection closes next. */
  onError(message: string): void
  /** The connection closed; `opened` is false when it never opened (refused or unreachable). */
  onClose(info: { opened: boolean }): void
}

export interface TerminalConnection {
  /** Keystrokes or pasted text. */
  input(data: string): void
  resize(cols: number, rows: number): void
  close(): void
}
