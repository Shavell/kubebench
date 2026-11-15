import { useState } from 'react'
import { refreshData, useProviders } from '@/data/DataContext'
import type { K8sNode, NodeTaint, TaintEffect } from '@/data/types'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Select, TextInput } from '@/shared/form'
import { Icon } from '@/shared/Icon'
import { notify } from '@/shared/notify'
import { tw, useWorkloadsTranslation } from '../i18n'
import { labelValueError, qualifiedNameError } from './validation'
import './ops.css'

const EFFECTS: TaintEffect[] = ['NoSchedule', 'PreferNoSchedule', 'NoExecute']
const EFFECT_OPTIONS = EFFECTS.map((effect) => ({ value: effect, label: effect }))

/** The problem of each row, or null: the API server's rules for taints. */
function rowErrors(rows: NodeTaint[]): (string | null)[] {
  return rows.map((row, i) => {
    const key = row.key.trim()
    if (!key) return tw('workloadOps.taints.keyRequired')
    const keyProblem = qualifiedNameError(key)
    if (keyProblem) return keyProblem
    const valueProblem = labelValueError(row.value)
    if (valueProblem) return valueProblem
    const twice = rows.some((other, j) => j < i && other.key.trim() === key && other.effect === row.effect)
    return twice ? tw('workloadOps.taints.duplicate') : null
  })
}

const sameTaint = (a: NodeTaint, b: NodeTaint) => a.key === b.key && a.value === b.value && a.effect === b.effect

/** Edits the node's taints; saving replaces the whole list. */
export function TaintsDialog({ node, onClose }: { node: K8sNode; onClose: () => void }) {
  const { t } = useWorkloadsTranslation()
  const { workloadOps } = useProviders()
  const [rows, setRows] = useState<NodeTaint[]>(() => (node.taints ?? []).map((taint) => ({ ...taint })))
  const [attempted, setAttempted] = useState(false)
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)

  const problems = rowErrors(rows)
  const errors = attempted ? problems : rows.map(() => null)
  const before = node.taints ?? []
  const addsNoExecute = rows.some((row) => row.effect === 'NoExecute' && !before.some((old) => sameTaint(old, row)))

  const update = (index: number, patch: Partial<NodeTaint>) =>
    setRows((current) => current.map((row, i) => (i === index ? { ...row, ...patch } : row)))

  const save = async () => {
    setAttempted(true)
    if (problems.some(Boolean)) return
    setBusy(true)
    setFailure(null)
    try {
      await workloadOps.setNodeTaints(
        node.name,
        rows.map((row) => ({ ...row, key: row.key.trim() })),
      )
    } catch (err) {
      setFailure(err)
      setBusy(false)
      return
    }
    notify.success(t('workloadOps.taints.saved', { name: node.name }))
    refreshData()
    onClose()
  }

  return (
    <Dialog
      open
      size="medium"
      icon="tag"
      title={t('workloadOps.taints.title', { name: node.name })}
      description={t('workloadOps.taints.description')}
      busy={busy}
      onClose={onClose}
      onSubmit={() => void save()}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" busy={busy}>
            {t('common.save')}
          </Button>
        </>
      }
    >
      {rows.length === 0 ? (
        <p className="ops-muted">{t('workloadOps.taints.empty')}</p>
      ) : (
        <div className="ops-taints" role="group" aria-label={t('workloadOps.taints.listLabel')}>
          <div className="ops-taints__head" aria-hidden="true">
            <span>{t('workloadOps.taints.key')}</span>
            <span>{t('workloadOps.taints.value')}</span>
            <span>{t('workloadOps.taints.effect')}</span>
          </div>
          {rows.map((row, index) => {
            const n = index + 1
            return (
              // Rows have no identity of their own; the inputs are controlled.
              <div key={index} className="ops-taints__row">
                <TextInput
                  value={row.key}
                  onChange={(key) => update(index, { key })}
                  monospace
                  aria-label={t('workloadOps.taints.keyLabel', { row: n })}
                  aria-invalid={errors[index] ? true : undefined}
                  disabled={busy}
                />
                <TextInput
                  value={row.value}
                  onChange={(value) => update(index, { value })}
                  monospace
                  aria-label={t('workloadOps.taints.valueLabel', { row: n })}
                  disabled={busy}
                />
                <Select
                  value={row.effect}
                  onChange={(effect) => update(index, { effect })}
                  options={EFFECT_OPTIONS}
                  aria-label={t('workloadOps.taints.effectLabel', { row: n })}
                  disabled={busy}
                />
                <button
                  type="button"
                  className="kb-kv__remove"
                  aria-label={t('workloadOps.taints.remove', { row: n })}
                  title={t('workloadOps.taints.remove', { row: n })}
                  disabled={busy}
                  onClick={() => setRows((current) => current.filter((_, i) => i !== index))}
                >
                  <Icon name="close" size={14} />
                </button>
                {errors[index] && <div className="ops-taints__error">{errors[index]}</div>}
              </div>
            )
          })}
        </div>
      )}
      <div>
        <Button
          variant="secondary"
          icon="add"
          disabled={busy}
          onClick={() => setRows((current) => [...current, { key: '', value: '', effect: 'NoSchedule' }])}
        >
          {t('workloadOps.taints.add')}
        </Button>
      </div>
      {addsNoExecute && (
        <div className="ops-warning" role="note">
          <Icon name="warning" size={14} />
          <span>{t('workloadOps.taints.noExecuteWarning')}</span>
        </div>
      )}
      <ErrorNotice error={failure} />
    </Dialog>
  )
}
