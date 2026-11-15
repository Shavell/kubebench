import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePluginQuery } from '@/data/pluginData'
import { CodeEditor } from '@/shared/editor/CodeEditor'
import { Checkbox } from '@/shared/form'
import { Unavailable } from '@/shared/Unavailable'
import { helmData, type HelmReleaseDetail } from '../../data'
import { NS } from '../../i18n'
import { RevisionSelect } from './RevisionSelect'

/** Values tab: user-supplied or all (merged with the chart defaults) values of any revision. */
export function ValuesTab({ release }: { release: HelmReleaseDetail }) {
  const { t } = useTranslation(NS)
  const [revision, setRevision] = useState(0)
  const [all, setAll] = useState(false)
  const { namespace, name } = release
  const values = usePluginQuery(
    () => helmData().getValues(namespace, name, { revision, all }),
    [namespace, name, revision, all],
  )

  return (
    <div className="helm-pane">
      <div className="helm-pane__controls">
        <RevisionSelect history={release.history} value={revision} onChange={setRevision} label={t('release.values.revision')} />
        <Checkbox checked={all} onChange={setAll} label={t('release.values.all')} />
      </div>
      {values.error ? (
        <Unavailable compact code={values.errorCode} reason={values.error} />
      ) : values.data === null ? (
        <div className="page-loading">{t('release.values.loading')}</div>
      ) : (
        <div className="helm-pane__editor">
          <CodeEditor
            value={values.data || (all ? '' : t('release.values.none'))}
            readOnly
            ariaLabel={t('release.values.aria', { name })}
          />
        </div>
      )}
    </div>
  )
}
