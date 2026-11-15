import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePluginQuery } from '@/data/pluginData'
import { CodeEditor } from '@/shared/editor/CodeEditor'
import { Unavailable } from '@/shared/Unavailable'
import { helmData, type HelmReleaseDetail } from '../../data'
import { NS } from '../../i18n'
import { RevisionSelect } from './RevisionSelect'

/** Manifest tab: the rendered manifest (hooks included) of any revision. */
export function ManifestTab({ release }: { release: HelmReleaseDetail }) {
  const { t } = useTranslation(NS)
  const [revision, setRevision] = useState(0)
  const { namespace, name } = release
  const manifest = usePluginQuery(
    () => helmData().getManifest(namespace, name, revision),
    [namespace, name, revision],
  )

  return (
    <div className="helm-pane">
      <div className="helm-pane__controls">
        <RevisionSelect history={release.history} value={revision} onChange={setRevision} label={t('release.manifest.revision')} />
      </div>
      {manifest.error ? (
        <Unavailable compact code={manifest.errorCode} reason={manifest.error} />
      ) : manifest.data === null ? (
        <div className="page-loading">{t('release.manifest.loading')}</div>
      ) : (
        <div className="helm-pane__editor">
          <CodeEditor value={manifest.data} readOnly ariaLabel={t('release.manifest.aria', { name })} />
        </div>
      )}
    </div>
  )
}
