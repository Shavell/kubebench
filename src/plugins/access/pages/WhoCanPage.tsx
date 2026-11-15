import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePluginQuery } from '@/data/pluginData'
import { Button } from '@/shared/Button'
import { EmptyState } from '@/shared/EmptyState'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { accessData } from '../data'
import type { AccessRequest, Grant, SubjectGrants } from '../data/types'
import { NS } from '../i18n'
import { requestTarget, ruleSummary, subjectKey } from '../model/format'
import { EMPTY_DRAFT, requestOf, type RequestDraft } from '../model/request'
import { AccessRequestForm } from './AccessRequestForm'
import { PathCell, SubjectCell } from './parts'
import '@/shared/tabPage.css'
import '../access.css'

/** Common questions, one click away. */
const PRESETS: { id: string; draft: Partial<RequestDraft> }[] = [
  { id: 'readSecrets', draft: { verb: 'list', group: '', resource: 'secrets', subresource: '' } },
  { id: 'execPods', draft: { verb: 'create', group: '', resource: 'pods', subresource: 'exec' } },
  { id: 'createPods', draft: { verb: 'create', group: '', resource: 'pods', subresource: '' } },
  { id: 'impersonate', draft: { verb: 'impersonate', group: '', resource: 'serviceaccounts', subresource: '' } },
  { id: 'bindRoles', draft: { verb: 'bind', group: 'rbac.authorization.k8s.io', resource: 'clusterroles', subresource: '' } },
  { id: 'deleteNamespaces', draft: { verb: 'delete', group: '', resource: 'namespaces', subresource: '' } },
]

/** Who may perform a request, and through which binding (design §7.8). */
export function WhoCanPage() {
  const { t } = useTranslation(NS)
  const [draft, setDraft] = useState<RequestDraft>(EMPTY_DRAFT)
  const [asked, setAsked] = useState<AccessRequest>(() => requestOf(EMPTY_DRAFT))
  const query = usePluginQuery(() => accessData().whoCan(asked), [asked])

  const ask = (next: RequestDraft) => {
    setDraft(next)
    setAsked(requestOf(next))
  }

  const columns = useMemo<Column<SubjectGrants>[]>(
    () => [
      {
        key: 'subject',
        header: t('columns.subject'),
        sortValue: (s) => s.name,
        render: (s) => <SubjectCell subject={s} />,
      },
      { key: 'kind', header: t('columns.kind'), sortValue: (s) => s.kind, render: (s) => t(`kinds.${s.kind}`) },
      {
        key: 'grants',
        header: t('columns.grantedBy'),
        render: (s) => (
          <div className="access-stack">
            {s.grants.map((g: Grant) => (
              <div key={`${g.binding.kind}/${g.binding.namespace ?? ''}/${g.binding.name}`} className="access-grant">
                <PathCell path={g} />
                <span className="access-rule" title={ruleSummary(g.rule)}>
                  {t('whoCan.rule', { rule: ruleSummary(g.rule) })}
                </span>
              </div>
            ))}
          </div>
        ),
      },
    ],
    [t],
  )

  const results = () => {
    if (query.loading && !query.data) return <div className="page-loading">{t('whoCan.loading')}</div>
    if (query.error) return <Unavailable compact code={query.errorCode} reason={query.error} title={t('whoCan.unavailable')} />
    const subjects = query.data?.subjects ?? []
    if (!subjects.length) return <EmptyState icon="shield" title={t('whoCan.nobody')} hint={t('whoCan.nobodyHint')} />
    return <Table columns={columns} rows={subjects} rowKey={subjectKey} />
  }

  return (
    <div className="page">
      <Toolbar title={t('whoCan.title')} />
      <div className="page__scroll access-page">
        <p className="access-muted">{t('whoCan.intro')}</p>
        <AccessRequestForm draft={draft} onChange={setDraft} onSubmit={() => ask(draft)} submitLabel={t('whoCan.ask')} busy={query.loading} />
        <div className="access-presets" role="group" aria-label={t('whoCan.presets')}>
          {PRESETS.map((p) => (
            <Button key={p.id} variant="secondary" onClick={() => ask({ ...draft, mode: 'resource', name: '', ...p.draft })}>
              {t(`whoCan.preset.${p.id}`)}
            </Button>
          ))}
        </div>
        <h2 className="access-section-title">
          {t('whoCan.resultTitle', {
            verb: asked.verb,
            target: requestTarget(asked),
            scope: asked.namespace ? t('paths.inNamespace', { namespace: asked.namespace }) : t('paths.clusterWide'),
          })}
          {query.data && <span className="access-muted"> · {t('whoCan.count', { count: query.data.subjects.length })}</span>}
        </h2>
        {results()}
      </div>
    </div>
  )
}
