import { useTranslation } from 'react-i18next'
import { usePanelRequest } from '@/core/plugin/panelRequests'
import { EmptyState } from '@/shared/EmptyState'
import { NS } from '../i18n'
import { LogsView } from '../logview/LogsView'
import { podTarget } from './podTarget'

/**
 * Bottom-panel Logs: follows the container of the latest `pod-logs`
 * request (e.g. a pod row's "Logs" action); each request starts a fresh
 * view.
 */
export function LogsPanel() {
  const { t } = useTranslation(NS)
  const request = usePanelRequest('pod-logs')
  const target = podTarget(request?.props)

  if (!request || !target) return <EmptyState icon="output" title={t('logs.noTarget')} hint={t('logs.hint')} />
  return <LogsView key={request.seq} target={{ kind: 'pod', ...target }} compact />
}
