/** Longest font family list a user may enter. */
const MAX_LENGTH = 200

/**
 * Whether `value` is a font family list we pass on to CSS, Monaco and
 * xterm.js: names (quoted or not) separated by commas, nothing that could
 * end the declaration or open another one.
 */
export function isValidFontFamily(value: string): boolean {
  const trimmed = value.trim()
  if (trimmed.length > MAX_LENGTH) return false
  if (!/^[\p{L}\p{N} ,'"._-]*$/u.test(trimmed)) return false
  // Quotes must pair up within each name.
  return trimmed.split(',').every((name) => {
    const n = name.trim()
    if (n === '') return trimmed === ''
    const quote = n[0] === '"' || n[0] === "'" ? n[0] : null
    if (quote) return n.length > 2 && n.endsWith(quote) && !n.slice(1, -1).includes(quote)
    return !n.includes('"') && !n.includes("'")
  })
}

/**
 * The family list to use: the user's choice ahead of `fallback` (so a font
 * that is not installed falls back to the default stack), or `fallback`
 * alone when there is no valid choice.
 */
export function fontStack(custom: string, fallback: string): string {
  const trimmed = custom.trim()
  return trimmed && isValidFontFamily(trimmed) ? `${trimmed}, ${fallback}` : fallback
}
