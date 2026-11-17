import type { BrowserKind, BrowserPort, BrowserTarget, FrameSource, ProxySession } from './types'

/**
 * The service browser's data capability (design §7.14). Live: the core
 * routes /api/proxy/… and the proxy under /proxy/<token>/; mock: a
 * deterministic placeholder page per path.
 */
export interface ServiceBrowserProvider {
  /** The TCP ports of a Service or of a Pod's containers. */
  listPorts(kind: BrowserKind, namespace: string, name: string): Promise<BrowserPort[]>
  /** Opens a browsing session (needs the browse permission; audited once). */
  open(target: BrowserTarget): Promise<ProxySession>
  /** The session as it is now, with the documents its frames loaded. */
  session(token: string): Promise<ProxySession>
  /** Ends the session; its URLs stop working. */
  close(token: string): Promise<void>
  /** What the frame loads to show `path` of the session's target. */
  frame(session: ProxySession, path: string): FrameSource
}
