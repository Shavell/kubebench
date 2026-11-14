/**
 * When the app may run on deterministic mock data (design §6.3).
 *
 * - `forced`: a demo build (`VITE_DEMO_MODE=true`) or a URL with `?demo`.
 *   The backend is not contacted at all.
 * - `fallback`: the Vite dev server (`import.meta.env.DEV`) uses mock data
 *   when the backend cannot be reached, so the UI can be developed alone.
 *
 * A production build never falls back: when its backend is unreachable or
 * failing it shows the boot error screen, never invented data.
 */
export interface MockPolicy {
  forced: boolean
  fallback: boolean
}

export function mockPolicy(search: string = window.location.search): MockPolicy {
  const demoBuild = import.meta.env.VITE_DEMO_MODE === 'true'
  const demoUrl = new URLSearchParams(search).has('demo')
  return { forced: demoBuild || demoUrl, fallback: import.meta.env.DEV }
}
