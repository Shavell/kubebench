import { apiUrl } from '../apiClient'
import type { TerminalConnection, TerminalHandlers, TerminalTarget } from '../streams'

/**
 * Input is sent in chunks so that every message stays far below the
 * server's 64 KiB limit, even when JSON escaping multiplies its size.
 */
const INPUT_CHUNK = 8192

type ServerMessage = { type: 'exit'; code: number } | { type: 'error'; message: string }

const isHighSurrogate = (code: number) => code >= 0xd800 && code <= 0xdbff

/**
 * Opens GET /api/exec/ws (design §7.1): JSON text frames to the server,
 * binary output and JSON exit/error frames back. The session cookie
 * authenticates the handshake; the browser adds the Origin the server
 * checks.
 */
export function openTerminal(target: TerminalTarget, handlers: TerminalHandlers): TerminalConnection {
  const url = new URL(
    apiUrl('/exec/ws', {
      ns: target.namespace,
      pod: target.pod,
      container: target.container,
      command: target.command,
    }),
    window.location.href,
  )
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  const socket = new WebSocket(url)
  socket.binaryType = 'arraybuffer'
  let opened = false
  let cancelled = false

  socket.onopen = () => {
    opened = true
    if (!cancelled) handlers.onOpen()
  }
  socket.onmessage = (event: MessageEvent<ArrayBuffer | string>) => {
    if (cancelled) return
    if (event.data instanceof ArrayBuffer) {
      handlers.onOutput(new Uint8Array(event.data))
      return
    }
    let message: ServerMessage
    try {
      message = JSON.parse(event.data) as ServerMessage
    } catch {
      return // not part of the protocol
    }
    if (message.type === 'exit') handlers.onExit(Number(message.code))
    else if (message.type === 'error') handlers.onError(String(message.message))
  }
  socket.onclose = () => {
    if (!cancelled) handlers.onClose({ opened })
  }

  const send = (message: object) => {
    if (socket.readyState === WebSocket.OPEN) socket.send(JSON.stringify(message))
  }
  return {
    input: (data) => {
      for (let at = 0; at < data.length; ) {
        let end = Math.min(at + INPUT_CHUNK, data.length)
        // Never split a surrogate pair: half of one is not valid text.
        if (end < data.length && isHighSurrogate(data.charCodeAt(end - 1))) end -= 1
        send({ type: 'input', data: data.slice(at, end) })
        at = end
      }
    },
    resize: (cols, rows) => send({ type: 'resize', cols, rows }),
    close: () => {
      cancelled = true
      socket.close()
    },
  }
}

/**
 * A terminal that cannot open, e.g. because the cluster lacks the exec
 * capability: reports `message` and closes, asynchronously like a refused
 * handshake.
 */
export function refusedTerminal(handlers: TerminalHandlers, message: string): TerminalConnection {
  let cancelled = false
  setTimeout(() => {
    if (cancelled) return
    handlers.onError(message)
    handlers.onClose({ opened: false })
  })
  return {
    input: () => {},
    resize: () => {},
    close: () => {
      cancelled = true
    },
  }
}
