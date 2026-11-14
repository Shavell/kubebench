import clsx from 'clsx'
import { useId, useMemo, type ReactNode } from 'react'
import { Icon } from '../Icon'
import { FieldContext } from './fieldContext'
import './form.css'

export interface FieldProps {
  /** Visible label; also the accessible name of the control. */
  label: ReactNode
  /** Explanation shown under the label. */
  help?: ReactNode
  /** Validation message; marks the control invalid (aria-invalid) while set. */
  error?: ReactNode
  /** Shows a required marker and sets aria-required on the control. */
  required?: boolean
  /**
   * `stacked` (default): label above the control, for dialogs and forms.
   * `inline`: label and help on the left, control on the right, like a
   * Settings row.
   */
  layout?: 'stacked' | 'inline'
  /** id for the control; generated when omitted. */
  id?: string
  /** One form primitive (TextInput, Select, …); it picks up the ids automatically. */
  children: ReactNode
  className?: string
}

/** Label, help and error around one form control, wired up for screen readers. */
export function Field({
  label,
  help,
  error,
  required = false,
  layout = 'stacked',
  id,
  children,
  className,
}: FieldProps) {
  const baseId = useId()
  const controlId = id ?? `${baseId}control`
  const labelId = `${baseId}label`
  const helpId = help ? `${baseId}help` : undefined
  const errorId = error ? `${baseId}error` : undefined
  const describedBy = [errorId, helpId].filter(Boolean).join(' ') || undefined
  const invalid = Boolean(error)

  const context = useMemo(
    () => ({ controlId, labelId, describedBy, invalid, required }),
    [controlId, labelId, describedBy, invalid, required],
  )

  const text = (
    <>
      <label id={labelId} htmlFor={controlId} className="kb-field__label">
        {label}
        {required && (
          <span className="kb-field__required" aria-hidden="true">
            *
          </span>
        )}
      </label>
      {help && (
        <div id={helpId} className="kb-field__help">
          {help}
        </div>
      )}
    </>
  )

  const control = (
    <>
      <FieldContext.Provider value={context}>{children}</FieldContext.Provider>
      {error && (
        <div id={errorId} className="kb-field__error">
          <Icon name="error" size={13} />
          <span>{error}</span>
        </div>
      )}
    </>
  )

  return (
    <div className={clsx('kb-field', `kb-field--${layout}`, invalid && 'kb-field--invalid', className)}>
      {layout === 'inline' ? (
        <>
          <div className="kb-field__text">{text}</div>
          <div className="kb-field__control">{control}</div>
        </>
      ) : (
        <>
          {text}
          {control}
        </>
      )}
    </div>
  )
}
