import { PLATFORM_NS } from '@/core/locales'
import { errorMessage } from '@/data/apiClient'
import { i18n } from '@/i18n'
import { notify } from './notify'

/**
 * Copies text to the clipboard and confirms with a toast. The Clipboard API
 * only exists in secure contexts (HTTPS, localhost); a KubeBench served over
 * plain HTTP falls back to a hidden text field and the copy command.
 */
export async function copyText(text: string): Promise<void> {
  try {
    if (navigator.clipboard) await navigator.clipboard.writeText(text)
    else copyWithSelection(text)
    notify.success(i18n.t('ui.clipboard.copied', { ns: PLATFORM_NS }))
  } catch (err) {
    notify.error(i18n.t('ui.clipboard.failed', { ns: PLATFORM_NS }), { detail: errorMessage(err) })
  }
}

function copyWithSelection(text: string): void {
  const field = document.createElement('textarea')
  field.value = text
  field.setAttribute('readonly', '')
  field.style.position = 'fixed'
  field.style.opacity = '0'
  document.body.append(field)
  field.select()
  try {
    if (!document.execCommand('copy')) throw new Error('the browser refused to copy')
  } finally {
    field.remove()
  }
}

/** Saves text as a file through the browser's download. */
export function downloadText(filename: string, text: string, type = 'application/yaml'): void {
  const url = URL.createObjectURL(new Blob([text], { type }))
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  // Revoked once the click has started the download.
  setTimeout(() => URL.revokeObjectURL(url))
}
