import type { LogEntry } from './logLine'

/** The log lines (not the view's notices) as text, with or without their timestamps. */
export function logText(entries: LogEntry[], timestamps: boolean): string {
  let text = ''
  for (const entry of entries) {
    if (entry.kind === 'notice') continue
    const source = entry.source ? `[${entry.source}] ` : ''
    const ts = timestamps && entry.ts ? `${entry.ts} ` : ''
    text += `${source}${ts}${entry.text}\n`
  }
  return text
}

/** A file name from parts like the pod and container: `web-1-app.log`. */
export function logFileName(...parts: string[]): string {
  const base = parts
    .join('-')
    .replace(/[^\w.-]+/g, '-')
    .replace(/^-+|-+$/g, '')
  return `${base || 'logs'}.log`
}

/** Saves text as a download. */
export function downloadText(fileName: string, text: string): void {
  const url = URL.createObjectURL(new Blob([text], { type: 'text/plain;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = url
  link.download = fileName
  document.body.appendChild(link)
  link.click()
  link.remove()
  // Revoked after the click has been handled, when the download has started.
  setTimeout(() => URL.revokeObjectURL(url))
}
