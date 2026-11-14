import { useTranslation } from 'react-i18next'
import { useProviderQuery } from '@/data/DataContext'
import { Button } from '@/shared/Button'
import { Field, Select, TextInput, type SelectOption } from '@/shared/form'
import { AUDIT_NS } from './namespace'
import type { AuditFilters } from './useAuditLog'

interface AuditFiltersBarProps {
  filters: AuditFilters
  onChange: (filters: AuditFilters) => void
}

/** Choices for an exact-match filter: "all", the known values, and the current one if it is gone. */
function choices(all: string, known: string[], current: string): SelectOption<string>[] {
  const values = [...new Set([...known, ...(current ? [current] : [])])].sort((a, b) => a.localeCompare(b))
  return [{ value: '', label: all }, ...values.map((value) => ({ value, label: value }))]
}

/**
 * User and cluster are exact matches on the server, so they are picked
 * from the known users and clusters; the action matches any part.
 */
export function AuditFiltersBar({ filters, onChange }: AuditFiltersBarProps) {
  const { t } = useTranslation(AUDIT_NS)
  const users = useProviderQuery((p) => p.users.listUsers(), [])
  const clusters = useProviderQuery((p) => p.inventory.listClusters(), [])
  const set = (patch: Partial<AuditFilters>) => onChange({ ...filters, ...patch })
  const active = Boolean(filters.user || filters.cluster || filters.action)

  return (
    <div className="audit-filters" role="search">
      <Field label={t('filters.user')}>
        <Select
          value={filters.user}
          onChange={(user) => set({ user })}
          options={choices(t('filters.allUsers'), (users.data ?? []).map((u) => u.username), filters.user)}
        />
      </Field>
      <Field label={t('filters.cluster')}>
        <Select
          value={filters.cluster}
          onChange={(cluster) => set({ cluster })}
          options={choices(t('filters.allClusters'), (clusters.data ?? []).map((c) => c.name), filters.cluster)}
        />
      </Field>
      <Field label={t('filters.action')}>
        <TextInput
          type="search"
          value={filters.action}
          onChange={(action) => set({ action })}
          placeholder={t('filters.actionPlaceholder')}
          monospace
        />
      </Field>
      <Button variant="secondary" icon="clear-all" disabled={!active} onClick={() => onChange({ user: '', cluster: '', action: '' })}>
        {t('filters.clear')}
      </Button>
    </div>
  )
}
