import clsx from 'clsx'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviders } from '@/data/DataContext'
import { useDataMode } from '@/data/mode'
import type { ClusterEntry, ClusterEnvironment } from '@/data/types'
import { i18n } from '@/i18n'
import { ProviderBadge } from '@/shared/cloud/ProviderBadge'
import { platformLabel, platformTooltip } from '@/shared/cloud/platformText'
import { Icon } from '@/shared/Icon'
import { notify } from '@/shared/notify'
import { StatusMenu, type StatusMenuEntry } from './StatusMenu'
import { useMenuAnchor } from './useMenuAnchor'

/** The Clusters view's inventory tab (tab kind of the clusters plugin). */
function openInventory(): void {
  useLayoutStore.getState().openTab({
    id: 'clusters-inventory',
    title: i18n.t('statusBar.cluster.inventoryTab', { ns: PLATFORM_NS }),
    titleKey: 'statusBar.cluster.inventoryTab',
    titleNs: PLATFORM_NS,
    icon: 'server-environment',
    kind: 'clusters-inventory',
  })
}

/**
 * The active cluster, coloured by its environment (production red, staging
 * amber, development green; design §6.3). Opens a menu to switch clusters,
 * which goes through the server and reloads the app.
 */
export function ClusterItem() {
  const { t } = useTranslation(PLATFORM_NS)
  const { cluster, server, clusterError } = useDataMode()
  const providers = useProviders()
  const { anchor, toggle, close } = useMenuAnchor()
  const [clusters, setClusters] = useState<ClusterEntry[] | null>(null)

  // The inventory is read when the menu opens, not on every refresh; an
  // opening menu shows the loading state until it arrives.
  const [openedFor, setOpenedFor] = useState(anchor)
  if (openedFor !== anchor) {
    setOpenedFor(anchor)
    if (anchor) setClusters(null)
  }
  useEffect(() => {
    if (!anchor) return
    let cancelled = false
    providers.inventory.listClusters().then(
      (list) => !cancelled && setClusters(list),
      (err: unknown) => {
        if (cancelled) return
        setClusters([])
        notify.error(err)
      },
    )
    return () => {
      cancelled = true
    }
  }, [anchor, providers])

  const envLabel = (environment: ClusterEnvironment | undefined) =>
    environment ? t(`environment.${environment}`) : ''

  const activate = (name: string) => {
    notify.info(t('statusBar.cluster.switching', { name }))
    providers.inventory.activateCluster(name).then(
      () => window.location.reload(),
      (err: unknown) => notify.error(err),
    )
  }

  const clusterEntries: StatusMenuEntry[] = clusters
    ? clusters.map((c) => ({
        id: `cluster:${c.name}`,
        label: c.name,
        icon: 'vm',
        checked: c.active,
        // Address entries are bookmarks without credentials.
        disabled: c.source === 'address' || Boolean(c.error),
        detail: c.error
          ? t('statusBar.cluster.broken')
          : c.source === 'address'
            ? t('statusBar.cluster.bookmark')
            : [c.platform && platformLabel(t, c.platform), envLabel(c.environment)].filter(Boolean).join(' · '),
        onSelect: () => !c.active && activate(c.name),
      }))
    : [{ id: 'loading', label: t('statusBar.cluster.loading'), disabled: true, onSelect: () => {} }]

  const title = cluster
    ? [
        t('statusBar.cluster.title', { name: cluster.name }),
        envLabel(cluster.environment),
        cluster.platform ? platformTooltip(t, cluster.platform) : '',
        server,
        t('statusBar.cluster.switchHint'),
      ]
        .filter(Boolean)
        .join('\n')
    : clusterError || t('statusBar.cluster.noneHint')

  return (
    <>
      <button
        type="button"
        className={clsx(
          'status-bar__item status-bar__item--button',
          cluster?.environment && `status-bar__item--env-${cluster.environment}`,
        )}
        title={title}
        aria-haspopup="menu"
        aria-expanded={Boolean(anchor)}
        onClick={toggle}
      >
        <Icon name={cluster ? 'server-environment' : 'warning'} size={13} />
        <span>{cluster ? cluster.name : t('statusBar.cluster.none')}</span>
        <ProviderBadge platform={cluster?.platform} compact />
        {cluster?.environment && <span className="status-bar__env">{envLabel(cluster.environment)}</span>}
      </button>
      {anchor && (
        <StatusMenu
          anchor={anchor}
          label={t('statusBar.cluster.menu')}
          header={t('statusBar.cluster.menu')}
          entries={[
            ...clusterEntries,
            { type: 'separator', id: 'sep' },
            {
              id: 'manage',
              label: t('statusBar.cluster.manage'),
              icon: 'settings',
              onSelect: openInventory,
            },
          ]}
          onClose={close}
        />
      )}
    </>
  )
}
