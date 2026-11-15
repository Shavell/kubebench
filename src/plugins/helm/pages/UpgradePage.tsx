import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import type { TabDescriptor } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { refreshData } from '@/data/DataContext'
import { usePluginQuery } from '@/data/pluginData'
import { Button } from '@/shared/Button'
import { CodeEditor } from '@/shared/editor/CodeEditor'
import { DiffEditor } from '@/shared/editor/DiffEditor'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Checkbox, Select } from '@/shared/form'
import { notify } from '@/shared/notify'
import { Tabs } from '@/shared/Tabs'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { helmData, type HelmChartResult, type HelmUpgradeRequest, type HelmUpgradeResult } from '../data'
import { NS } from '../i18n'
import { useTabSession } from '../sessions'
import { openReleaseTab } from '../tabs'
import '@/shared/tabPage.css'
import '../helm.css'

interface UpgradeSession {
  /** Repository to take the chart from; "" keeps the release's current chart. */
  repo: string
  version: string
  /** The values being edited; null until the release's values are loaded. */
  values: string | null
  reuseValues: boolean
  view: 'values' | 'diff' | 'notes'
  /** The last dry run, for the inputs it was made with (any change clears it). */
  result: HelmUpgradeResult | null
}

/**
 * Upgrade of one release (design §7.4): pick the chart version from an
 * added repository (or keep the current chart), edit the values, review
 * the dry run's manifest diff, then upgrade.
 */
export function UpgradePage({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const namespace = tab.props?.namespace as string
  const name = tab.props?.name as string
  const write = usePermission('write')
  const closeTab = useLayoutStore((s) => s.closeTab)
  const [session, update] = useTabSession<UpgradeSession>(tab.id, () => ({
    repo: '',
    version: '',
    values: null,
    reuseValues: false,
    view: 'values',
    result: null,
  }))
  const [busy, setBusy] = useState<'dryRun' | 'upgrade' | null>(null)
  const [failure, setFailure] = useState<unknown>(null)

  const release = usePluginQuery(() => helmData().getRelease(namespace, name), [namespace, name])
  const currentValues = usePluginQuery(() => helmData().getValues(namespace, name), [namespace, name])
  const chartName = release.data?.chart
  const charts = usePluginQuery(
    async () => (chartName ? (await helmData().searchCharts(chartName)).filter((c) => c.name === chartName) : []),
    [chartName],
  )

  // The editor starts from the release's current user values.
  useEffect(() => {
    if (session.values === null && currentValues.data !== null) update({ values: currentValues.data })
  }, [session.values, currentValues.data, update])

  if (!release.data) {
    if (release.error) return <Unavailable code={release.errorCode} reason={release.error} onRetry={refreshData} />
    return <div className="page-loading">{t('release.loading', { name: `${namespace}/${name}` })}</div>
  }
  const current = release.data
  const sources: HelmChartResult[] = charts.data ?? []
  const source = sources.find((c) => c.repo === session.repo)
  const values = session.values ?? ''

  const change = (patch: Partial<UpgradeSession>) => {
    setFailure(null)
    update({ ...patch, result: null, view: 'values' })
  }

  const request = (dryRun: boolean): HelmUpgradeRequest => ({
    chart: source ? { repo: source.repo, name: source.name, version: session.version || source.version } : undefined,
    values,
    reuseValues: session.reuseValues,
    dryRun,
  })

  const run = async (dryRun: boolean) => {
    setBusy(dryRun ? 'dryRun' : 'upgrade')
    setFailure(null)
    try {
      const result = await helmData().upgrade(namespace, name, request(dryRun))
      if (dryRun) {
        update({ result, view: 'diff' })
      } else {
        notify.success(t('upgrade.done', { name, revision: result.release.revision }))
        refreshData()
        openReleaseTab(namespace, name)
        closeTab(tab.id)
      }
    } catch (err) {
      setFailure(err)
    } finally {
      setBusy(null)
    }
  }

  const sourceOptions = [
    { value: '', label: t('upgrade.currentChart', { chart: `${current.chart}-${current.chartVersion}` }) },
    ...sources.map((c) => ({ value: c.repo, label: `${c.repo}/${c.name}` })),
  ]
  const versionOptions = (source?.versions ?? []).map((v) => ({
    value: v.version,
    label:
      v.version === current.chartVersion
        ? t('upgrade.versionCurrent', { version: v.version, appVersion: v.appVersion })
        : t('upgrade.versionOption', { version: v.version, appVersion: v.appVersion }),
  }))
  const result = session.result

  return (
    <div className="page">
      <Toolbar title={t('upgrade.title', { name: `${namespace}/${name}` })}>
        <Button
          variant="secondary"
          icon="beaker"
          busy={busy === 'dryRun'}
          disabled={!write.allowed || busy !== null || session.values === null}
          title={write.reason ?? undefined}
          onClick={() => void run(true)}
        >
          {t('upgrade.dryRun')}
        </Button>
        <Button
          icon="arrow-circle-up"
          busy={busy === 'upgrade'}
          disabled={!write.allowed || busy !== null || session.values === null}
          title={write.reason ?? undefined}
          onClick={() => void run(false)}
        >
          {t('upgrade.upgrade')}
        </Button>
      </Toolbar>
      <div className="helm-pane__controls">
        <label>
          {t('upgrade.chart')}
          <Select
            value={session.repo}
            options={sourceOptions}
            onChange={(repo) => change({ repo, version: sources.find((c) => c.repo === repo)?.version ?? '' })}
          />
        </label>
        {source && (
          <label>
            {t('upgrade.version')}
            <Select value={session.version || source.version} options={versionOptions} onChange={(version) => change({ version })} />
          </label>
        )}
        <Checkbox
          checked={session.reuseValues}
          onChange={(reuseValues) => change({ reuseValues })}
          label={t('upgrade.reuseValues')}
          title={t('upgrade.reuseValuesHint')}
        />
        {charts.data && sources.length === 0 && <span className="helm-muted">{t('upgrade.noRepoChart', { chart: current.chart })}</span>}
      </div>
      {failure !== null && (
        <div className="helm-pane__controls">
          <ErrorNotice error={failure} />
        </div>
      )}
      <div className="helm-detail">
        <Tabs
          tabs={[
            { id: 'values' as const, label: t('upgrade.tabs.values'), icon: 'settings' },
            { id: 'diff' as const, label: t('upgrade.tabs.diff'), icon: 'diff', disabled: !result },
            { id: 'notes' as const, label: t('upgrade.tabs.notes'), icon: 'note', disabled: !result },
          ]}
          active={result ? session.view : 'values'}
          onChange={(view) => update({ view })}
        >
          {(!result || session.view === 'values') && (
            <div className="helm-pane">
              <div className="helm-pane__controls helm-muted">
                {session.reuseValues ? t('upgrade.valuesMergeHint') : t('upgrade.valuesReplaceHint')}
              </div>
              <div className="helm-pane__editor">
                {session.values === null && currentValues.error ? (
                  <Unavailable compact code={currentValues.errorCode} reason={currentValues.error} />
                ) : session.values === null ? (
                  <div className="page-loading">{t('release.values.loading')}</div>
                ) : (
                  <CodeEditor value={values} onChange={(text) => change({ values: text })} ariaLabel={t('upgrade.valuesAria', { name })} />
                )}
              </div>
            </div>
          )}
          {result && session.view === 'diff' && (
            <div className="helm-pane">
              <div className="helm-pane__controls helm-muted">
                {result.manifest === result.currentManifest
                  ? t('upgrade.noChanges')
                  : t('upgrade.diffHint', { from: current.revision, to: result.release.revision })}
              </div>
              <div className="helm-pane__editor">
                <DiffEditor
                  original={result.currentManifest}
                  modified={result.manifest}
                  originalTitle={t('upgrade.currentManifest', { revision: current.revision })}
                  modifiedTitle={t('upgrade.newManifest')}
                />
              </div>
            </div>
          )}
          {result && session.view === 'notes' && <pre className="helm-notes">{result.notes || t('release.notes.none')}</pre>}
        </Tabs>
      </div>
    </div>
  )
}
