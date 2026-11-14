import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { usePermissions } from '@/core/permissions'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { useProviderQuery } from '@/data/DataContext'
import { Icon } from '@/shared/Icon'

function NamespaceSelect() {
  const { t } = useTranslation(PLATFORM_NS)
  const namespace = useContextStore((s) => s.namespace)
  const setNamespace = useContextStore((s) => s.setNamespace)
  const { data } = useProviderQuery((p) => p.cluster.listNamespaces(), [])

  return (
    <label className="status-bar__item status-bar__item--select" title={t('statusBar.namespace.tooltip')}>
      <Icon name="folder" size={13} />
      <select
        className="status-bar__select"
        value={namespace}
        aria-label={t('statusBar.namespace.tooltip')}
        onChange={(e) => setNamespace(e.target.value)}
      >
        <option value={ALL_NAMESPACES}>{t('statusBar.namespace.all')}</option>
        {(data ?? []).map((ns) => (
          <option key={ns.name} value={ns.name}>
            {ns.name}
          </option>
        ))}
      </select>
    </label>
  )
}

/** Namespace filter of the list views; hidden while no cluster can be read. */
export function NamespaceItem() {
  const { canRead } = usePermissions()
  return canRead ? <NamespaceSelect /> : null
}
