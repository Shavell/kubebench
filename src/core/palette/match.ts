/**
 * Case- and accent-insensitive form for matching translated titles:
 * "is yukleri" finds "İş Yükleri", and dotted/dotless i are interchangeable.
 */
export function fold(text: string): string {
  return text.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase().replace(/ı/g, 'i')
}

/** Subsequence match on folded text, like VSCode's fuzzy filter. */
export function fuzzyMatch(query: string, target: string): boolean {
  const haystack = fold(target)
  let ti = 0
  for (const ch of fold(query)) {
    if (ch === ' ') continue
    ti = haystack.indexOf(ch, ti)
    if (ti === -1) return false
    ti++
  }
  return true
}
