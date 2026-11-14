import clsx from 'clsx'
import type { Ref, SelectHTMLAttributes } from 'react'
import { useFieldControlProps } from './fieldContext'
import './form.css'

export interface SelectOption<T extends string | number> {
  value: T
  /** Already translated. */
  label: string
  disabled?: boolean
}

export interface SelectProps<T extends string | number>
  extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'value' | 'onChange' | 'multiple'> {
  value: T
  onChange: (value: T) => void
  options: readonly SelectOption<T>[]
  ref?: Ref<HTMLSelectElement>
}

/** Drop-down list; values keep their type (numbers stay numbers). */
export function Select<T extends string | number>({
  value,
  onChange,
  options,
  className,
  ...rest
}: SelectProps<T>) {
  const a11y = useFieldControlProps(rest.id)
  return (
    <select
      {...rest}
      {...a11y}
      value={String(value)}
      onChange={(event) => {
        const option = options.find((o) => String(o.value) === event.target.value)
        if (option) onChange(option.value)
      }}
      className={clsx('kb-select', className)}
    >
      {options.map((option) => (
        <option key={String(option.value)} value={String(option.value)} disabled={option.disabled}>
          {option.label}
        </option>
      ))}
    </select>
  )
}
