import type { FitAddon } from '@xterm/addon-fit'
import type { ITheme, Terminal } from '@xterm/xterm'
import type { ClusterProvider } from '@/data/providers'
import type { TerminalConnection, TerminalTarget } from '@/data/streams'
import { loadXterm } from './xterm'

/** Where a session stands; the toolbar shows it with its reason. */
export type TerminalStatus =
  | { state: 'loading' }
  | { state: 'loadFailed'; error: unknown }
  | { state: 'connecting' }
  | { state: 'open' }
  | { state: 'exited'; code: number }
  /** The server ended the session with an error. */
  | { state: 'failed'; message: string }
  /** The connection never opened: refused or unreachable. */
  | { state: 'refused' }
  /** The connection dropped before the process exited. */
  | { state: 'lost' }

export interface TerminalAppearance {
  fontSize: number
  fontFamily: string
  theme: ITheme
}

const IS_MAC = /Mac|iPhone|iPad/.test(navigator.userAgent)

/**
 * One terminal session: an xterm.js screen plus its connection to a
 * container. It lives outside React so it survives views that unmount
 * while hidden (bottom panel tabs, editor tabs). Views move `host` into
 * their layout with attach() and take it out again with detach().
 */
export class TerminalSession {
  readonly id: string
  /** The element xterm.js renders into. */
  readonly host: HTMLDivElement
  private targetValue: TerminalTarget
  private statusValue: TerminalStatus = { state: 'loading' }
  private readonly provider: ClusterProvider
  private readonly listeners = new Set<() => void>()
  private term: Terminal | null = null
  private fit: FitAddon | null = null
  private connection: TerminalConnection | null = null
  private appearance: TerminalAppearance | null = null
  private opened = false
  private disposed = false

  constructor(id: string, target: TerminalTarget, provider: ClusterProvider) {
    this.id = id
    this.targetValue = target
    this.provider = provider
    this.host = document.createElement('div')
    this.host.className = 'kb-terminal__screen'
    void this.load()
  }

  get target(): TerminalTarget {
    return this.targetValue
  }

  get status(): TerminalStatus {
    return this.statusValue
  }

  /** For useSyncExternalStore: notifies on status and target changes. */
  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)
    return () => this.listeners.delete(listener)
  }

  /** Puts the screen into `parent` and gives it the keyboard. */
  attach(parent: HTMLElement): void {
    parent.appendChild(this.host)
    this.openScreen()
    this.fitToHost()
    this.term?.focus()
  }

  detach(): void {
    this.host.remove()
  }

  /** Resizes the terminal to its element; the process learns the new size. */
  fitToHost(): void {
    if (this.term && this.fit && this.opened && this.host.isConnected && this.host.clientWidth > 0) {
      this.fit.fit()
    }
  }

  setAppearance(appearance: TerminalAppearance): void {
    this.appearance = appearance
    if (!this.term) return
    this.term.options.fontSize = appearance.fontSize
    this.term.options.fontFamily = appearance.fontFamily
    this.term.options.theme = appearance.theme
    this.fitToHost()
  }

  /** Starts over: a new connection, a clean screen and, optionally, another container. */
  reconnect(target: TerminalTarget = this.targetValue): void {
    if (!this.term) return
    this.targetValue = target
    this.term.reset()
    this.connect()
  }

  /** Tries to load xterm.js again after a failed load. */
  retryLoad(): void {
    if (this.statusValue.state === 'loadFailed') void this.load()
  }

  hasSelection(): boolean {
    return this.term?.hasSelection() ?? false
  }

  /** Copies the selected text; false when nothing is selected. */
  async copySelection(): Promise<boolean> {
    const text = this.term?.getSelection() ?? ''
    if (!text) return false
    await navigator.clipboard.writeText(text)
    return true
  }

  /** Sends text as a paste (bracketed when the program asked for it). */
  paste(text: string): void {
    this.term?.paste(text)
    this.term?.focus()
  }

  selectAll(): void {
    this.term?.selectAll()
  }

  clear(): void {
    this.term?.clear()
    this.term?.focus()
  }

  dispose(): void {
    this.disposed = true
    this.connection?.close()
    this.connection = null
    this.term?.dispose()
    this.term = null
    this.host.remove()
    this.listeners.clear()
  }

  private setStatus(status: TerminalStatus): void {
    this.statusValue = status
    this.listeners.forEach((listener) => listener())
  }

  private async load(): Promise<void> {
    this.setStatus({ state: 'loading' })
    let xterm
    try {
      xterm = await loadXterm()
    } catch (error) {
      if (!this.disposed) this.setStatus({ state: 'loadFailed', error })
      return
    }
    if (this.disposed) return
    const term = new xterm.Terminal({ cursorBlink: true, scrollback: 5000, ...this.appearance })
    this.fit = new xterm.FitAddon()
    term.loadAddon(this.fit)
    term.onData((data) => this.connection?.input(data))
    term.onResize(({ cols, rows }) => this.connection?.resize(cols, rows))
    term.attachCustomKeyEventHandler((event) => this.clipboardKey(event))
    this.term = term
    this.openScreen()
    this.fitToHost()
    this.connect()
  }

  /**
   * Opens xterm.js into the host once the host is in the document: xterm
   * measures its font when it opens and would render nothing otherwise.
   */
  private openScreen(): void {
    if (!this.term || this.opened || !this.host.isConnected) return
    this.term.open(this.host)
    this.opened = true
  }

  private connect(): void {
    const term = this.term
    if (!term || this.disposed) return
    this.connection?.close()
    this.setStatus({ state: 'connecting' })
    let ended = false
    const connection = this.provider.openTerminal(this.targetValue, {
      onOpen: () => {
        this.setStatus({ state: 'open' })
        connection.resize(term.cols, term.rows)
      },
      onOutput: (data) => term.write(data),
      onExit: (code) => {
        ended = true
        this.setStatus({ state: 'exited', code })
      },
      onError: (message) => {
        ended = true
        this.setStatus({ state: 'failed', message })
      },
      onClose: ({ opened }) => {
        if (this.connection === connection) this.connection = null
        if (!ended) this.setStatus({ state: opened ? 'lost' : 'refused' })
      },
    })
    this.connection = connection
  }

  /**
   * Clipboard keys: Ctrl+C copies when text is selected (else it is the
   * interrupt), Ctrl+Shift+C always copies, and Ctrl+V / Ctrl+Shift+V are
   * left to the browser, whose paste event xterm.js turns into input. On
   * macOS Cmd+C and Cmd+V do the same.
   */
  private clipboardKey(event: KeyboardEvent): boolean {
    if (event.type !== 'keydown') return true
    const modifier = IS_MAC ? event.metaKey : event.ctrlKey
    if (!modifier || event.altKey) return true
    if (event.code === 'KeyC' && (event.shiftKey || this.hasSelection())) {
      void this.copySelection().then((copied) => copied && this.term?.clearSelection(), () => undefined)
      event.preventDefault()
      return false
    }
    return event.code !== 'KeyV'
  }
}
