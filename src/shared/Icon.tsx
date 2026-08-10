import clsx from 'clsx'
import type { CSSProperties } from 'react'

interface IconProps {
  /** Codicon name without the "codicon-" prefix, e.g. "file-code". */
  name: string
  size?: number
  className?: string
  style?: CSSProperties
}

export function Icon({ name, size, className, style }: IconProps) {
  return (
    <i
      className={clsx('codicon', `codicon-${name}`, className)}
      style={size ? { fontSize: size, ...style } : style}
      aria-hidden="true"
    />
  )
}
