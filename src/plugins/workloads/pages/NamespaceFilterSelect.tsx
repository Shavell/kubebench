import { ALL_NAMESPACES } from '@/core/store/contextStore'
import { NamespaceOptions } from '@/shared/NamespaceOptions'
import { useWorkloadsTranslation } from '../i18n'
import { LOCAL_ALL_NAMESPACES } from './namespaceFilter'

/** The local namespace filter of a list page (see namespaceFilter.ts). */
export function NamespaceFilterSelect({
  value,
  onChange,
  globalNamespace,
  namespaces,
}: {
  value: string
  onChange: (value: string) => void
  globalNamespace: string
  namespaces: readonly string[]
}) {
  const { t } = useWorkloadsTranslation()
  const followsAll = globalNamespace === ALL_NAMESPACES
  return (
    <select title={t('filters.namespaceTitle')} value={value} onChange={(e) => onChange(e.target.value)}>
      <option value="">
        {followsAll ? t('filters.namespaceAll') : t('filters.namespaceGlobal', { namespace: globalNamespace })}
      </option>
      {(!followsAll || value === LOCAL_ALL_NAMESPACES) && (
        <option value={LOCAL_ALL_NAMESPACES}>{t('filters.namespaceAll')}</option>
      )}
      <NamespaceOptions names={namespaces} selected={value} />
    </select>
  )
}
