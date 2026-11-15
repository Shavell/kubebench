/** What one apply request may carry (the backend's maxManifestBytes). */
export const MAX_MANIFEST_BYTES = 4 << 20

/** Whether a drag carries files (rather than text being moved in the editor). */
export const draggingFiles = (types: readonly string[]) => types.includes('Files')

/**
 * Picked or dropped manifest files as one multi-document text. Several
 * files are joined with document separators, the first one included, so
 * that JSON files are read as YAML documents too.
 */
export async function readManifestFiles(files: readonly File[]): Promise<string> {
  const texts = await Promise.all(files.map((file) => file.text()))
  if (texts.length === 1) return texts[0]
  return texts.map((text) => `---\n${text.trim()}\n`).join('')
}
