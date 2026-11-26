import { useTranslation } from 'react-i18next'
import { useNamespaceNames } from '@/core/namespaceNames'
import { CodeEditor } from '@/shared/editor/CodeEditor'
import { Checkbox, Field, Select, TextInput } from '@/shared/form'
import { NS } from '../../i18n'
import { nameError, namespaceError, type InstallSession } from './session'

/** Step 2: release name, namespace and the values, starting from the chart defaults. */
export function ValuesStep({
  session,
  change,
}: {
  session: InstallSession
  change: (patch: Partial<InstallSession>) => void
}) {
  const { t } = useTranslation(NS)
  const namespaces = useNamespaceNames()
  const names = namespaces.names
  const exists = names.includes(session.namespace)
  const nameProblem = nameError(session.name)
  const namespaceProblem = namespaceError(session.namespace)
  const chart = session.chartInfo

  return (
    <div className="helm-pane">
      <div className="helm-pane__scroll" style={{ flex: 'none' }}>
        {chart && (
          <div className="helm-summary" style={{ marginTop: 0, marginBottom: 12 }}>
            <span className="helm-charts__name">{`${chart.name}-${chart.version}`}</span>
            {chart.appVersion && <span className="helm-muted">{t('install.values.appVersion', { version: chart.appVersion })}</span>}
            <span className="helm-muted">{chart.description}</span>
          </div>
        )}
        <div className="helm-form">
          <Field label={t('install.values.name')} required error={session.name && nameProblem ? t('install.values.nameInvalid') : undefined}>
            <TextInput value={session.name} onChange={(name) => change({ name })} monospace />
          </Field>
          <Field
            label={t('install.values.namespace')}
            required
            error={session.namespace && namespaceProblem ? t('install.values.namespaceInvalid') : undefined}
            help={namespaces.listed && !namespaces.loading && !exists && !namespaceProblem ? t('install.values.namespaceNew') : undefined}
          >
            {names.length > 0 && !session.createNamespace ? (
              <Select
                value={session.namespace}
                options={(exists ? names : [session.namespace, ...names]).map((n) => ({ value: n, label: n }))}
                onChange={(namespace) => change({ namespace })}
              />
            ) : (
              <TextInput value={session.namespace} onChange={(namespace) => change({ namespace })} monospace />
            )}
          </Field>
          <Field label={t('install.values.createNamespace')}>
            <Checkbox
              checked={session.createNamespace}
              onChange={(createNamespace) => change({ createNamespace })}
              label={t('install.values.createNamespaceHint')}
            />
          </Field>
        </div>
        <div className="detail__subheading">{t('install.values.values')}</div>
        <div className="helm-muted" style={{ fontSize: 12 }}>
          {t('install.values.valuesHint')}
        </div>
      </div>
      <div className="helm-pane__editor">
        <CodeEditor value={session.values} onChange={(values) => change({ values })} ariaLabel={t('install.values.values')} />
      </div>
    </div>
  )
}
