import { useState, type ReactNode } from 'react'
import clsx from 'clsx'
import { Icon } from './Icon'
import './Tree.css'

export interface TreeNode {
  id: string
  label: string
  /** Muted text after the label, like VSCode's tree item descriptions (e.g. a namespace). */
  description?: string
  icon?: string
  /** Small count bubble on the right, like VSCode badge counts. */
  count?: number
  children?: TreeNode[]
  /** Extra content rendered right-aligned (e.g. a status dot). */
  suffix?: ReactNode
}

interface TreeProps {
  nodes: TreeNode[]
  selectedId?: string | null
  onSelect?: (node: TreeNode) => void
  /** Node ids expanded on first render. */
  defaultExpanded?: string[]
}

export function Tree({ nodes, selectedId, onSelect, defaultExpanded }: TreeProps) {
  const [expanded, setExpanded] = useState<Set<string>>(
    () => new Set(defaultExpanded ?? nodes.map((n) => n.id)),
  )

  const toggle = (id: string) =>
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  const renderNode = (node: TreeNode, depth: number): ReactNode => {
    const hasChildren = !!node.children?.length
    const isExpanded = expanded.has(node.id)
    return (
      <div key={node.id}>
        <div
          className={clsx('kb-tree__row', selectedId === node.id && 'kb-tree__row--selected')}
          style={{ paddingLeft: 8 + depth * 14 }}
          onClick={() => {
            if (hasChildren) toggle(node.id)
            onSelect?.(node)
          }}
        >
          <span className="kb-tree__twistie">
            {hasChildren && <Icon name={isExpanded ? 'chevron-down' : 'chevron-right'} size={14} />}
          </span>
          {node.icon && <Icon name={node.icon} size={14} className="kb-tree__icon" />}
          <span className="kb-tree__label">
            {node.label}
            {node.description && <span className="kb-tree__description">{node.description}</span>}
          </span>
          {node.suffix}
          {node.count !== undefined && <span className="kb-tree__count">{node.count}</span>}
        </div>
        {hasChildren && isExpanded && node.children!.map((child) => renderNode(child, depth + 1))}
      </div>
    )
  }

  return <div className="kb-tree">{nodes.map((n) => renderNode(n, 0))}</div>
}
