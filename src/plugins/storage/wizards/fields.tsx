import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Checkbox, Field, KeyValueEditor, Select, type KeyValuePair } from '@/shared/form'
import { FieldContext, useField } from '@/shared/form/fieldContext'
import { NS } from '../i18n'
import { ACCESS_MODES, labelKeyProblem, VOLUME_MODES } from '../model/rules'
import { problemText } from './useWizard'

/** Access mode checkboxes; the short kubectl names explain the long ones. */
export function AccessModesField({
  value,
  onChange,
  error,
  disabled,
}: {
  value: string[]
  onChange: (modes: string[]) => void
  error: string | null
  disabled?: boolean
}) {
  const { t } = useTranslation(NS)
  const toggle = (mode: string, on: boolean) =>
    onChange(on ? ACCESS_MODES.filter((m) => m === mode || value.includes(m)) : value.filter((m) => m !== mode))
  return (
    <Field label={t('fields.accessModes')} help={t('fields.accessModesHelp')} error={error} required>
      <CheckboxGroup>
        {ACCESS_MODES.map((mode) => (
          <Checkbox
            key={mode}
            label={t(`accessModes.${mode}`)}
            checked={value.includes(mode)}
            onChange={(on) => toggle(mode, on)}
            disabled={disabled}
          />
        ))}
      </CheckboxGroup>
    </Field>
  )
}

/**
 * Several labelled checkboxes inside one Field: the field's label names the
 * group, and each box keeps its own id (a Field hands its single control id
 * to every control inside it).
 */
export function CheckboxGroup({ children }: { children: ReactNode }) {
  const field = useField()
  return (
    <div
      className="storage-checks"
      role="group"
      aria-labelledby={field?.labelId}
      aria-describedby={field?.describedBy}
    >
      <FieldContext.Provider value={null}>{children}</FieldContext.Provider>
    </div>
  )
}

export function VolumeModeField({ value, onChange }: { value: string; onChange: (mode: string) => void }) {
  const { t } = useTranslation(NS)
  return (
    <Field label={t('fields.volumeMode')} help={t('fields.volumeModeHelp')}>
      <Select value={value} onChange={onChange} options={VOLUME_MODES.map((m) => ({ value: m, label: m }))} />
    </Field>
  )
}

export function LabelsField({
  value,
  onChange,
  error,
}: {
  value: KeyValuePair[]
  onChange: (pairs: KeyValuePair[]) => void
  error: string | null
}) {
  const { t } = useTranslation(NS)
  return (
    <Field label={t('fields.labels')} error={error}>
      <KeyValueEditor
        value={value}
        onChange={onChange}
        validateKey={(key) => problemText(labelKeyProblem(key) ?? undefined)}
        addLabel={t('fields.addLabel')}
      />
    </Field>
  )
}
