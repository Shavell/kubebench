import { useTranslation } from 'react-i18next'
import { Select } from '@/shared/form'
import type { HelmRevision } from '../../data'
import { NS } from '../../i18n'

/** Picks a revision of the history (newest first); 0 stands for the latest. */
export function RevisionSelect({
  history,
  value,
  onChange,
  label,
}: {
  history: HelmRevision[]
  value: number
  onChange: (revision: number) => void
  label: string
}) {
  const { t } = useTranslation(NS)
  const latest = history[0]?.revision ?? 0
  const options = history.map((h) => ({
    value: h.revision,
    label:
      h.revision === latest
        ? t('release.revisionLatest', { revision: h.revision, status: h.status })
        : t('release.revisionOption', { revision: h.revision, status: h.status }),
  }))
  return (
    <label>
      {label}
      <Select value={value || latest} onChange={onChange} options={options} />
    </label>
  )
}
