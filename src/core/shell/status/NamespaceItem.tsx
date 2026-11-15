import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { usePermissions } from '@/core/permissions'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { useNamespacePrefs } from '@/core/store/namespacePrefsStore'
import { Icon } from '@/shared/Icon'
import { NamespacePicker } from './NamespacePicker'
import { useNamespacePickerRequest } from '@/core/store/namespacePickerRequest'

function NamespaceButton() {
  const { t } = useTranslation(PLATFORM_NS)
  const namespace = useContextStore((s) => s.namespace)
  const { pinned } = useNamespacePrefs()
  const buttonRef = useRef<HTMLButtonElement>(null)
  const [anchor, setAnchor] = useState<HTMLElement | null>(null)
  const request = useNamespacePickerRequest((s) => s.seq)

  // The shortcut and the palette command open the picker from anywhere.
  useEffect(() => {
    if (request > 0) setAnchor(buttonRef.current)
  }, [request])

  const label = namespace === ALL_NAMESPACES ? t('statusBar.namespace.all') : namespace
  return (
    <>
      <button
        ref={buttonRef}
        type="button"
        className="status-bar__item status-bar__item--button"
        title={t('statusBar.namespace.tooltip')}
        aria-haspopup="dialog"
        aria-expanded={Boolean(anchor)}
        onClick={(e) => {
          const button = e.currentTarget
          setAnchor((current) => (current ? null : button))
        }}
      >
        <Icon name={pinned.includes(namespace) ? 'pinned' : 'folder'} size={13} />
        <span>{label}</span>
      </button>
      {anchor && <NamespacePicker anchor={anchor} onClose={() => setAnchor(null)} />}
    </>
  )
}

/** Namespace filter of the list views; hidden while no cluster can be read. */
export function NamespaceItem() {
  const { canRead } = usePermissions()
  return canRead ? <NamespaceButton /> : null
}
