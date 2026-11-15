import { useEffect, useRef, useState, type DragEvent } from 'react'
import { usePermission } from '@/core/permissions'
import { openYamlTab } from '@/core/plugin/editorTabs'
import type { TabDescriptor } from '@/core/plugin/types'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { Button } from '@/shared/Button'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { CodeEditor } from '@/shared/editor/CodeEditor'
import { Select } from '@/shared/form'
import { Icon } from '@/shared/Icon'
import { notify } from '@/shared/notify'
import { Toolbar } from '@/shared/Toolbar'
import { ApplyResults } from './ApplyResults'
import { outcomeFailed, summarize, useApplyRunner, type ApplyOutcome } from './applyRunner'
import { DiscardDialog } from './DiscardDialog'
import { useEditorTranslation } from './i18n'
import { draggingFiles, MAX_MANIFEST_BYTES, readManifestFiles } from './manifestFiles'
import { useTabSession } from './tabSessions'
import { templateById, templateFor, TEMPLATES } from './templates'
import './editor.css'

interface NewResourceSession {
  templateId: string
  /** For objects that name no namespace, and the namespace templates are written for. */
  namespace: string
  text: string
  /** The text as inserted (template, import) or last applied: equal means nothing to lose. */
  baseline: string
  /** The imported files the text came from; empty for a template. */
  files: string[]
  outcome: ApplyOutcome | null
}

/** A new text for the editor, waiting for the user to discard their edits. */
interface Replacement {
  text: string
  templateId?: string
  files: string[]
}

/**
 * Creates objects (design §7.2): start from a template for the chosen
 * namespace, or import YAML/JSON files (picked or dropped, any number of
 * documents), then dry run and apply them with server-side apply.
 */
export function NewResourceTab({ tab }: { tab: TabDescriptor }) {
  const { t } = useEditorTranslation()
  const write = usePermission('write')
  const setTabDirty = useLayoutStore((s) => s.setTabDirty)
  const globalNamespace = useContextStore((s) => s.namespace)
  const props = tab.props ?? {}
  const [session, update] = useTabSession<NewResourceSession>(tab.id, () => {
    const namespace =
      (typeof props.namespace === 'string' && props.namespace) ||
      (globalNamespace !== ALL_NAMESPACES ? globalNamespace : 'default')
    const template = templateFor(typeof props.kind === 'string' ? props.kind : undefined)
    const text = template.render(namespace)
    return { templateId: template.id, namespace, text, baseline: text, files: [], outcome: null }
  })
  const { templateId, namespace, text, baseline, files, outcome } = session
  const { busy, run } = useApplyRunner()
  const [pending, setPending] = useState<Replacement | null>(null)
  const [dropping, setDropping] = useState(false)
  const [forcing, setForcing] = useState(false)
  const fileInput = useRef<HTMLInputElement>(null)
  const namespaces = useProviderQuery((p) => p.cluster.listNamespaces(), [])

  const dirty = text !== baseline
  useEffect(() => setTabDirty(tab.id, dirty), [tab.id, dirty, setTabDirty])

  const replace = (next: Replacement) =>
    update({ text: next.text, baseline: next.text, templateId: next.templateId ?? templateId, files: next.files, outcome: null })
  const offer = (next: Replacement) => (dirty ? setPending(next) : replace(next))

  const changeNamespace = (ns: string) => {
    // An untouched template follows the namespace; edited or imported text keeps its own.
    if (!dirty && files.length === 0) {
      const rendered = templateById(templateId).render(ns)
      update({ namespace: ns, text: rendered, baseline: rendered, outcome: null })
    } else {
      update({ namespace: ns })
    }
  }

  const importFiles = async (picked: File[]) => {
    if (picked.length === 0) return
    if (picked.reduce((total, file) => total + file.size, 0) > MAX_MANIFEST_BYTES) {
      notify.error(t('newResource.tooLarge', { size: MAX_MANIFEST_BYTES >> 20 }))
      return
    }
    try {
      offer({ text: await readManifestFiles(picked), files: picked.map((file) => file.name) })
    } catch (err) {
      notify.error(err)
    }
  }

  // Files dropped anywhere on the editor are imported; text drags stay with Monaco.
  const onDragOver = (event: DragEvent) => {
    if (!draggingFiles(event.dataTransfer.types)) return
    event.preventDefault()
    event.stopPropagation()
    event.dataTransfer.dropEffect = 'copy'
    setDropping(true)
  }
  const onDragLeave = (event: DragEvent) => {
    if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setDropping(false)
  }
  const onDrop = (event: DragEvent) => {
    if (!draggingFiles(event.dataTransfer.types)) return
    event.preventDefault()
    event.stopPropagation()
    setDropping(false)
    void importFiles(Array.from(event.dataTransfer.files))
  }

  const execute = async (dryRun: boolean, force = false) => {
    const result = await run(text, { namespace, dryRun, force })
    if (dryRun || outcomeFailed(result)) {
      update({ outcome: result })
      return
    }
    update({ outcome: result, baseline: text })
    const first = result.results.find((r) => r.action)
    notify.success(t('newResource.applied'), {
      detail: summarize(result.results, false),
      action: first
        ? {
            label: t('newResource.open'),
            run: () => openYamlTab({ apiVersion: first.apiVersion, kind: first.kind, namespace: first.namespace, name: first.name }),
          }
        : undefined,
    })
  }

  const templateOptions = [
    ...(files.length > 0 ? [{ value: '', label: t('newResource.importedOption'), disabled: true }] : []),
    ...TEMPLATES.map((tpl) => ({ value: tpl.id, label: t(`newResource.templates.${tpl.id}`) })),
  ]
  const namespaceNames = (namespaces.data ?? []).map((ns) => ns.name)
  if (!namespaceNames.includes(namespace)) namespaceNames.unshift(namespace)
  const conflict = outcome?.results.find((r) => r.code === 'conflict')

  return (
    <div className="kb-yaml">
      <Toolbar title={t('newResource.title')}>
        <Select
          className="kb-yaml__select"
          aria-label={t('newResource.template')}
          title={t('newResource.template')}
          value={files.length > 0 ? '' : templateId}
          options={templateOptions}
          onChange={(id) => offer({ text: templateById(id).render(namespace), templateId: id, files: [] })}
        />
        <Select
          className="kb-yaml__select"
          aria-label={t('newResource.namespace')}
          title={t('newResource.namespaceHint')}
          value={namespace}
          options={namespaceNames.map((name) => ({ value: name, label: name }))}
          onChange={changeNamespace}
        />
        <Button variant="secondary" icon="folder-opened" onClick={() => fileInput.current?.click()}>
          {t('newResource.import')}
        </Button>
        <input
          ref={fileInput}
          type="file"
          accept=".yaml,.yml,.json,application/json,application/yaml,text/yaml"
          multiple
          hidden
          onChange={(event) => {
            void importFiles(Array.from(event.target.files ?? []))
            event.target.value = '' // the same file can be picked again
          }}
        />
        <Button variant="secondary" icon="beaker" busy={busy === 'dryRun'} disabled={busy !== null || !text.trim()} onClick={() => void execute(true)}>
          {t('yaml.dryRun')}
        </Button>
        <Button
          icon="cloud-upload"
          busy={busy === 'apply' && !forcing}
          disabled={!write.allowed || busy !== null || !text.trim()}
          title={write.reason ?? undefined}
          onClick={() => void execute(false)}
        >
          {t('yaml.apply')}
        </Button>
      </Toolbar>
      <div className="kb-yaml__banner">
        <Icon name="info" size={14} />
        <span>
          {files.length > 0 ? t('newResource.importedFrom', { files: files.join(', ') }) : t('newResource.hint')}
        </span>
      </div>
      <div
        className="kb-yaml__editor"
        onDragOverCapture={onDragOver}
        onDragLeave={onDragLeave}
        onDropCapture={onDrop}
      >
        <CodeEditor value={text} onChange={(next) => update({ text: next })} ariaLabel={t('newResource.title')} />
        {dropping && (
          <div className="kb-yaml__drop">
            <Icon name="cloud-upload" size={18} /> {t('newResource.drop')}
          </div>
        )}
      </div>
      {outcome && (
        <ApplyResults
          outcome={outcome}
          onForce={() => setForcing(true)}
          forcing={busy === 'apply' && forcing}
          onDismiss={() => update({ outcome: null })}
        />
      )}
      <DiscardDialog
        open={pending !== null}
        title={t('newResource.replaceTitle')}
        message={t('newResource.replaceMessage')}
        confirmLabel={t('newResource.replace')}
        onConfirm={() => pending && replace(pending)}
        onClose={() => setPending(null)}
      />
      <ConfirmDialog
        open={forcing}
        danger
        title={t('apply.forceTitle', { name: conflict?.name ?? '' })}
        message={t('apply.forceMessage')}
        confirmLabel={t('apply.force')}
        confirmPhrase={conflict?.name}
        onConfirm={() => execute(false, true)}
        onClose={() => setForcing(false)}
      />
    </div>
  )
}
