/**
 * Turns an eager Vite glob of translation files into the `{ <lang>: bundle }`
 * map that `registerTranslations` and `PluginManifest.locales` take:
 *
 *   localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' }))
 *
 * The glob has to stay at the call site (Vite resolves it relative to the
 * importing module), but the language set comes from the files on disk, so
 * adding a language never touches the code that registers a namespace.
 */
export function localeBundles(modules: Record<string, unknown>): Record<string, object> {
  const bundles: Record<string, object> = {}
  for (const [path, bundle] of Object.entries(modules)) {
    const code = /([^/]+)\.json$/.exec(path)?.[1]
    if (code && bundle !== null && typeof bundle === 'object') bundles[code] = bundle
  }
  return bundles
}

/** Picks one top-level subtree of every bundle (e.g. "workbench" of the platform files). */
export function subtreeBundles(bundles: Record<string, object>, key: string): Record<string, object> {
  const out: Record<string, object> = {}
  for (const [code, bundle] of Object.entries(bundles)) {
    const subtree = (bundle as Record<string, unknown>)[key]
    if (subtree !== null && typeof subtree === 'object') out[code] = subtree
  }
  return out
}
