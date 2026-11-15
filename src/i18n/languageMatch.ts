/**
 * Matching BCP 47 language tags (browser preferences) to the supported UI
 * languages. Pure, so it is tested without i18next.
 */

/**
 * Returns a function that maps a tag to a supported code, ignoring case:
 * the exact tag first ("pt-BR" when that is a language of its own), else
 * its base language ("de-AT" → "de"), else undefined.
 */
export function languageMatcher(codes: readonly string[]): (tag: string) => string | undefined {
  const byLowerCase = new Map(codes.map((code) => [code.toLowerCase(), code]))
  return (tag) => {
    const lower = tag.toLowerCase()
    return byLowerCase.get(lower) ?? byLowerCase.get(lower.split('-')[0])
  }
}

/**
 * The locale to format numbers and dates in for UI language `language`: the
 * first browser tag that matches it (a regional variant such as "de-AT"
 * keeps that region's conventions), else the language itself.
 */
export function regionalLocale(
  language: string,
  browserTags: readonly string[],
  match: (tag: string) => string | undefined,
): string {
  return browserTags.find((tag) => match(tag) === language) ?? language
}
