import clsx from 'clsx'
import type { InputHTMLAttributes, Ref } from 'react'
import { useFieldControlProps } from './fieldContext'
import './form.css'

export interface TextInputProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, 'value' | 'onChange' | 'type'> {
  value: string
  onChange: (value: string) => void
  type?: 'text' | 'password' | 'email' | 'url' | 'search'
  /** Monospace text without spell checking, for names, paths and other machine values. */
  monospace?: boolean
  ref?: Ref<HTMLInputElement>
}

/** Single-line text input. */
export function TextInput({
  value,
  onChange,
  type = 'text',
  monospace = false,
  className,
  ...rest
}: TextInputProps) {
  const a11y = useFieldControlProps(rest.id)
  return (
    <input
      spellCheck={monospace ? false : undefined}
      autoCapitalize={monospace ? 'off' : undefined}
      autoCorrect={monospace ? 'off' : undefined}
      {...rest}
      {...a11y}
      type={type}
      value={value}
      onChange={(event) => onChange(event.target.value)}
      className={clsx('kb-input', monospace && 'kb-input--mono', className)}
    />
  )
}
