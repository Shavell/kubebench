/** Types of the service browser (design §7.14), shaped like the backend's JSON. */

/** The API resource whose proxy subresource a browser uses. */
export type BrowserKind = 'services' | 'pods'

export type Scheme = 'http' | 'https'

/** A port a browser can open. */
export interface BrowserPort {
  port: number
  name?: string
  scheme: Scheme
}

/** The object and port a browser session reaches. */
export interface BrowserTarget {
  kind: BrowserKind
  namespace: string
  name: string
  /** Port number (or name), as a string. */
  port: string
  scheme: Scheme
}

/**
 * A browsing session (a grant of the backend's proxy). Its pages are
 * served below `base`; the token in it is the only credential they carry.
 */
export interface ProxySession {
  token: string
  /** URL of the target's root without a trailing slash, e.g. `/proxy/<token>`. */
  base: string
  /**
   * Served on KubeBench's own origin with a sandbox; false when the
   * server has a separate proxy origin (KUBEBENCH_PROXY_ORIGIN).
   */
  sandboxed: boolean
  /** The cluster is read-only: only GET and HEAD requests are sent. */
  readOnly: boolean
  target: BrowserTarget
  /** In-cluster path of the document the frame shows, "" before the first. */
  location: string
}

/** What the frame loads: a URL, or (demo mode) a document of its own. */
export type FrameSource = { src: string } | { srcDoc: string }
