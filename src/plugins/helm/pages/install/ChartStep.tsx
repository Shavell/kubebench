import clsx from 'clsx'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import { usePluginQuery } from '@/data/pluginData'
import { Button } from '@/shared/Button'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Field, Select, TextInput } from '@/shared/form'
import { Unavailable } from '@/shared/Unavailable'
import { helmData } from '../../data'
import { NS } from '../../i18n'
import { openReposTab } from '../../tabs'
import type { ChartSource, InstallSession } from './session'

/** Charts are searched this long after the last keystroke. */
const SEARCH_DELAY_MS = 250

/** Step 1: a chart from a repository (search, then a version), an OCI registry, or an upload. */
export function ChartStep({
  session,
  change,
}: {
  session: InstallSession
  change: (patch: Partial<InstallSession>) => void
}) {
  const { t } = useTranslation(NS)
  const sources: { value: ChartSource; label: string }[] = [
    { value: 'repo', label: t('install.source.repo') },
    { value: 'oci', label: t('install.source.oci') },
    { value: 'upload', label: t('install.source.upload') },
  ]
  return (
    <>
      <div className="helm-form">
        <Field label={t('install.source.label')}>
          <Select value={session.source} options={sources} onChange={(source) => change({ source })} />
        </Field>
      </div>
      {session.source === 'repo' && <RepoChart session={session} change={change} />}
      {session.source === 'oci' && (
        <div className="helm-form" style={{ marginTop: 12 }}>
          <Field label={t('install.oci.ref')} help={t('install.oci.refHelp')} required>
            <TextInput
              value={session.ociRef}
              onChange={(ociRef) => change({ ociRef })}
              placeholder="oci://ghcr.io/stefanprodan/charts/podinfo"
              monospace
            />
          </Field>
          <Field label={t('install.oci.version')} help={t('install.oci.versionHelp')}>
            <TextInput value={session.ociVersion} onChange={(ociVersion) => change({ ociVersion })} placeholder="6.7.1" monospace />
          </Field>
        </div>
      )}
      {session.source === 'upload' && <UploadChart session={session} change={change} />}
    </>
  )
}

function RepoChart({ session, change }: { session: InstallSession; change: (patch: Partial<InstallSession>) => void }) {
  const { t } = useTranslation(NS)
  const [query, setQuery] = useState(session.query)
  useEffect(() => {
    if (query === session.query) return
    const timer = setTimeout(() => change({ query }), SEARCH_DELAY_MS)
    return () => clearTimeout(timer)
  }, [query, session.query, change])
  const repos = usePluginQuery(() => helmData().listRepos(), [])
  const results = usePluginQuery(() => helmData().searchCharts(session.query), [session.query])
  const selected = results.data?.find((c) => c.repo === session.selected?.repo && c.name === session.selected?.name)

  if (repos.data && repos.data.length === 0) {
    return (
      <div className="helm-summary">
        <span className="helm-muted">{t('install.repo.noRepos')}</span>
        <Button variant="secondary" icon="repo" onClick={openReposTab}>
          {t('install.repo.openRepos')}
        </Button>
      </div>
    )
  }

  return (
    <>
      <div className="helm-form" style={{ marginTop: 12 }}>
        <Field label={t('install.repo.search')}>
          <TextInput value={query} onChange={setQuery} placeholder={t('install.repo.searchPlaceholder')} type="search" />
        </Field>
        {selected && (
          <Field label={t('install.repo.version')}>
            <Select
              value={session.version || selected.version}
              options={selected.versions.map((v) => ({
                value: v.version,
                label: t('install.repo.versionOption', { version: v.version, appVersion: v.appVersion || '—' }),
              }))}
              onChange={(version) => change({ version })}
            />
          </Field>
        )}
      </div>
      {results.error ? (
        <Unavailable compact code={results.errorCode} reason={results.error} />
      ) : results.data && results.data.length === 0 ? (
        <div className="helm-summary helm-muted">{t('install.repo.noResults')}</div>
      ) : (
        <div className="helm-charts" role="listbox" aria-label={t('install.repo.results')}>
          {(results.data ?? []).map((c) => {
            const isSelected = c === selected
            return (
              <button
                key={`${c.repo}/${c.name}`}
                type="button"
                role="option"
                aria-selected={isSelected}
                className={clsx('helm-charts__item', isSelected && 'helm-charts__item--selected')}
                onClick={() => change({ selected: { repo: c.repo, name: c.name }, version: '' })}
              >
                <span className="helm-charts__name">{`${c.repo}/${c.name}`}</span>
                <span>{c.version}</span>
                <span className="helm-charts__description" title={c.description}>
                  {c.description}
                </span>
              </button>
            )
          })}
        </div>
      )}
    </>
  )
}

function UploadChart({ session, change }: { session: InstallSession; change: (patch: Partial<InstallSession>) => void }) {
  const { t } = useTranslation(NS)
  const write = usePermission('write')
  const input = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)

  const upload = async (file: File | undefined) => {
    if (!file) return
    setBusy(true)
    setFailure(null)
    try {
      change({ upload: await helmData().uploadChart(file) })
    } catch (err) {
      setFailure(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={{ marginTop: 12 }}>
      <div className="helm-summary">
        <Button
          variant="secondary"
          icon="cloud-upload"
          busy={busy}
          disabled={!write.allowed}
          title={write.reason ?? undefined}
          onClick={() => input.current?.click()}
        >
          {t('install.upload.pick')}
        </Button>
        <span className="helm-muted">{t('install.upload.hint')}</span>
        <input
          ref={input}
          type="file"
          accept=".tgz,.tar.gz,application/gzip,application/x-gzip"
          hidden
          onChange={(event) => {
            void upload(event.target.files?.[0])
            event.target.value = '' // the same file can be picked again
          }}
        />
      </div>
      {session.upload && (
        <div className="helm-summary">
          <span className="helm-charts__name">{`${session.upload.chart.name}-${session.upload.chart.version}`}</span>
          <span className="helm-muted">{session.upload.chart.description}</span>
        </div>
      )}
      <ErrorNotice error={failure} />
    </div>
  )
}
