/**
 * A tiny Markdown subset for release notes and the changelog (design
 * §7.11): headings, bullet and numbered lists (one nesting level), fenced
 * code, paragraphs; inline `code`, **bold**, *emphasis* and links. The
 * result is plain data rendered as React text nodes, so raw HTML in the
 * source stays visible text and can never become markup.
 */

export type Inline =
  | { type: 'text'; text: string }
  | { type: 'code'; text: string }
  | { type: 'strong'; children: Inline[] }
  | { type: 'em'; children: Inline[] }
  | { type: 'link'; href: string; children: Inline[] }

export interface ListItem {
  content: Inline[]
  children: ListItem[]
}

export type Block =
  | { type: 'heading'; level: number; content: Inline[] }
  | { type: 'paragraph'; content: Inline[] }
  | { type: 'list'; ordered: boolean; items: ListItem[] }
  | { type: 'code'; text: string }

const HEADING = /^(#{1,6})\s+(.*?)\s*#*\s*$/
const ITEM = /^(\s*)([-*+]|\d+[.)])\s+(.*)$/
const FENCE = /^\s*(```|~~~)/
const COMMENT_START = /^\s*<!--/

/** Only web links become links; anything else (javascript:, data:, relative) stays text. */
export function safeHref(href: string): string | null {
  try {
    const url = new URL(href)
    return url.protocol === 'https:' || url.protocol === 'http:' ? url.href : null
  } catch {
    return null
  }
}

export function parseMarkdown(source: string): Block[] {
  const lines = source.replace(/\r\n?/g, '\n').split('\n')
  const blocks: Block[] = []
  let paragraph: string[] = []
  let list: { ordered: boolean; items: ListItem[]; indent: number } | null = null

  const flushParagraph = () => {
    if (paragraph.length) blocks.push({ type: 'paragraph', content: parseInline(paragraph.join(' ')) })
    paragraph = []
  }
  const flushList = () => {
    if (list) blocks.push({ type: 'list', ordered: list.ordered, items: list.items })
    list = null
  }

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (FENCE.test(line)) {
      flushParagraph()
      flushList()
      const fence = FENCE.exec(line)![1]
      const code: string[] = []
      for (i++; i < lines.length && !lines[i].trimStart().startsWith(fence); i++) code.push(lines[i])
      blocks.push({ type: 'code', text: code.join('\n') })
      continue
    }
    if (COMMENT_START.test(line)) {
      // HTML comments (Keep a Changelog notes to maintainers) are not content.
      while (i < lines.length && !lines[i].includes('-->')) i++
      continue
    }
    if (!line.trim()) {
      flushParagraph()
      continue
    }
    const heading = HEADING.exec(line)
    if (heading) {
      flushParagraph()
      flushList()
      blocks.push({ type: 'heading', level: heading[1].length, content: parseInline(heading[2]) })
      continue
    }
    const item = ITEM.exec(line)
    if (item) {
      flushParagraph()
      const indent = item[1].length
      const ordered = /\d/.test(item[2])
      const entry: ListItem = { content: parseInline(item[3]), children: [] }
      if (list && indent > list.indent && list.items.length) {
        list.items[list.items.length - 1].children.push(entry)
      } else {
        if (!list || list.ordered !== ordered) {
          flushList()
          list = { ordered, items: [], indent }
        }
        list.items.push(entry)
      }
      continue
    }
    if (list && /^\s+/.test(line)) {
      // A continuation line of the last item (or of its last child).
      const last = list.items[list.items.length - 1]
      const target = last.children.length ? last.children[last.children.length - 1] : last
      target.content = [...target.content, { type: 'text', text: ' ' }, ...parseInline(line.trim())]
      continue
    }
    flushList()
    paragraph.push(line.trim())
  }
  flushParagraph()
  flushList()
  return blocks
}

/** Inline spans: `code`, **strong**, *em* / _em_, [text](url). Everything else is text. */
export function parseInline(text: string): Inline[] {
  const out: Inline[] = []
  let plain = ''
  const pushText = () => {
    if (plain) out.push({ type: 'text', text: plain })
    plain = ''
  }

  let i = 0
  while (i < text.length) {
    const rest = text.slice(i)
    const code = /^(`+)([\s\S]*?[^`])\1(?!`)/.exec(rest)
    if (code) {
      pushText()
      out.push({ type: 'code', text: code[2].trim() })
      i += code[0].length
      continue
    }
    const strong = /^(\*\*|__)(?=\S)([\s\S]*?\S)\1/.exec(rest)
    if (strong) {
      pushText()
      out.push({ type: 'strong', children: parseInline(strong[2]) })
      i += strong[0].length
      continue
    }
    const em = /^(\*|_)(?=\S)([\s\S]*?\S)\1(?![*_])/.exec(rest)
    if (em && (em[1] === '*' || !/\w/.test(text[i - 1] ?? ''))) {
      pushText()
      out.push({ type: 'em', children: parseInline(em[2]) })
      i += em[0].length
      continue
    }
    const link = /^\[([^\]]+)\]\(([^)\s]+)(?:\s+"[^"]*")?\)/.exec(rest)
    if (link) {
      pushText()
      const href = safeHref(link[2])
      const children = parseInline(link[1])
      if (href) out.push({ type: 'link', href, children })
      else out.push(...children)
      i += link[0].length
      continue
    }
    plain += text[i]
    i++
  }
  pushText()
  return out
}
