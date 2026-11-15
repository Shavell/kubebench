import { useEffect, useState } from 'react'
import { usePermission } from '@/core/permissions'
import { useLayoutStore } from '@/core/store/layoutStore'
import { refreshData, useProviderQuery, useProviders } from '@/data/DataContext'
import type { ObjectRef } from '@/data/types'
import { Button } from '@/shared/Button'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { CodeEditor } from '@/shared/editor/CodeEditor'
import { DiffEditor } from '@/shared/editor/DiffEditor'
import { copyText, downloadText } from '@/shared/fileActions'
import { Checkbox } from '@/shared/form'
import { Icon } from '@/shared/Icon'
import { notify } from '@/shared/notify'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { ApplyResults } from './ApplyResults'
import { outcomeFailed, summarize, useApplyRunner, type ApplyOutcome } from './applyRunner'
import { DiscardDialog } from './DiscardDialog'
import { useEditorTranslation } from './i18n'
import { useTabSession } from './tabSessions'
import './editor.css'

type Mode = 'view' | 'edit' | 'review'

interface YamlSession {
  mode: Mode
  /** Viewing option; editing always starts from the neat YAML. */
  neat: boolean
  /** The neat YAML editing started from, and the edited text. */
  base: string
  draft: string
  outcome: ApplyOutcome | null
}

const newSession = (): YamlSession => ({ mode: 'view', neat: false, base: '', draft: '', outcome: null })

/**
 * The YAML of a cluster object (design §7.2): view it in full or neat,
 * copy or download it; edit it, review the diff against the live object,
 * dry run and apply it with server-side apply, forcing past conflicts with
 * other field managers. Unapplied edits survive tab switches and mark the
 * tab dirty, so closing it asks first.
 */
export function ResourceYamlEditor({ tabId, target }: { tabId: string; target: ObjectRef }) {
  const { t } = useEditorTranslation()
  const { cluster } = useProviders()
  const write = usePermission('write')
  const setTabDirty = useLayoutStore((s) => s.setTabDirty)
  const [session, update] = useTabSession(tabId, newSession)
  const { mode, neat, base, draft, outcome } = session
  const { busy, run } = useApplyRunner()
  const [starting, setStarting] = useState(false)
  const [discarding, setDiscarding] = useState(false)
  const [forcing, setForcing] = useState(false)

  const editing = mode !== 'view'
  const dirty = editing && draft !== base
  useEffect(() => setTabDirty(tabId, dirty), [tabId, dirty, setTabDirty])

  const targetKey = `${target.apiVersion}/${target.kind}/${target.namespace ?? ''}/${target.name}`
  const wantNeat = editing || neat
  const live = useProviderQuery((p) => p.cluster.getYaml(target, { neat: wantNeat }), [targetKey, wantNeat])
  const isSecret = target.apiVersion === 'v1' && target.kind === 'Secret'
  const applyOptions = { namespace: target.namespace ?? undefined }

  const startEditing = async () => {
    setStarting(true)
    try {
      const text = await cluster.getYaml(target, { neat: true })
      update({ mode: 'edit', base: text, draft: text, outcome: null })
    } catch (err) {
      notify.error(err)
    } finally {
      setStarting(false)
    }
  }

  const stopEditing = () => update({ mode: 'view', base: '', draft: '', outcome: null })

  const dryRun = async () => update({ outcome: await run(draft, { ...applyOptions, dryRun: true }) })

  const apply = async (force: boolean) => {
    const result = await run(draft, { ...applyOptions, force })
    if (outcomeFailed(result)) {
      update({ outcome: result })
      return
    }
    notify.success(t('yaml.applied', { name: target.name }), { detail: summarize(result.results, false) })
    update({ mode: 'view', base: '', draft: '', outcome: result })
  }

  const shownText = editing ? draft : (live.data ?? '')
  const title = target.namespace ? `${target.kind} · ${target.namespace} / ${target.name}` : `${target.kind} · ${target.name}`
  const busyApply = busy === 'apply'

  const toolbar = (
    <Toolbar title={title}>
      {mode === 'view' && (
        <>
          <div className="kb-yaml__options">
            <Checkbox checked={neat} onChange={(on) => update({ neat: on })} label={t('yaml.neat')} title={t('yaml.neatHint')} />
          </div>
          <Button variant="secondary" icon="refresh" onClick={refreshData} title={t('yaml.reloadHint')}>
            {t('yaml.reload')}
          </Button>
        </>
      )}
      <Button variant="secondary" icon="copy" disabled={!shownText} onClick={() => void copyText(shownText)}>
        {t('common.copy')}
      </Button>
      <Button
        variant="secondary"
        icon="desktop-download"
        disabled={!shownText}
        onClick={() => downloadText(`${target.name}.yaml`, shownText)}
      >
        {t('common.download')}
      </Button>
      {mode === 'view' && (
        <Button icon="edit" busy={starting} disabled={!live.data} title={t('yaml.editHint')} onClick={startEditing}>
          {t('yaml.edit')}
        </Button>
      )}
      {editing && (
        <>
          <Button variant="secondary" onClick={() => (dirty ? setDiscarding(true) : stopEditing())} disabled={busy !== null}>
            {t('yaml.cancel')}
          </Button>
          {mode === 'review' && (
            <Button variant="secondary" icon="arrow-left" onClick={() => update({ mode: 'edit' })}>
              {t('yaml.backToEdit')}
            </Button>
          )}
          <Button variant="secondary" icon="beaker" busy={busy === 'dryRun'} disabled={busy !== null} onClick={dryRun}>
            {t('yaml.dryRun')}
          </Button>
          {mode === 'edit' ? (
            <Button
              icon="diff"
              disabled={!dirty}
              title={dirty ? undefined : t('yaml.noChanges')}
              onClick={() => update({ mode: 'review' })}
            >
              {t('yaml.review')}
            </Button>
          ) : (
            <Button
              icon="cloud-upload"
              busy={busyApply && !forcing}
              disabled={!write.allowed || busy !== null}
              title={write.reason ?? undefined}
              onClick={() => void apply(false)}
            >
              {t('yaml.apply')}
            </Button>
          )}
        </>
      )}
    </Toolbar>
  )

  const body = () => {
    if (!editing && live.error) {
      return <Unavailable code={live.errorCode} reason={live.error} title={t('yaml.loadFailed', { name: target.name })} onRetry={refreshData} />
    }
    if (!editing && !live.data) return <div className="page-loading">{t('yaml.loading', { name: target.name })}</div>
    if (mode === 'review') {
      return (
        <DiffEditor
          original={live.data ?? base}
          modified={draft}
          readOnly={false}
          onChange={(text) => update({ draft: text })}
          originalTitle={t('yaml.live')}
          modifiedTitle={t('yaml.edited')}
        />
      )
    }
    return (
      <CodeEditor
        value={shownText}
        readOnly={!editing}
        onChange={editing ? (text) => update({ draft: text }) : undefined}
        minimap
        ariaLabel={t('yaml.ariaLabel', { name: target.name })}
      />
    )
  }

  return (
    <div className="kb-yaml">
      {toolbar}
      {editing && (
        <div className={isSecret ? 'kb-yaml__banner kb-yaml__banner--warning' : 'kb-yaml__banner'}>
          <Icon name={isSecret ? 'warning' : 'info'} size={14} />
          <span>{isSecret ? t('yaml.secretHint') : mode === 'review' ? t('yaml.reviewHint') : t('yaml.editHintBanner')}</span>
        </div>
      )}
      <div className="kb-yaml__editor">{body()}</div>
      {outcome && (
        <ApplyResults
          outcome={outcome}
          onForce={editing ? () => setForcing(true) : undefined}
          forcing={busyApply && forcing}
          onDismiss={() => update({ outcome: null })}
        />
      )}
      <DiscardDialog
        open={discarding}
        title={t('yaml.discardTitle')}
        message={t('yaml.discardMessage', { name: target.name })}
        confirmLabel={t('yaml.discard')}
        onConfirm={stopEditing}
        onClose={() => setDiscarding(false)}
      />
      <ConfirmDialog
        open={forcing}
        danger
        title={t('apply.forceTitle', { name: target.name })}
        message={t('apply.forceMessage')}
        confirmLabel={t('apply.force')}
        confirmPhrase={target.name}
        onConfirm={() => apply(true)}
        onClose={() => setForcing(false)}
      />
    </div>
  )
}
