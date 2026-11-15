import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { refreshData } from '@/data/DataContext'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Field } from '@/shared/form'
import { notify } from '@/shared/notify'
import { storageData } from '../data'
import type { PvcSummary } from '../data/types'
import { NS } from '../i18n'
import { quantityBytes, sizeText, splitSize, type SizeUnit } from '../model/quantity'
import { sizeProblem } from '../model/rules'
import { SizeInput } from '../wizards/SizeInput'

/**
 * Raises a claim's storage request. Volumes only grow; the server checks
 * that again, along with the StorageClass's allowVolumeExpansion.
 */
export function ExpandPvcDialog({ pvc, onClose }: { pvc: PvcSummary; onClose: () => void }) {
  const { t } = useTranslation(NS)
  const current = pvc.capacity || pvc.requested
  const initial = splitSize(pvc.requested)
  const [amount, setAmount] = useState(initial.amount)
  const [unit, setUnit] = useState<SizeUnit>(initial.unit)
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)

  const size = sizeText(amount, unit)
  const problem = sizeProblem(size)
  const floor = Math.max(pvc.requestedBytes, pvc.capacityBytes)
  const error = problem
    ? t(`validation.${problem.key}`)
    : (quantityBytes(size) ?? 0) <= floor
      ? t('expand.mustGrow', { current })
      : null

  const submit = async () => {
    if (error) return
    setBusy(true)
    setFailure(null)
    try {
      await storageData().resizePvc(pvc.namespace, pvc.name, size)
    } catch (err) {
      setFailure(err)
      setBusy(false)
      return
    }
    notify.success(t('expand.done', { name: pvc.name, size }))
    refreshData()
    onClose()
  }

  return (
    <Dialog
      open
      icon="arrow-both"
      title={t('expand.title', { name: pvc.name })}
      description={t('expand.description', { current, storageClass: pvc.storageClass })}
      busy={busy}
      onClose={onClose}
      onSubmit={() => void submit()}
      size="small"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" busy={busy} disabled={Boolean(error)}>
            {t('expand.submit')}
          </Button>
        </>
      }
    >
      <Field label={t('expand.newSize')} error={amount ? error : null} required>
        <SizeInput amount={amount} unit={unit} onAmount={setAmount} onUnit={setUnit} disabled={busy} />
      </Field>
      <ErrorNotice error={failure} />
    </Dialog>
  )
}
