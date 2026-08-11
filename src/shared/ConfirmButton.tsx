import { useEffect, useRef, useState } from 'react'
import { Icon } from './Icon'
import './ConfirmButton.css'

interface ConfirmButtonProps {
  icon: string
  label: string
  confirmLabel: string
  onConfirm: () => void
}

/**
 * Two-step destructive action button: first click arms it ("Confirm?"),
 * second click within 3s runs the action. Avoids modal dialogs.
 */
export function ConfirmButton({ icon, label, confirmLabel, onConfirm }: ConfirmButtonProps) {
  const [armed, setArmed] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current)
    },
    [],
  )

  const onClick = () => {
    if (armed) {
      setArmed(false)
      onConfirm()
    } else {
      setArmed(true)
      timer.current = setTimeout(() => setArmed(false), 5000)
    }
  }

  return (
    <button className={armed ? 'kb-button kb-button--danger-armed' : 'kb-button kb-button--danger'} onClick={onClick}>
      <Icon name={icon} size={14} /> {armed ? confirmLabel : label}
    </button>
  )
}
