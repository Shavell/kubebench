import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { useProviderQuery } from '@/data/DataContext'
import type { TerminalTarget } from '@/data/streams'
import { CORE_NS } from '@/i18n'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { Field, Select } from '@/shared/form'
import { NS } from '../i18n'

interface NewSessionDialogProps {
  /** Preselected container, e.g. the one of the active session. */
  initial?: TerminalTarget
  onClose: () => void
  onOpen: (target: TerminalTarget) => void
}

/**
 * Picks namespace, pod and container for a new terminal session. Mount it
 * while it is shown, so every opening starts from `initial`.
 */
export function NewSessionDialog({ initial, onClose, onOpen }: NewSessionDialogProps) {
  const { t } = useTranslation(NS)
  const contextNamespace = useContextStore((s) => s.namespace)
  const [namespace, setNamespace] = useState(
    initial?.namespace ?? (contextNamespace === ALL_NAMESPACES ? 'default' : contextNamespace),
  )
  const [pod, setPod] = useState(initial?.pod ?? '')
  const [container, setContainer] = useState(initial?.container ?? '')

  const namespaces = useProviderQuery((p) => p.cluster.listNamespaces(), [])
  const pods = useProviderQuery((p) => p.cluster.listPods(namespace), [namespace])
  const podList = [...(pods.data ?? [])].sort((a, b) => a.name.localeCompare(b.name))
  const selectedPod = podList.find((p) => p.name === pod) ?? podList[0]
  const containers = selectedPod?.containers.map((c) => c.name) ?? []
  const selectedContainer = containers.includes(container) ? container : (containers[0] ?? '')

  const namespaceNames = (namespaces.data ?? []).map((n) => n.name)
  if (!namespaceNames.includes(namespace)) namespaceNames.unshift(namespace)
  const podError = pods.error ?? (!pods.loading && podList.length === 0 ? t('shell.noPods') : undefined)

  const submit = () => {
    if (!selectedPod || !selectedContainer) return
    onOpen({ namespace, pod: selectedPod.name, container: selectedContainer })
    onClose()
  }

  return (
    <Dialog
      open
      icon="terminal"
      size="small"
      title={t('shell.newSessionTitle')}
      onClose={onClose}
      onSubmit={submit}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            {t('common.cancel', { ns: CORE_NS })}
          </Button>
          <Button type="submit" icon="terminal" disabled={!selectedPod || !selectedContainer}>
            {t('common.open', { ns: CORE_NS })}
          </Button>
        </>
      }
    >
      <Field label={t('shell.namespace')}>
        <Select
          value={namespace}
          options={namespaceNames.map((name) => ({ value: name, label: name }))}
          onChange={(value) => {
            setNamespace(value)
            setPod('')
          }}
        />
      </Field>
      <Field label={t('shell.pod')} error={podError}>
        <Select
          value={selectedPod?.name ?? ''}
          options={podList.map((p) => ({ value: p.name, label: `${p.name} (${p.status})` }))}
          disabled={podList.length === 0}
          onChange={setPod}
        />
      </Field>
      <Field label={t('shell.container')}>
        <Select
          value={selectedContainer}
          options={containers.map((name) => ({ value: name, label: name }))}
          disabled={containers.length === 0}
          onChange={setContainer}
        />
      </Field>
    </Dialog>
  )
}
