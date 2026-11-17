import { useMemo } from 'react'
import { parseMarkdown, type Block, type Inline, type ListItem } from './markdown'
import './markdown.css'

/**
 * Renders the Markdown subset of `markdown.ts` as React elements. Text is
 * always passed as text nodes (never innerHTML), so release notes and user
 * notes cannot inject markup. `headingOffset` shifts heading levels to fit
 * the host.
 */
export function Markdown({ source, headingOffset = 2 }: { source: string; headingOffset?: number }) {
  const blocks = useMemo(() => parseMarkdown(source), [source])
  return <div className="kb-md">{blocks.map((block, i) => renderBlock(block, i, headingOffset))}</div>
}

function renderBlock(block: Block, key: number, headingOffset: number) {
  switch (block.type) {
    case 'heading': {
      const level = Math.min(6, block.level + headingOffset)
      const Tag = `h${level}` as 'h3'
      return (
        <Tag key={key} className={`kb-md__heading kb-md__heading--${Math.min(block.level, 3)}`}>
          {renderInlines(block.content)}
        </Tag>
      )
    }
    case 'paragraph':
      return <p key={key}>{renderInlines(block.content)}</p>
    case 'code':
      return (
        <pre key={key} className="kb-md__pre">
          <code>{block.text}</code>
        </pre>
      )
    case 'list':
      return renderList(block.items, block.ordered, key)
  }
}

function renderList(items: ListItem[], ordered: boolean, key: number) {
  const Tag = ordered ? 'ol' : 'ul'
  return (
    <Tag key={key}>
      {items.map((item, i) => (
        <li key={i}>
          {renderInlines(item.content)}
          {item.children.length > 0 && renderList(item.children, false, 0)}
        </li>
      ))}
    </Tag>
  )
}

function renderInlines(inlines: Inline[]) {
  return inlines.map((inline, i) => {
    switch (inline.type) {
      case 'text':
        return inline.text
      case 'code':
        return (
          <code key={i} className="kb-md__code">
            {inline.text}
          </code>
        )
      case 'strong':
        return <strong key={i}>{renderInlines(inline.children)}</strong>
      case 'em':
        return <em key={i}>{renderInlines(inline.children)}</em>
      case 'link':
        return (
          <a key={i} href={inline.href} target="_blank" rel="noopener noreferrer">
            {renderInlines(inline.children)}
          </a>
        )
    }
  })
}
