import { useTranslation } from 'react-i18next'
import { Select, TextInput } from '@/shared/form'
import { FieldContext } from '@/shared/form/fieldContext'
import { NS } from '../i18n'
import { SIZE_UNITS, type SizeUnit } from '../model/quantity'

/** Amount + binary unit, e.g. 10 Gi. Inside a Field the text box takes the label. */
export function SizeInput({
  amount,
  unit,
  onAmount,
  onUnit,
  disabled,
}: {
  amount: string
  unit: SizeUnit
  onAmount: (amount: string) => void
  onUnit: (unit: SizeUnit) => void
  disabled?: boolean
}) {
  const { t } = useTranslation(NS)
  return (
    <div className="storage-size">
      <TextInput value={amount} onChange={onAmount} inputMode="decimal" monospace disabled={disabled} />
      {/* The unit is a second control: it must not take the field's id, which labels the amount. */}
      <FieldContext.Provider value={null}>
        <Select
          aria-label={t('fields.unit')}
          value={unit}
          onChange={onUnit}
          options={SIZE_UNITS.map((u) => ({ value: u, label: u }))}
          disabled={disabled}
        />
      </FieldContext.Provider>
    </div>
  )
}
