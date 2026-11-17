/**
 * The locally bundled Monaco editor (design §6.6: no CDN, works offline).
 * Only the lazily loaded editor components import this module, so Monaco
 * stays out of the main chunk.
 *
 * All editor features are registered up front: Monaco's JSON support loads
 * nearly every editor contribution anyway (through its worker manager), and
 * contributions registered after the first editor exists fail to find their
 * services. Languages are opted into individually.
 */
import * as monaco from 'monaco-editor/editor'
import 'monaco-editor/features/register.all'
import 'monaco-editor/languages/definitions/yaml/register'
import 'monaco-editor/languages/definitions/shell/register'
import 'monaco-editor/languages/definitions/markdown/register'
import 'monaco-editor/languages/features/json/register'
import EditorWorker from 'monaco-editor/editor/editor.worker?worker'
import JsonWorker from 'monaco-editor/languages/features/json/json.worker?worker'
import { loader } from '@monaco-editor/react'
import type { ResolvedTheme } from '@/core/theme/themeController'

// Workers are bundled by Vite (`?worker`); JSON validation and formatting
// run in their own worker, everything else in the generic editor worker.
self.MonacoEnvironment = {
  getWorker: (_workerId, label) => (label === 'json' ? new JsonWorker() : new EditorWorker()),
}

// Hand the bundled instance to @monaco-editor/react so it never injects the
// AMD loader from a CDN.
loader.config({ monaco })

/** Monaco's built-in theme matching the app palette. */
export function monacoTheme(theme: ResolvedTheme): 'vs' | 'vs-dark' {
  return theme === 'light' ? 'vs' : 'vs-dark'
}

/**
 * The --font-mono token as a concrete family list: Monaco also draws text on
 * canvases (minimap), where CSS variables do not resolve.
 */
export const EDITOR_FONT_FAMILY =
  getComputedStyle(document.documentElement).getPropertyValue('--font-mono').trim() || 'monospace'
