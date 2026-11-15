import clsx from 'clsx'
import { Icon } from '@/shared/Icon'
import './toolButton.css'

interface ToolButtonProps {
  icon: string
  /** Tooltip and accessible name (already translated). */
  title: string
  onClick: () => void
  disabled?: boolean
  /** Makes it a toggle; true shows it switched on. */
  pressed?: boolean
}

/** Icon-only toolbar button of the terminal and log views (VS Code panel style). */
export function ToolButton({ icon, title, onClick, disabled, pressed }: ToolButtonProps) {
  return (
    <button
      type="button"
      className={clsx('term-tool', pressed && 'term-tool--on')}
      title={title}
      aria-label={title}
      aria-pressed={pressed}
      disabled={disabled}
      onClick={onClick}
    >
      <Icon name={icon} size={16} />
    </button>
  )
}
