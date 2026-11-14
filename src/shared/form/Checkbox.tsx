import clsx from 'clsx'
import { useId, type InputHTMLAttributes, type ReactNode, type Ref } from 'react'
import { useFieldControlProps } from './fieldContext'
import './form.css'

export interface CheckboxProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, 'checked' | 'onChange' | 'type'> {
  checked: boolean
  onChange: (checked: boolean) => void
  /** Text next to the box. Omit it inside a Field, whose label names the box. */
  label?: ReactNode
  /** Secondary text under the label. */
  description?: ReactNode
  ref?: Ref<HTMLInputElement>
}

/** VS Code style checkbox, optionally with its own label. */
export function Checkbox({ checked, onChange, label, description, className, ...rest }: CheckboxProps) {
  const a11y = useFieldControlProps(rest.id)
  const baseId = useId()
  const id = a11y.id ?? `${baseId}box`
  const descriptionId = description ? `${baseId}description` : undefined
  const describedBy = [a11y['aria-describedby'], descriptionId].filter(Boolean).join(' ')

  const box = (
    <input
      {...rest}
      {...a11y}
      id={id}
      aria-describedby={describedBy || undefined}
      type="checkbox"
      checked={checked}
      onChange={(event) => onChange(event.target.checked)}
      className={clsx('kb-checkbox__box', !label && className)}
    />
  )
  if (!label) return box

  return (
    <div className={clsx('kb-checkbox', rest.disabled && 'kb-checkbox--disabled', className)}>
      {box}
      <div className="kb-checkbox__text">
        <label htmlFor={id} className="kb-checkbox__label">
          {label}
        </label>
        {description && (
          <div id={descriptionId} className="kb-checkbox__description">
            {description}
          </div>
        )}
      </div>
    </div>
  )
}
