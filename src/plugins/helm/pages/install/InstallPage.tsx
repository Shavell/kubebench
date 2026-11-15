import clsx from 'clsx'
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import type { TabDescriptor } from '@/core/plugin/types'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { useLayoutStore } from '@/core/store/layoutStore'
import { refreshData } from '@/data/DataContext'
import { Button } from '@/shared/Button'
import { CodeEditor } from '@/shared/editor/CodeEditor'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Icon } from '@/shared/Icon'
import { notify } from '@/shared/notify'
import { Tabs } from '@/shared/Tabs'
import { Toolbar } from '@/shared/Toolbar'
import { helmData } from '../../data'
import { NS } from '../../i18n'
import { useTabSession } from '../../sessions'
import { openReleaseTab } from '../../tabs'
import { ChartStep } from './ChartStep'
import { chartRefOf, initialSession, nameError, namespaceError, type InstallSession, type InstallStep } from './session'
import { ValuesStep } from './ValuesStep'
import '@/shared/tabPage.css'
import '../../helm.css'

const STEPS: InstallStep[] = ['chart', 'values', 'review']

/**
 * The install wizard (design §7.4): chart (repository search and version,
 * OCI reference or uploaded .tgz) → values from the chart defaults → dry
 * run (manifest and notes, nothing created) → install.
 */
export function InstallPage({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const write = usePermission('write')
  const closeTab = useLayoutStore((s) => s.closeTab)
  const contextNamespace = useContextStore((s) => s.namespace)
  const [session, update] = useTabSession<InstallSession>(tab.id, () =>
    initialSession(contextNamespace === ALL_NAMESPACES ? 'default' : contextNamespace),
  )
  const [busy, setBusy] = useState<'next' | 'dryRun' | 'install' | null>(null)
  const [failure, setFailure] = useState<unknown>(null)
  const [view, setView] = useState<'manifest' | 'notes'>('manifest')

  // Any change of the inputs makes the last dry run stale.
  const change = useCallback(
    (patch: Partial<InstallSession>) => {
      setFailure(null)
      update({ ...patch, result: null })
    },
    [update],
  )

  const attempt = async (what: 'next' | 'dryRun' | 'install', action: () => Promise<void>) => {
    setBusy(what)
    setFailure(null)
    try {
      await action()
    } catch (err) {
      setFailure(err)
    } finally {
      setBusy(null)
    }
  }

  const chart = chartRefOf(session)

  // Chart → values: load the chart's defaults as the starting values.
  const toValues = () =>
    attempt('next', async () => {
      if (!chart) return
      const loaded = await helmData().chartValues(chart)
      update({
        step: 'values',
        chart,
        chartInfo: loaded.chart,
        values: loaded.values,
        name: session.name || loaded.chart.name,
        result: null,
      })
    })

  // The values step installs the chart it loaded the defaults of.
  const run = (dryRun: boolean) => {
    if (!session.chart) throw new Error('no chart loaded')
    return helmData().install({
      name: session.name,
      namespace: session.namespace,
      createNamespace: session.createNamespace,
      chart: session.chart,
      values: session.values,
      dryRun,
    })
  }

  const dryRun = () =>
    attempt('dryRun', async () => {
      update({ result: await run(true), step: 'review' })
      setView('manifest')
    })

  const install = () =>
    attempt('install', async () => {
      const result = await run(false)
      notify.success(t('install.done', { name: result.release.name, namespace: result.release.namespace }))
      refreshData()
      openReleaseTab(result.release.namespace, result.release.name)
      closeTab(tab.id)
    })

  const stepIndex = STEPS.indexOf(session.step)
  const valuesReady = !nameError(session.name) && !namespaceError(session.namespace)
  // The server connects to whatever host an OCI reference names, so
  // loading one needs write, like the dry run and the install (§7.4).
  const nextDenied = chart !== null && 'ref' in chart && !write.allowed

  return (
    <div className="page">
      <Toolbar title={t('install.title')} />
      <ol className="helm-steps" aria-label={t('install.steps.label')}>
        {STEPS.map((step, i) => (
          <li
            key={step}
            className={clsx('helm-steps__step', i === stepIndex && 'helm-steps__step--active', i < stepIndex && 'helm-steps__step--done')}
            aria-current={i === stepIndex ? 'step' : undefined}
          >
            {i < stepIndex ? <Icon name="check" size={12} /> : <span>{i + 1}</span>}
            {t(`install.steps.${step}`)}
          </li>
        ))}
      </ol>
      <div className="helm-detail">
        {session.step === 'chart' && (
          <div className="helm-pane__scroll">
            <ChartStep session={session} change={change} />
          </div>
        )}
        {session.step === 'values' && <ValuesStep session={session} change={change} />}
        {session.step === 'review' && session.result && (
          <div className="helm-detail">
            <div className="helm-pane__controls helm-muted">
              {t('install.review.hint', { name: session.name, namespace: session.namespace })}
            </div>
            <Tabs
              tabs={[
                { id: 'manifest' as const, label: t('install.review.manifest'), icon: 'file-code' },
                { id: 'notes' as const, label: t('install.review.notes'), icon: 'note' },
              ]}
              active={view}
              onChange={setView}
            >
              {view === 'manifest' ? (
                <div className="helm-pane__editor">
                  <CodeEditor value={session.result.manifest} readOnly ariaLabel={t('install.review.manifest')} />
                </div>
              ) : (
                <pre className="helm-notes">{session.result.notes || t('release.notes.none')}</pre>
              )}
            </Tabs>
          </div>
        )}
      </div>
      {failure !== null && (
        <div className="helm-pane__controls">
          <ErrorNotice error={failure} />
        </div>
      )}
      <div className="helm-footer">
        {stepIndex > 0 && (
          <Button variant="secondary" icon="arrow-left" disabled={busy !== null} onClick={() => update({ step: STEPS[stepIndex - 1] })}>
            {t('install.back')}
          </Button>
        )}
        {session.step === 'chart' && (
          <Button
            icon="arrow-right"
            busy={busy === 'next'}
            disabled={!chart || nextDenied || busy !== null}
            title={nextDenied ? (write.reason ?? undefined) : undefined}
            onClick={() => void toValues()}
          >
            {t('install.next')}
          </Button>
        )}
        {session.step === 'values' && (
          <Button
            icon="beaker"
            busy={busy === 'dryRun'}
            disabled={!write.allowed || !valuesReady || busy !== null}
            title={write.reason ?? undefined}
            onClick={() => void dryRun()}
          >
            {t('install.dryRun')}
          </Button>
        )}
        {session.step === 'review' && (
          <Button
            icon="cloud-download"
            busy={busy === 'install'}
            disabled={!write.allowed || !session.result || busy !== null}
            title={write.reason ?? undefined}
            onClick={() => void install()}
          >
            {t('install.install')}
          </Button>
        )}
      </div>
    </div>
  )
}
