import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { isValidFontFamily } from '@/shared/fontFamily'
import { Field, TextInput } from '@/shared/form'
import { SETTINGS_NS } from '../namespace'

interface FontFamilyFieldProps {
  label: string
  help: string
  value: string
  /** Called with every valid entry; an invalid one stays a draft with an error. */
  onChange: (value: string) => void
}

/** A font family list, applied as typed while it is valid; empty = default. */
export function FontFamilyField({ label, help, value, onChange }: FontFamilyFieldProps) {
  const { t } = useTranslation(SETTINGS_NS)
  const [draft, setDraft] = useState(value)
  const valid = isValidFontFamily(draft)

  return (
    <Field layout="inline" label={label} help={help} error={valid ? undefined : t('fonts.invalid')}>
      <TextInput
        value={draft}
        monospace
        placeholder={t('fonts.default')}
        maxLength={200}
        onChange={(next) => {
          setDraft(next)
          if (isValidFontFamily(next)) onChange(next.trim())
        }}
      />
    </Field>
  )
}
