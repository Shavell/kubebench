import { useCallback, useState, type MouseEvent } from 'react'

/** Open state of a status bar menu: the button it hangs from, or null when closed. */
export function useMenuAnchor() {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null)
  const toggle = useCallback(
    (event: MouseEvent<HTMLElement>) => {
      const button = event.currentTarget
      setAnchor((current) => (current ? null : button))
    },
    [],
  )
  const close = useCallback(() => setAnchor(null), [])
  return { anchor, toggle, close }
}
