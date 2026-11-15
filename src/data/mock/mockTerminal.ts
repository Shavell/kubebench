import type { TerminalConnection, TerminalHandlers, TerminalTarget } from '../streams'

const GREEN = '\x1b[1;32m'
const DIM = '\x1b[2m'
const RESET = '\x1b[0m'
const CLEAR_SCREEN = '\x1b[2J\x1b[3J\x1b[H'
const CLEAR_LINE = '\r\x1b[2K'

/**
 * The escape sequence of a cursor or function key at data[at] (ESC [ or
 * ESC O, parameters, a final character; e.g. ESC [ A for ↑), or null.
 */
function keySequenceAt(data: string, at: number): { length: number; final: string } | null {
  if (data[at] !== '\x1b' || (data[at + 1] !== '[' && data[at + 1] !== 'O')) return null
  let end = at + 2
  while (end < data.length && ((data[end] >= '0' && data[end] <= '9') || data[end] === ';')) end++
  return end < data.length ? { length: end + 1 - at, final: data[end] } : null
}

/**
 * A simulated shell for mock mode, speaking the same byte-level protocol
 * as a real terminal: it echoes input, edits the line (Backspace, Ctrl+C,
 * Ctrl+U, Ctrl+L, ↑/↓ history), runs commands through `exec` (the mock
 * exec's canned answers) and handles `clear` and `exit [code]` itself.
 * Deterministic: no clock in the output, no randomness.
 */
export function openMockTerminal(
  target: TerminalTarget,
  handlers: TerminalHandlers,
  exec: (command: string) => Promise<string[]>,
): TerminalConnection {
  const encoder = new TextEncoder()
  const prompt = `${GREEN}root@${target.pod}${RESET}:/# `
  const history: string[] = []
  let historyAt = 0
  let line = ''
  let opened = false
  let busy = false
  let ended = false
  let cancelled = false

  const write = (text: string) => {
    if (!cancelled) handlers.onOutput(encoder.encode(text))
  }
  const replaceLine = (text: string) => {
    line = text
    write(CLEAR_LINE + prompt + text)
  }
  const exit = (code: number) => {
    ended = true
    // The server closes the connection right after the exit message.
    setTimeout(() => {
      if (cancelled) return
      handlers.onExit(code)
      handlers.onClose({ opened: true })
    })
  }

  const run = async (command: string) => {
    if (command) history.push(command)
    historyAt = history.length
    const exitCode = /^exit(?:\s+(\d+))?$/.exec(command)
    if (exitCode) {
      exit(Number(exitCode[1] ?? 0) % 256)
      return
    }
    if (command === 'clear') {
      write(CLEAR_SCREEN + prompt)
      return
    }
    busy = true
    const output = await exec(command)
    busy = false
    if (ended) return
    write(output.map((text) => `${text}\r\n`).join('') + prompt)
  }

  const key = (ch: string) => {
    switch (ch) {
      case '\r':
      case '\n':
        write('\r\n')
        void run(line.trim())
        line = ''
        return
      case '\x7f': // Backspace
      case '\b':
        if (line) {
          line = Array.from(line).slice(0, -1).join('')
          write('\b \b')
        }
        return
      case '\x03': // Ctrl+C
        write('^C\r\n' + prompt)
        line = ''
        historyAt = history.length
        return
      case '\x04': // Ctrl+D on an empty line
        if (!line) {
          write('exit\r\n')
          exit(0)
        }
        return
      case '\x0c': // Ctrl+L
        write(CLEAR_SCREEN + prompt + line)
        return
      case '\x15': // Ctrl+U
        replaceLine('')
        return
    }
    if (ch >= ' ') {
      line += ch
      write(ch)
    }
  }

  const cursorKey = (final: string) => {
    if (final === 'A' && historyAt > 0) replaceLine(history[--historyAt])
    if (final === 'B' && historyAt < history.length) replaceLine(history[++historyAt] ?? '')
  }

  const timer = setTimeout(() => {
    if (cancelled) return
    opened = true
    handlers.onOpen()
    write(
      `${DIM}KubeBench mock shell for ${target.namespace}/${target.pod} (${target.container}). ` +
        `Commands are simulated; try help, clear or exit.${RESET}\r\n${prompt}`,
    )
  }, 120)

  return {
    input: (data) => {
      if (!opened || ended || busy) return
      for (let at = 0; at < data.length; ) {
        const sequence = keySequenceAt(data, at)
        if (sequence) {
          cursorKey(sequence.final)
          at += sequence.length
          continue
        }
        const ch = String.fromCodePoint(data.codePointAt(at) ?? 0)
        key(ch)
        at += ch.length
        if (ended || busy) return
      }
    },
    resize: () => {},
    close: () => {
      cancelled = true
      clearTimeout(timer)
    },
  }
}
