import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import { usePluginQuery } from '@/data/pluginData'
import { CORE_NS } from '@/i18n'
import { Badge } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { DiffEditor } from '@/shared/editor/DiffEditor'
import { Unavailable } from '@/shared/Unavailable'
import { helmData, type HelmReleaseDetail } from '../../data'
import { DateTime } from '../../DateTime'
import { NS } from '../../i18n'
import { RevisionSelect } from './RevisionSelect'

/**
 * History tab: every revision with a rollback action, and the user values
 * of two revisions side by side.
 */
export function HistoryTab({
  release,
  onRollback,
}: {
  release: HelmReleaseDetail
  onRollback: (revision: number) => void
}) {
  const { t } = useTranslation(NS)
  const write = usePermission('write')
  const { history, namespace, name } = release
  // A release that is uninstalled (kept history) can roll back to its own latest revision.
  const uninstalled = release.status === 'uninstalled'

  return (
    <div className="helm-pane__scroll">
      <table className="kb-table" style={{ maxWidth: 1200 }}>
        <thead>
          <tr>
            <th>{t('release.history.revision')}</th>
            <th>{t('common.status', { ns: CORE_NS })}</th>
            <th>{t('release.history.chart')}</th>
            <th>{t('release.history.appVersion')}</th>
            <th>{t('release.history.updated')}</th>
            <th>{t('release.history.description')}</th>
            <th aria-label={t('common.actions', { ns: CORE_NS })}></th>
          </tr>
        </thead>
        <tbody>
          {history.map((h) => (
            <tr key={h.revision}>
              <td>{h.revision}</td>
              <td>
                <Badge>{h.status}</Badge>
              </td>
              <td>{`${h.chart}-${h.chartVersion}`}</td>
              <td>{h.appVersion}</td>
              <td>
                <DateTime value={h.updated} />
              </td>
              <td>{h.description}</td>
              <td>
                {(h.revision !== release.revision || uninstalled) && (
                  <Button
                    variant="secondary"
                    icon="history"
                    disabled={!write.allowed}
                    title={write.allowed ? `helm rollback ${name} ${h.revision} -n ${namespace}` : (write.reason ?? undefined)}
                    onClick={() => onRollback(h.revision)}
                  >
                    {t('release.history.rollback')}
                  </Button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {history.length > 1 && <ValuesComparison release={release} />}
    </div>
  )
}

/** The user values of two revisions, older on the left. */
function ValuesComparison({ release }: { release: HelmReleaseDetail }) {
  const { t } = useTranslation(NS)
  const { history, namespace, name } = release
  const [from, setFrom] = useState(history[1].revision)
  const [to, setTo] = useState(history[0].revision)
  const values = usePluginQuery(async () => {
    const data = helmData()
    const [left, right] = await Promise.all([
      data.getValues(namespace, name, { revision: from }),
      data.getValues(namespace, name, { revision: to }),
    ])
    return { left, right }
  }, [namespace, name, from, to])

  return (
    <div className="helm-compare">
      <div className="detail__section">{t('release.compare.title')}</div>
      <div className="helm-pane__controls">
        <RevisionSelect history={history} value={from} onChange={setFrom} label={t('release.compare.from')} />
        <RevisionSelect history={history} value={to} onChange={setTo} label={t('release.compare.to')} />
      </div>
      {values.error ? (
        <Unavailable compact code={values.errorCode} reason={values.error} />
      ) : values.data === null ? (
        <div className="helm-muted">{t('release.values.loading')}</div>
      ) : values.data.left === values.data.right ? (
        <div className="helm-muted">{t('release.compare.same', { from, to })}</div>
      ) : (
        <DiffEditor
          className="helm-compare__editor"
          height={360}
          original={values.data.left}
          modified={values.data.right}
          originalTitle={t('release.compare.revision', { revision: from })}
          modifiedTitle={t('release.compare.revision', { revision: to })}
        />
      )}
    </div>
  )
}
