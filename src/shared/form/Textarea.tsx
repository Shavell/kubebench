import clsx from 'clsx'
import type { Ref, TextareaHTMLAttributes } from 'react'
import { useFieldControlProps } from './fieldContext'
import './form.css'

export interface TextareaProps
  extends Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, 'value' | 'onChange'> {
  value: string
  onChange: (value: string) => void
  /** Monospace text without spell checking (certificates, tokens, snippets). */
  monospace?: boolean
  ref?: Ref<HTMLTextAreaElement>
}

/** Multi-line plain text input. Use CodeEditor for YAML or JSON. */
export function Textarea({ value, onChange, monospace = false, rows = 4, className, ...rest }: TextareaProps) {
  const a11y = useFieldControlProps(rest.id)
  return (
    <textarea
      spellCheck={monospace ? false : undefined}
      {...rest}
      {...a11y}
      rows={rows}
      value={value}
      onChange={(event) => onChange(event.target.value)}
      className={clsx('kb-textarea', monospace && 'kb-input--mono', className)}
    />
  )
}
