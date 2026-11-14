import clsx from 'clsx'
import { useId, useState, type InputHTMLAttributes, type Ref } from 'react'
import { useFieldControlProps } from './fieldContext'
import './form.css'

export interface NumberInputProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, 'value' | 'onChange' | 'type' | 'min' | 'max' | 'step'> {
  /** null while the input is empty or holds no valid number. */
  value: number | null
  onChange: (value: number | null) => void
  min?: number
  max?: number
  step?: number
  /** Unit shown after the input, e.g. "Gi" or "s". */
  unit?: string
  ref?: Ref<HTMLInputElement>
}

const toText = (value: number | null) => (value === null ? '' : String(value))

function parse(text: string): number | null {
  if (text.trim() === '') return null
  const value = Number(text)
  return Number.isFinite(value) ? value : null
}

/**
 * Numeric input. It keeps the typed text while it is being edited, so
 * partial input such as "1." is not reformatted; the owner receives the
 * parsed number (or null).
 */
export function NumberInput({
  value,
  onChange,
  min,
  max,
  step,
  unit,
  className,
  ...rest
}: NumberInputProps) {
  const a11y = useFieldControlProps(rest.id)
  const unitId = useId()
  const [draft, setDraft] = useState(() => toText(value))
  const [lastValue, setLastValue] = useState(value)

  // Adopt a value set by the owner unless the draft already says the same.
  if (value !== lastValue) {
    setLastValue(value)
    if (parse(draft) !== value) setDraft(toText(value))
  }

  const describedBy = [a11y['aria-describedby'], unit ? unitId : undefined].filter(Boolean).join(' ')

  const input = (
    <input
      inputMode="decimal"
      {...rest}
      {...a11y}
      aria-describedby={describedBy || undefined}
      type="number"
      min={min}
      max={max}
      step={step}
      value={draft}
      onChange={(event) => {
        const next = parse(event.target.value)
        setDraft(event.target.value)
        setLastValue(next)
        onChange(next)
      }}
      className={clsx('kb-input', 'kb-input--number', className)}
    />
  )

  if (!unit) return input
  return (
    <span className="kb-input-group">
      {input}
      <span id={unitId} className="kb-input-group__unit">
        {unit}
      </span>
    </span>
  )
}
