import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import type { TabDescriptor } from '@/core/plugin/types'
import { refreshData } from '@/data/DataContext'
import { usePluginQuery } from '@/data/pluginData'
import { EmptyState } from '@/shared/EmptyState'
import { Unavailable } from '@/shared/Unavailable'
import { serviceBrowserData } from '../data'
import type { BrowserTarget, Scheme } from '../data/types'
import { NS } from '../i18n'
import type { BrowserTabProps } from '../tabs'
import { BrowserSession } from './BrowserSession'
import '../serviceBrowser.css'

/**
 * A browser tab (design §7.14): picks the port of the tab's Service or
 * Pod, then shows it through a browsing session. Changing the port or the
 * scheme starts a new session.
 */
export function BrowserPage({ tab }: { tab: TabDescriptor }) {
  const { kind, namespace, name } = tab.props as unknown as BrowserTabProps
  const { t } = useTranslation(NS)
  const permission = usePermission('browse')
  const ports = usePluginQuery(
    () => serviceBrowserData().listPorts(kind, namespace, name),
    [kind, namespace, name],
  )
  const [choice, setChoice] = useState<{ port: number; scheme: Scheme } | null>(null)

  if (!permission.allowed) return <Unavailable code={permission.code} reason={permission.reason} />
  if (ports.error) return <Unavailable code={ports.errorCode} reason={ports.error} onRetry={refreshData} />
  if (!ports.data) return <div className="page-loading">{t('page.loadingPorts')}</div>
  if (ports.data.length === 0) {
    return <EmptyState icon="plug" title={t('page.noPorts')} hint={t('page.noPortsHint')} />
  }

  const selected = ports.data.find((p) => p.port === choice?.port) ?? ports.data[0]
  const scheme = choice?.port === selected.port ? choice.scheme : selected.scheme
  const target: BrowserTarget = { kind, namespace, name, port: String(selected.port), scheme }

  return (
    <BrowserSession
      key={`${target.port}/${target.scheme}`}
      target={target}
      ports={ports.data}
      onTargetChange={(port, nextScheme) => setChoice({ port, scheme: nextScheme })}
    />
  )
}
