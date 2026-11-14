import clsx from 'clsx'
import type { ButtonHTMLAttributes, Ref } from 'react'
import { Icon } from './Icon'
// The base .kb-button / .kb-button--secondary rules live with the toolbar styles.
import './Toolbar.css'
import './Button.css'

export type ButtonVariant = 'primary' | 'secondary' | 'danger'

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** primary (default): the main action; secondary: everything else; danger: destructive actions. */
  variant?: ButtonVariant
  /** Codicon name shown before the label, e.g. "trash". */
  icon?: string
  /** An action is running: shows a spinner and ignores clicks. */
  busy?: boolean
  ref?: Ref<HTMLButtonElement>
}

/**
 * The workbench button (VS Code style). Defaults to `type="button"` so it
 * never submits a surrounding form by accident; pass `type="submit"` for
 * the primary action of a form or dialog.
 */
export function Button({
  variant = 'primary',
  icon,
  busy = false,
  disabled,
  type = 'button',
  className,
  children,
  ...rest
}: ButtonProps) {
  return (
    <button
      {...rest}
      type={type}
      disabled={disabled || busy}
      aria-busy={busy || undefined}
      className={clsx(
        'kb-button',
        variant === 'secondary' && 'kb-button--secondary',
        variant === 'danger' && 'kb-button--destructive',
        className,
      )}
    >
      {busy ? (
        <Icon name="loading" size={14} className="codicon-modifier-spin" />
      ) : (
        icon && <Icon name={icon} size={14} />
      )}
      {children}
    </button>
  )
}
