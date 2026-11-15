import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { keyCaps } from './keybinding'
import './shortcuts.css'

/** Keys whose names are shown translated (or as symbols, e.g. arrows). */
const NAMED_KEYS = new Set(['ArrowUp', 'ArrowDown', 'Enter', 'Escape', 'Tab', 'Space', 'PageUp', 'PageDown', 'Home', 'End', 'ContextMenu'])

const IS_MAC = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.userAgent)

/** A key combination as key caps, e.g. [Ctrl] [Shift] [P]. */
export function KeyCaps({ binding }: { binding: string }) {
  const { t } = useTranslation(PLATFORM_NS)
  return (
    <span className="kb-keycaps">
      {keyCaps(binding, IS_MAC).map((cap, i) => (
        <kbd key={i} className="kb-keycap">
          {NAMED_KEYS.has(cap) ? t(`shortcuts.keys.${cap}`) : cap}
        </kbd>
      ))}
    </span>
  )
}
