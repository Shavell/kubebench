/** Build-time constants injected by vite.config.ts (`define`). */

/** package.json version of this frontend build. */
declare const __APP_VERSION__: string

interface ImportMetaEnv {
  /** "true" builds a demo bundle that always runs on deterministic mock data. */
  readonly VITE_DEMO_MODE?: string
}
