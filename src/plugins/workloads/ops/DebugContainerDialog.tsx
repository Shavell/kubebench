import { useState } from 'react'
import { useLayoutStore } from '@/core/store/layoutStore'
import { refreshData, useProviders } from '@/data/DataContext'
import type { Pod } from '@/data/types'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Field, Select, TextInput } from '@/shared/form'
import { notify } from '@/shared/notify'
import { useWorkloadsTranslation } from '../i18n'

/** The backend's default image (kube.DefaultDebugImage). */
const DEFAULT_IMAGE = 'busybox:1.37'

/**
 * Adds an ephemeral debug container to a running Pod and opens a shell in
 * it: a `pod-exec` tab for the new container (the terminal plugin renders
 * it). The request answers once the container runs, so the terminal can
 * attach right away. Needs exec permission, which the callers check.
 */
export function DebugContainerDialog({ pod, onClose }: { pod: Pod; onClose: () => void }) {
  const { t } = useWorkloadsTranslation()
  const { workloadOps } = useProviders()
  const [image, setImage] = useState(DEFAULT_IMAGE)
  const [target, setTarget] = useState(pod.containers[0]?.name ?? '')
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)

  const imageError = /\s/.test(image.trim()) ? t('workloadOps.debug.invalidImage') : null

  const add = async () => {
    if (imageError) return
    setBusy(true)
    setFailure(null)
    let container: string
    try {
      container = (await workloadOps.addDebugContainer(pod.namespace, pod.name, { image: image.trim(), target })).container
    } catch (err) {
      setFailure(err)
      setBusy(false)
      refreshData() // a container that did not start was added all the same
      return
    }
    notify.success(t('workloadOps.debug.added', { container, pod: pod.name }))
    refreshData()
    useLayoutStore.getState().openTab({
      id: `exec:${pod.namespace}/${pod.name}/${container}`,
      title: `${pod.name} · ${container}`,
      icon: 'debug',
      kind: 'pod-exec',
      props: { namespace: pod.namespace, pod: pod.name, container },
    })
    onClose()
  }

  return (
    <Dialog
      open
      size="medium"
      icon="debug"
      title={t('workloadOps.debug.title', { name: pod.name })}
      description={t('workloadOps.debug.description')}
      busy={busy}
      onClose={onClose}
      onSubmit={() => void add()}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" icon="terminal" busy={busy} disabled={imageError !== null}>
            {t('workloadOps.debug.submit')}
          </Button>
        </>
      }
    >
      <Field
        label={t('workloadOps.debug.image')}
        help={t('workloadOps.debug.imageHelp', { image: DEFAULT_IMAGE })}
        error={imageError}
      >
        <TextInput value={image} onChange={setImage} monospace autoComplete="off" disabled={busy} />
      </Field>
      <Field label={t('workloadOps.debug.target')} help={t('workloadOps.debug.targetHelp')}>
        <Select
          value={target}
          onChange={setTarget}
          disabled={busy}
          options={[
            { value: '', label: t('workloadOps.debug.noTarget') },
            ...pod.containers.map((c) => ({ value: c.name, label: c.name })),
          ]}
        />
      </Field>
      {busy && <p className="ops-muted">{t('workloadOps.debug.starting')}</p>}
      <ErrorNotice error={failure} />
    </Dialog>
  )
}
