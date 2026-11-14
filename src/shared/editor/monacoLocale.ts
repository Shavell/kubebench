import { i18n } from '@/i18n'

/**
 * Monaco's own UI strings (find widget, context menu, read-only notice) for
 * the app languages; English is built in. Monaco reads them once, while its
 * modules evaluate, so the language active when the first editor opens is
 * kept until the page reloads.
 */
const MONACO_MESSAGES: Record<string, () => Promise<unknown>> = {
  tr: () => import('monaco-editor/nls/lang/tr.js'),
}

let messagesLoaded: Promise<void> | null = null

/** Runs `load` (an import of a module that uses Monaco) after Monaco's messages are in place. */
export function importWithMonacoMessages<T>(load: () => Promise<T>): Promise<T> {
  messagesLoaded ??= Promise.resolve(MONACO_MESSAGES[i18n.language]?.()).then(
    () => undefined,
    (err: unknown) => {
      messagesLoaded = null // let a retry fetch them again
      throw err
    },
  )
  return messagesLoaded.then(load)
}
