import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import type { TabDescriptor } from '@/core/plugin/types'
import { usePluginQuery } from '@/data/pluginData'
import { Badge } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { EmptyState } from '@/shared/EmptyState'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { accessData } from '../data'
import { RBAC_SCOPE } from '../data/topics'
import type { GrantPath, ScopedRule, Subject, SubjectKind } from '../data/types'
import { NS } from '../i18n'
import { ruleGroups, ruleTargets, subjectName } from '../model/format'
import { openResource } from '../tabs'
import { CanIBox } from './CanIBox'
import { Mono, PathCell } from './parts'
import '@/shared/tabPage.css'
import '../access.css'

const pathKey = (p: GrantPath) => `${p.binding.kind}/${p.binding.namespace ?? ''}/${p.binding.name}`

/** One subject: its identity, the bindings that reach it, effective rules by namespace and a quick can-i. */
export function SubjectPage({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const props = (tab.props ?? {}) as { kind?: SubjectKind; name?: string; namespace?: string }
  const kind = props.kind ?? 'User'
  const name = props.name ?? ''
  const namespace = props.namespace
  const subject = useMemo<Subject>(() => ({ kind, name, namespace }), [kind, name, namespace])
  const query = usePluginQuery(() => accessData().getSubject(subject), [subject], RBAC_SCOPE)

  const bindingColumns = useMemo<Column<GrantPath>[]>(
    () => [
      { key: 'path', header: t('columns.grantedBy'), render: (p) => <PathCell path={p} compact /> },
      {
        key: 'scope',
        header: t('columns.scope'),
        sortValue: (p) => p.scope,
        render: (p) => (p.scope ? p.scope : <Badge tone="info">{t('paths.clusterWide')}</Badge>),
      },
      { key: 'via', header: t('columns.via'), render: (p) => (p.via ? <span className="access-mono">{p.via}</span> : t('subject.direct')) },
    ],
    [t],
  )

  const ruleColumns = useMemo<Column<ScopedRule>[]>(
    () => [
      { key: 'verbs', header: t('columns.verbs'), render: (r) => <Mono values={r.verbs} /> },
      { key: 'groups', header: t('columns.apiGroups'), render: (r) => <span className="access-mono">{ruleGroups(r, t('rules.coreGroup'))}</span> },
      { key: 'targets', header: t('columns.resources'), render: (r) => <span className="access-mono">{ruleTargets(r)}</span> },
      { key: 'from', header: t('columns.from'), render: (r) => <PathCell path={r.path} compact /> },
    ],
    [t],
  )

  const title = t('subject.title', { kind: t(`kinds.${kind}`), name: subjectName(subject) })
  const actions =
    kind === 'ServiceAccount' ? (
      <Button variant="secondary" icon="go-to-file" onClick={() => openResource('ServiceAccount', namespace, name)}>
        {t('subject.openServiceAccount')}
      </Button>
    ) : null

  if (query.loading && !query.data) return <div className="page-loading">{t('subject.loading')}</div>
  const detail = query.data
  if (!detail) {
    return (
      <div className="page">
        <Toolbar title={title}>{actions}</Toolbar>
        <Unavailable code={query.errorCode} reason={query.error} title={t('subject.unavailable')} />
      </div>
    )
  }

  return (
    <div className="page">
      <Toolbar title={title}>{actions}</Toolbar>
      <div className="page__scroll access-page">
        {detail.missing && <div className="access-note">{t('subject.missingServiceAccount')}</div>}
        <section>
          <h2 className="access-section-title">{t('subject.identity')}</h2>
          <dl className="access-identity">
            {detail.user && (
              <>
                <dt>{t('subject.user')}</dt>
                <dd className="access-mono">{detail.user}</dd>
              </>
            )}
            <dt>{t('subject.groups')}</dt>
            <dd>
              <Mono values={detail.groups} />
            </dd>
          </dl>
        </section>

        <section>
          <h2 className="access-section-title">{t('canI.title')}</h2>
          <p className="access-muted">{t('canI.intro')}</p>
          <CanIBox subject={subject} />
        </section>

        <section>
          <h2 className="access-section-title">{t('subject.bindings', { count: detail.bindings.length })}</h2>
          {detail.bindings.length ? (
            <Table columns={bindingColumns} rows={detail.bindings} rowKey={pathKey} />
          ) : (
            <EmptyState icon="shield" title={t('subject.noBindings')} hint={t('subject.noBindingsHint')} />
          )}
        </section>

        <section>
          <h2 className="access-section-title">{t('subject.rules')}</h2>
          {detail.rules.length === 0 && <EmptyState icon="shield" title={t('subject.noRules')} />}
          {detail.rules.map((group) => (
            <div key={group.namespace || '-'} className="access-scope">
              <h3 className="access-scope__title">
                {group.namespace ? t('subject.rulesIn', { namespace: group.namespace }) : t('subject.rulesClusterWide')}
              </h3>
              <Table columns={ruleColumns} rows={group.rules} rowKey={(r) => `${pathKey(r.path)}#${group.rules.indexOf(r)}`} />
            </div>
          ))}
        </section>
      </div>
    </div>
  )
}
