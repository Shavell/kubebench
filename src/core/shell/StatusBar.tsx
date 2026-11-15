import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { registry } from '@/core/plugin/registry'
import { useRegistryRevision } from '@/core/plugin/useRegistry'
import { ClusterItem } from './status/ClusterItem'
import { NamespaceItem } from './status/NamespaceItem'
import { ReadOnlyItem } from './status/ReadOnlyItem'
import { LiveItem } from './status/LiveItem'
import { DataSourceItem, UserItem, VersionItem } from './status/SessionItems'
import './StatusBar.css'

/**
 * Bottom status bar: the active cluster (coloured by environment), a
 * read-only marker and the namespace filter on the left; plugin items, live
 * updates, the data source (demo mode), the version and the user menu on
 * the right.
 */
export function StatusBar() {
  const { t } = useTranslation(PLATFORM_NS)
  // Plugin items follow plugins being enabled or disabled.
  useRegistryRevision()

  return (
    <footer className="status-bar" aria-label={t('statusBar.label')}>
      <div className="status-bar__side">
        <ClusterItem />
        <ReadOnlyItem />
        <NamespaceItem />
        {registry.statusBarItems('left').map((item) => (
          <item.component key={`${item.pluginId}/${item.id}`} />
        ))}
      </div>
      <div className="status-bar__side">
        {registry.statusBarItems('right').map((item) => (
          <item.component key={`${item.pluginId}/${item.id}`} />
        ))}
        <LiveItem />
        <DataSourceItem />
        <VersionItem />
        <UserItem />
      </div>
    </footer>
  )
}
