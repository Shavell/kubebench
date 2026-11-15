import { useState } from 'react'
import { formatNumber } from '@/i18n/format'
import { Badge } from '@/shared/Badge'
import { formatAge, groupByReason, type EventRow } from '@/shared/eventModel'
import { Icon } from '@/shared/Icon'
import { Table } from '@/shared/Table'
import { useWorkloadsTranslation } from '../../i18n'
import { withHeaders } from '../../resourceCatalog'
import { groupedEventColumns } from './eventColumns'

interface EventGroupsProps {
  rows: EventRow[]
  onOpen: (row: EventRow) => void
  onContextMenu: (row: EventRow, e: React.MouseEvent) => void
}

/**
 * Events grouped by reason, warnings first: one collapsible section per
 * reason with its totals; expanding it lists the events.
 */
export function EventGroups({ rows, onOpen, onContextMenu }: EventGroupsProps) {
  const { t } = useWorkloadsTranslation()
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(new Set())
  const groups = groupByReason(rows)
  const columns = withHeaders(groupedEventColumns)

  const toggle = (reason: string) =>
    setExpanded((current) => {
      const next = new Set(current)
      if (!next.delete(reason)) next.add(reason)
      return next
    })

  return (
    <div className="event-groups">
      {groups.map((group) => {
        const open = expanded.has(group.reason)
        const bodyId = `event-group-${group.reason}`
        return (
          <section key={group.reason} className="event-group">
            <button
              type="button"
              className="event-group__header"
              aria-expanded={open}
              aria-controls={open ? bodyId : undefined}
              onClick={() => toggle(group.reason)}
            >
              <Icon name={open ? 'chevron-down' : 'chevron-right'} size={14} />
              <Badge tone={group.type === 'Warning' ? 'warning' : 'neutral'}>{group.type}</Badge>
              <span className="event-group__reason">{group.reason}</span>
              <span className="event-group__summary">
                {t('eventsView.groupSummary', {
                  count: group.events.length,
                  events: formatNumber(group.events.length),
                  occurrences: formatNumber(group.occurrences),
                  lastSeen: formatAge(group.lastSeenMinutes),
                })}
              </span>
            </button>
            {open && (
              <div id={bodyId} className="event-group__body">
                <Table
                  columns={columns}
                  rows={group.events}
                  rowKey={(row) => row.key}
                  onRowClick={onOpen}
                  onRowContextMenu={onContextMenu}
                />
              </div>
            )}
          </section>
        )
      })}
    </div>
  )
}
